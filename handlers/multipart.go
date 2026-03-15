package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"telerealm/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	defaultMultipartChunkSize = int64(8 * 1024 * 1024)
	minMultipartChunkSize     = int64(1 * 1024 * 1024)
	maxMultipartChunkSize     = int64(64 * 1024 * 1024)
)

type multipartSession struct {
	UploadID     string
	FileKey      string
	FileName     string
	FileExt      string
	FileSize     int64
	ChunkSize    int64
	TotalParts   int
	ContentType  string
	PartDir      string
	MergedPath   string
	CreatedAt    time.Time
	Completed    bool
	UploadedBy   string
	mu           sync.Mutex
}

type multipartStore struct {
	mu         sync.RWMutex
	byUploadID map[string]*multipartSession
	byFileKey  map[string]*multipartSession
}

var store = multipartStore{
	byUploadID: make(map[string]*multipartSession),
	byFileKey:  make(map[string]*multipartSession),
}

type multipartInitRequest struct {
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	PartSize    int64  `json:"part_size"`
	TotalParts  int    `json:"total_parts"`
	ContentType string `json:"content_type"`
}

func (h *Handlers) InitMultipartUpload(c *gin.Context) {
	botToken := c.MustGet("bot_token").(string)

	var req multipartInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	req.FileName = strings.TrimSpace(req.FileName)
	if req.FileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_name is required"})
		return
	}
	if req.FileSize <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_size must be greater than 0"})
		return
	}

	if sizeErr := validateUploadSize(req.FileSize); sizeErr != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": sizeErr.Error()})
		return
	}

	chunkSize := req.PartSize
	if chunkSize <= 0 {
		chunkSize = defaultMultipartChunkSize
	}
	if chunkSize < minMultipartChunkSize {
		chunkSize = minMultipartChunkSize
	}
	if chunkSize > maxMultipartChunkSize {
		chunkSize = maxMultipartChunkSize
	}

	totalParts := req.TotalParts
	computedParts := int((req.FileSize + chunkSize - 1) / chunkSize)
	if totalParts <= 0 {
		totalParts = computedParts
	}
	if totalParts != computedParts {
		totalParts = computedParts
	}

	baseDir, err := ensureMultipartDirs()
	if err != nil {
		log.Printf("[multipart:init] failed to prepare storage file=%s size=%d err=%v", req.FileName, req.FileSize, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to prepare multipart storage: %v", err)})
		return
	}

	uploadID := uuid.NewString()
	fileKey := strings.ReplaceAll(uuid.NewString(), "-", "")
	partDir := filepath.Join(baseDir, "sessions", uploadID)
	if err := os.MkdirAll(partDir, 0o755); err != nil {
		log.Printf("[multipart:init] failed to create session dir upload_id=%s dir=%s err=%v", uploadID, partDir, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create upload session: %v", err)})
		return
	}

	fileExt := filepath.Ext(req.FileName)
	session := &multipartSession{
		UploadID:    uploadID,
		FileKey:     fileKey,
		FileName:    req.FileName,
		FileExt:     fileExt,
		FileSize:    req.FileSize,
		ChunkSize:   chunkSize,
		TotalParts:  totalParts,
		ContentType: strings.TrimSpace(req.ContentType),
		PartDir:     partDir,
		CreatedAt:   time.Now().UTC(),
		UploadedBy:  botToken,
	}

	store.mu.Lock()
	store.byUploadID[uploadID] = session
	store.byFileKey[fileKey] = session
	store.mu.Unlock()

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Multipart upload initialized",
		Data: gin.H{
			"upload_id":      uploadID,
			"file_key":       fileKey,
			"part_size":      chunkSize,
			"total_parts":    totalParts,
			"max_upload_bytes": getMaxUploadBytes(),
		},
	})
}

func (h *Handlers) UploadMultipartPart(c *gin.Context) {
	uploadID := c.Param("uploadID")
	session, ok := getMultipartSession(uploadID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "upload session not found"})
		return
	}

	partNumber, err := strconv.Atoi(c.Query("part_number"))
	if err != nil || partNumber <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "part_number must be a positive integer"})
		return
	}
	if partNumber > session.TotalParts {
		c.JSON(http.StatusBadRequest, gin.H{"error": "part_number exceeds total_parts"})
		return
	}

	partHeader, err := c.FormFile("chunk")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chunk is required"})
		return
	}

	if partHeader.Size > 0 && partHeader.Size > maxMultipartChunkSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chunk size exceeds maximum allowed chunk size"})
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Completed {
		c.JSON(http.StatusConflict, gin.H{"error": "upload already completed"})
		return
	}

	partFile, err := partHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open chunk"})
		return
	}
	defer partFile.Close()

	partPath := filepath.Join(session.PartDir, fmt.Sprintf("part_%06d", partNumber))
	tmpPath := partPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create chunk file"})
		return
	}

	if _, err := io.Copy(out, partFile); err != nil {
		out.Close()
		_ = os.Remove(tmpPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write chunk"})
		return
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to finalize chunk"})
		return
	}

	if err := os.Rename(tmpPath, partPath); err != nil {
		_ = os.Remove(tmpPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store chunk"})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Chunk uploaded",
		Data: gin.H{
			"upload_id":   session.UploadID,
			"part_number": partNumber,
		},
	})
}

