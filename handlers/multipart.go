package handlers

import (
	"fmt"
	"io"
	"net/http"
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
	UploadID    string
	FileKey     string
	FileName    string
	FileExt     string
	FileSize    int64
	ChunkSize   int64
	TotalParts  int
	ContentType string
	ChatID      string
	CreatedAt   time.Time
	Completed   bool
	UploadedBy  string
	PartFileIDs map[int]string
	mu          sync.Mutex
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
	ChatID      string `json:"chat_id"`
}

func (h *Handlers) InitMultipartUpload(c *gin.Context) {
	botToken := c.MustGet("bot_token").(string)

	var req multipartInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	req.FileName = strings.TrimSpace(req.FileName)
	req.ChatID = strings.TrimSpace(req.ChatID)
	if req.FileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_name is required"})
		return
	}
	if req.ChatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chat_id is required"})
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
	if totalParts <= 0 || totalParts != computedParts {
		totalParts = computedParts
	}

	uploadID := uuid.NewString()
	fileKey := strings.ReplaceAll(uuid.NewString(), "-", "")
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
		ChatID:      req.ChatID,
		CreatedAt:   time.Now().UTC(),
		UploadedBy:  botToken,
		PartFileIDs: make(map[int]string),
	}

	store.mu.Lock()
	store.byUploadID[uploadID] = session
	store.byFileKey[fileKey] = session
	store.mu.Unlock()

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Multipart upload initialized",
		Data: gin.H{
			"upload_id":        uploadID,
			"file_key":         fileKey,
			"part_size":        chunkSize,
			"total_parts":      totalParts,
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
	if partHeader.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chunk is empty"})
		return
	}
	if partHeader.Size > maxMultipartChunkSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chunk size exceeds maximum allowed chunk size"})
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Completed {
		c.JSON(http.StatusConflict, gin.H{"error": "upload already completed"})
		return
	}

	if existingID, exists := session.PartFileIDs[partNumber]; exists && strings.TrimSpace(existingID) != "" {
		c.JSON(http.StatusOK, models.Response{
			Success: true,
			Message: "Chunk already uploaded",
			Data: gin.H{
				"upload_id":   session.UploadID,
				"part_number": partNumber,
				"file_id":     existingID,
			},
		})
		return
	}

	partFile, err := partHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open chunk"})
		return
	}
	defer partFile.Close()

	chunkFileName := fmt.Sprintf("%s.part%06d%s", session.FileKey, partNumber, session.FileExt)
	fileID, err := h.service.SendFile(session.UploadedBy, session.ChatID, partFile, chunkFileName)
	if err != nil {
		c.JSON(resolveSendDocumentErrorStatus(err), gin.H{"error": fmt.Sprintf("failed to upload chunk %d: %v", partNumber, err)})
		return
	}

	session.PartFileIDs[partNumber] = fileID

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Chunk uploaded",
		Data: gin.H{
			"upload_id":   session.UploadID,
			"part_number": partNumber,
			"file_id":     fileID,
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

	session.mu.Lock()
	defer session.mu.Unlock()

	for i := 1; i <= session.TotalParts; i++ {
		if strings.TrimSpace(session.PartFileIDs[i]) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("missing chunk %d", i)})
			return
		}
	}
	session.Completed = true

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
}

func (h *Handlers) DownloadMultipartFile(c *gin.Context) {
	fileKey := strings.TrimSpace(c.Param("key"))
	if fileKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
		return
	}

	session, ok := getMultipartSessionByKey(fileKey)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if !session.Completed {
		for i := 1; i <= session.TotalParts; i++ {
			if strings.TrimSpace(session.PartFileIDs[i]) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file is not ready, missing chunk %d", i)})
				return
			}
		}
		session.Completed = true
	}

	contentType := strings.TrimSpace(session.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	fileName := strings.TrimSpace(session.FileName)
	if fileName == "" {
		fileName = "download" + session.FileExt
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	if session.FileSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(session.FileSize, 10))
	}
	c.Status(http.StatusOK)

	for i := 1; i <= session.TotalParts; i++ {
		fileID := session.PartFileIDs[i]
		fileURL, _, err := h.service.GetFileInfo(session.UploadedBy, fileID)
		if err != nil {
			return
		}

		resp, err := http.Get(fileURL)
		if err != nil {
			return
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return
		}

		_, _ = io.Copy(c.Writer, resp.Body)
		resp.Body.Close()
	}
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