func (h *Handlers) CompleteMultipartUpload(c *gin.Context) {
	uploadID := c.Param("uploadID")
	session, ok := getMultipartSession(uploadID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "upload session not found"})
		return
	}

	mergedPath, err := ensureMergedFile(session)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scheme := c.Request.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}
	downloadURL := fmt.Sprintf("%s://%s/multipart/download/%s", scheme, c.Request.Host, session.FileKey)

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Multipart upload completed",
		Data: models.FileData{
			ID:        session.FileKey,
			URL:       downloadURL,
			SecureURL: downloadURL,
			Bytes:     int(session.FileSize),
			Format:    strings.TrimPrefix(session.FileExt, "."),
		},
	})

	_ = mergedPath
}

func (h *Handlers) DownloadMultipartFile(c *gin.Context) {
	fileKey := c.Param("key")
	if strings.TrimSpace(fileKey) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
		return
	}

	session, ok := getMultipartSessionByKey(fileKey)
	if ok {
		mergedPath, err := ensureMergedFile(session)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		name := session.FileName
		if strings.TrimSpace(name) == "" {
			name = "download" + session.FileExt
		}
		c.FileAttachment(mergedPath, name)
		return
	}

	mergedPath, err := findMergedFileByKey(fileKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	c.FileAttachment(mergedPath, filepath.Base(mergedPath))
}

func ensureMultipartDirs() (string, error) {
	candidates := buildMultipartBaseDirCandidates()
	for _, baseDir := range candidates {
		if err := os.MkdirAll(filepath.Join(baseDir, "sessions"), 0o755); err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Join(baseDir, "files"), 0o755); err != nil {
			continue
		}
		return baseDir, nil
	}

	return "", fmt.Errorf("cannot create multipart directories in any candidate path")
}

func buildMultipartBaseDirCandidates() []string {
	custom := strings.TrimSpace(os.Getenv("MULTIPART_STORAGE_DIR"))
	list := make([]string, 0, 3)
	if custom != "" {
		list = append(list, custom)
	}

	list = append(list,
		filepath.Join("storage", "multipart"),
		filepath.Join(os.TempDir(), "telerealm", "multipart"),
	)

	return list
}

func getMultipartSession(uploadID string) (*multipartSession, bool) {
	store.mu.RLock()
	session, ok := store.byUploadID[uploadID]
	store.mu.RUnlock()
	return session, ok
}

func getMultipartSessionByKey(fileKey string) (*multipartSession, bool) {
	store.mu.RLock()
	session, ok := store.byFileKey[fileKey]
	store.mu.RUnlock()
	return session, ok
}

func ensureMergedFile(session *multipartSession) (string, error) {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Completed && session.MergedPath != "" {
		if _, err := os.Stat(session.MergedPath); err == nil {
			return session.MergedPath, nil
		}
	}

	baseDir, err := ensureMultipartDirs()
	if err != nil {
		return "", fmt.Errorf("failed to prepare storage: %w", err)
	}

	for i := 1; i <= session.TotalParts; i++ {
		partPath := filepath.Join(session.PartDir, fmt.Sprintf("part_%06d", i))
		if _, err := os.Stat(partPath); err != nil {
			return "", fmt.Errorf("missing chunk %d", i)
		}
	}

	mergedPath := filepath.Join(baseDir, "files", session.FileKey+session.FileExt)
	tmpPath := mergedPath + ".tmp"

	out, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to create merged file: %w", err)
	}

	for i := 1; i <= session.TotalParts; i++ {
		partPath := filepath.Join(session.PartDir, fmt.Sprintf("part_%06d", i))
		partFile, err := os.Open(partPath)
		if err != nil {
			out.Close()
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("failed to open chunk %d: %w", i, err)
		}

		if _, err := io.Copy(out, partFile); err != nil {
			partFile.Close()
			out.Close()
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("failed to merge chunk %d: %w", i, err)
		}
		partFile.Close()
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to finalize merged file: %w", err)
	}

	if err := os.Rename(tmpPath, mergedPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to commit merged file: %w", err)
	}

	session.MergedPath = mergedPath
	session.Completed = true
	return mergedPath, nil
}

func findMergedFileByKey(fileKey string) (string, error) {
	baseDir, err := ensureMultipartDirs()
	if err != nil {
		return "", err
	}

	matches, err := filepath.Glob(filepath.Join(baseDir, "files", fileKey+"*"))
	if err != nil || len(matches) == 0 {
		return "", fmt.Errorf("merged file not found")
	}
	return matches[0], nil
}
