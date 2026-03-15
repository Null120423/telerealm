package handlers

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"fmt"
	"io"
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
	UploadID    string
	FileKey     string
	FileName    string
	FileExt     string
	FileSize    int64
	ChunkSize   int64
	TotalParts  int
	ContentType string
	ExpectedFileSHA256 string
	ExpectedFileMD5    string
	CompletedFileSHA256 string
	CompletedFileMD5    string
	ChatID      string
	CreatedAt   time.Time
	Completed   bool
	UploadedBy  string
	PartFileIDs map[int]string
	PartSHA256  map[int]string
	PartMD5     map[int]string
	PartBytes   map[int]int64
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
	FileName         string `json:"file_name"`
	FileSize         int64  `json:"file_size"`
	PartSize         int64  `json:"part_size"`
	TotalParts       int    `json:"total_parts"`
	ContentType      string `json:"content_type"`
	ChatID           string `json:"chat_id"`
	FileSHA256       string `json:"file_sha256"`
	FileMD5          string `json:"file_md5"`
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
		ExpectedFileSHA256: normalizeHexChecksum(req.FileSHA256),
		ExpectedFileMD5:    normalizeHexChecksum(req.FileMD5),
		ChatID:      req.ChatID,
		CreatedAt:   time.Now().UTC(),
		UploadedBy:  botToken,
		PartFileIDs: make(map[int]string),
		PartSHA256:  make(map[int]string),
		PartMD5:     make(map[int]string),
		PartBytes:   make(map[int]int64),
	}

	store.mu.Lock()
	store.byUploadID[uploadID] = session
	store.byFileKey[fileKey] = session
	store.mu.Unlock()
	persistSession(session)

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

	providedPartSHA256 := normalizeHexChecksum(c.Query("part_sha256"))
	if providedPartSHA256 == "" {
		providedPartSHA256 = normalizeHexChecksum(c.GetHeader("X-Part-SHA256"))
	}
	if providedPartSHA256 == "" {
		providedPartSHA256 = normalizeHexChecksum(c.PostForm("part_sha256"))
	}

	providedPartMD5 := normalizeHexChecksum(c.Query("part_md5"))
	if providedPartMD5 == "" {
		providedPartMD5 = normalizeHexChecksum(c.GetHeader("X-Part-MD5"))
	}
	if providedPartMD5 == "" {
		providedPartMD5 = normalizeHexChecksum(c.PostForm("part_md5"))
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
		storedSHA := normalizeHexChecksum(session.PartSHA256[partNumber])
		storedMD5 := normalizeHexChecksum(session.PartMD5[partNumber])
		if providedPartSHA256 != "" && storedSHA != "" && providedPartSHA256 != storedSHA {
			c.JSON(http.StatusConflict, gin.H{"error": "part already uploaded with different sha256 checksum"})
			return
		}
		if providedPartMD5 != "" && storedMD5 != "" && providedPartMD5 != storedMD5 {
			c.JSON(http.StatusConflict, gin.H{"error": "part already uploaded with different md5 checksum"})
			return
		}

		c.JSON(http.StatusOK, models.Response{
			Success: true,
			Message: "Chunk already uploaded",
			Data: gin.H{
				"upload_id":   session.UploadID,
				"part_number": partNumber,
				"file_id":     existingID,
				"part_sha256": storedSHA,
				"part_md5":    storedMD5,
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

	chunkBytes, err := io.ReadAll(partFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read chunk"})
		return
	}
	if len(chunkBytes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chunk is empty"})
		return
	}

	actualPartSHA256 := checksumSHA256(chunkBytes)
	actualPartMD5 := checksumMD5(chunkBytes)
	if providedPartSHA256 != "" && providedPartSHA256 != actualPartSHA256 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "part sha256 checksum mismatch"})
		return
	}
	if providedPartMD5 != "" && providedPartMD5 != actualPartMD5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "part md5 checksum mismatch"})
		return
	}

	chunkFileName := fmt.Sprintf("%s.part%06d%s", session.FileKey, partNumber, session.FileExt)
	fileID, err := h.service.SendFile(session.UploadedBy, session.ChatID, bytes.NewReader(chunkBytes), chunkFileName)
	if err != nil {
		c.JSON(resolveSendDocumentErrorStatus(err), gin.H{"error": fmt.Sprintf("failed to upload chunk %d: %v", partNumber, err)})
		return
	}

	session.PartFileIDs[partNumber] = fileID
	session.PartSHA256[partNumber] = actualPartSHA256
	session.PartMD5[partNumber] = actualPartMD5
	session.PartBytes[partNumber] = int64(len(chunkBytes))
	persistSession(session)

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Chunk uploaded",
		Data: gin.H{
			"upload_id":   session.UploadID,
			"part_number": partNumber,
			"file_id":     fileID,
			"part_sha256": actualPartSHA256,
			"part_md5":    actualPartMD5,
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

	// Ensure no out-of-range part index exists (strict contiguous 1..N)
	for pn := range session.PartFileIDs {
		if pn < 1 || pn > session.TotalParts {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid chunk index %d", pn)})
			return
		}
	}

	shaAll := sha256.New()
	md5All := md5.New()
	var totalMergedBytes int64

	for i := 1; i <= session.TotalParts; i++ {
		fileID := session.PartFileIDs[i]
		fileURL, _, err := h.service.GetFileInfo(session.UploadedBy, fileID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to resolve chunk %d", i)})
			return
		}

		resp, err := http.Get(fileURL)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to fetch chunk %d", i)})
			return
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("chunk %d unavailable", i)})
			return
		}

		partHasherSHA := sha256.New()
		partHasherMD5 := md5.New()
		written, err := io.Copy(io.MultiWriter(shaAll, md5All, partHasherSHA, partHasherMD5), resp.Body)
		resp.Body.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to read chunk %d", i)})
			return
		}
		totalMergedBytes += written

		storedSHA := normalizeHexChecksum(session.PartSHA256[i])
		computedSHA := hex.EncodeToString(partHasherSHA.Sum(nil))
		if storedSHA != "" && storedSHA != computedSHA {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("chunk %d sha256 mismatch", i)})
			return
		}

		storedMD5 := normalizeHexChecksum(session.PartMD5[i])
		computedMD5 := hex.EncodeToString(partHasherMD5.Sum(nil))
		if storedMD5 != "" && storedMD5 != computedMD5 {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("chunk %d md5 mismatch", i)})
			return
		}

		session.PartSHA256[i] = computedSHA
		session.PartMD5[i] = computedMD5
		session.PartBytes[i] = written
	}

	if session.FileSize > 0 && totalMergedBytes != session.FileSize {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("merged size mismatch: expected %d, got %d", session.FileSize, totalMergedBytes)})
		return
	}

	completedSHA256 := hex.EncodeToString(shaAll.Sum(nil))
	completedMD5 := hex.EncodeToString(md5All.Sum(nil))

	if session.ExpectedFileSHA256 != "" && session.ExpectedFileSHA256 != completedSHA256 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "final file sha256 checksum mismatch"})
		return
	}
	if session.ExpectedFileMD5 != "" && session.ExpectedFileMD5 != completedMD5 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "final file md5 checksum mismatch"})
		return
	}

	session.Completed = true
	session.CompletedFileSHA256 = completedSHA256
	session.CompletedFileMD5 = completedMD5
	persistSession(session)

	scheme := c.Request.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}
	downloadURL := fmt.Sprintf("%s://%s/multipart/download/%s", scheme, c.Request.Host, session.FileKey)

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Multipart upload completed",
		Data: gin.H{
			"id":                 session.FileKey,
			"url":                downloadURL,
			"secure_url":         downloadURL,
			"bytes":              int(session.FileSize),
			"format":             strings.TrimPrefix(session.FileExt, "."),
			"file_sha256":        completedSHA256,
			"file_md5":           completedMD5,
			"expected_file_sha256": session.ExpectedFileSHA256,
			"expected_file_md5":    session.ExpectedFileMD5,
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
		// Try loading from disk (e.g. after server restart)
		session, ok = loadSessionFromDisk(fileKey)
		if ok {
			store.mu.Lock()
			store.byUploadID[session.UploadID] = session
			store.byFileKey[session.FileKey] = session
			store.mu.Unlock()
		}
	}
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
		persistSession(session)
	}

	contentType := strings.TrimSpace(session.ContentType)
	if contentType == "" {
		contentType = "video/mp4"
	}
	fileName := strings.TrimSpace(session.FileName)
	if fileName == "" {
		fileName = "download" + session.FileExt
	}

	totalSize := session.FileSize
	chunkSize := session.ChunkSize
	if chunkSize <= 0 {
		chunkSize = defaultMultipartChunkSize
	}

	// Parse Range header
	rangeHeader := c.Request.Header.Get("Range")
	var rangeStart, rangeEnd int64
	isRangeRequest := false

	if rangeHeader != "" && totalSize > 0 {
		if s, e, parsed := parseByteRange(rangeHeader, totalSize); parsed {
			rangeStart, rangeEnd = s, e
			isRangeRequest = true
		}
	}

	if !isRangeRequest {
		rangeStart = 0
		if totalSize > 0 {
			rangeEnd = totalSize - 1
		}
	}

	serveLen := rangeEnd - rangeStart + 1

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", fileName))
	c.Header("Accept-Ranges", "bytes")

	if isRangeRequest {
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rangeStart, rangeEnd, totalSize))
		c.Header("Content-Length", strconv.FormatInt(serveLen, 10))
		c.Status(http.StatusPartialContent)
	} else {
		if totalSize > 0 {
			c.Header("Content-Length", strconv.FormatInt(totalSize, 10))
		}
		c.Status(http.StatusOK)
	}

	startChunkIdx := int(rangeStart / chunkSize)
	startOffset := rangeStart % chunkSize
	remaining := serveLen

	for i := startChunkIdx; i < session.TotalParts && remaining > 0; i++ {
		partNumber := i + 1
		fileID := session.PartFileIDs[partNumber]
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

		// Skip bytes before rangeStart within the first chunk
		if i == startChunkIdx && startOffset > 0 {
			if _, err := io.CopyN(io.Discard, resp.Body, startOffset); err != nil {
				resp.Body.Close()
				return
			}
		}

		// Write exactly `remaining` bytes (no more)
		written, _ := io.Copy(c.Writer, io.LimitReader(resp.Body, remaining))
		resp.Body.Close()
		remaining -= written
	}
}

// parseByteRange parses a "bytes=start-end" header and returns clamped start/end.
func parseByteRange(header string, totalSize int64) (start, end int64, ok bool) {
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, false
	}
	parts := strings.SplitN(strings.TrimPrefix(header, "bytes="), "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}

	var err error
	if parts[0] == "" {
		// suffix range: bytes=-N
		var suffix int64
		suffix, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffix <= 0 {
			return 0, 0, false
		}
		start = totalSize - suffix
		end = totalSize - 1
	} else {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, 0, false
		}
		if parts[1] == "" {
			end = totalSize - 1
		} else {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return 0, 0, false
			}
		}
	}

	if start < 0 || end >= totalSize || start > end {
		return 0, 0, false
	}
	return start, end, true
}

// sessionStorageDir returns the directory where session JSON files are stored.
func sessionStorageDir() string {
	dir := os.Getenv("MULTIPART_STORAGE_DIR")
	if dir == "" {
		dir = "./storage/multipart"
	}
	return filepath.Join(dir, "sessions")
}

type sessionDisk struct {
	UploadID    string            `json:"upload_id"`
	FileKey     string            `json:"file_key"`
	FileName    string            `json:"file_name"`
	FileExt     string            `json:"file_ext"`
	FileSize    int64             `json:"file_size"`
	ChunkSize   int64             `json:"chunk_size"`
	TotalParts  int               `json:"total_parts"`
	ContentType string            `json:"content_type"`
	ExpectedFileSHA256 string     `json:"expected_file_sha256"`
	ExpectedFileMD5    string     `json:"expected_file_md5"`
	CompletedFileSHA256 string    `json:"completed_file_sha256"`
	CompletedFileMD5    string    `json:"completed_file_md5"`
	ChatID      string            `json:"chat_id"`
	CreatedAt   time.Time         `json:"created_at"`
	Completed   bool              `json:"completed"`
	UploadedBy  string            `json:"uploaded_by"`
	PartFileIDs map[string]string `json:"part_file_ids"`
	PartSHA256  map[string]string `json:"part_sha256"`
	PartMD5     map[string]string `json:"part_md5"`
	PartBytes   map[string]int64  `json:"part_bytes"`
}

// persistSession writes a session to disk so it survives server restarts.
func persistSession(s *multipartSession) {
	dir := sessionStorageDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	parts := make(map[string]string, len(s.PartFileIDs))
	for k, v := range s.PartFileIDs {
		parts[strconv.Itoa(k)] = v
	}
	partSHA := make(map[string]string, len(s.PartSHA256))
	for k, v := range s.PartSHA256 {
		partSHA[strconv.Itoa(k)] = v
	}
	partMD5 := make(map[string]string, len(s.PartMD5))
	for k, v := range s.PartMD5 {
		partMD5[strconv.Itoa(k)] = v
	}
	partBytes := make(map[string]int64, len(s.PartBytes))
	for k, v := range s.PartBytes {
		partBytes[strconv.Itoa(k)] = v
	}
	d := sessionDisk{
		UploadID: s.UploadID, FileKey: s.FileKey, FileName: s.FileName,
		FileExt: s.FileExt, FileSize: s.FileSize, ChunkSize: s.ChunkSize,
		TotalParts: s.TotalParts, ContentType: s.ContentType, ChatID: s.ChatID,
		ExpectedFileSHA256: s.ExpectedFileSHA256, ExpectedFileMD5: s.ExpectedFileMD5,
		CompletedFileSHA256: s.CompletedFileSHA256, CompletedFileMD5: s.CompletedFileMD5,
		CreatedAt: s.CreatedAt, Completed: s.Completed, UploadedBy: s.UploadedBy,
		PartFileIDs: parts,
		PartSHA256: partSHA,
		PartMD5: partMD5,
		PartBytes: partBytes,
	}
	data, err := json.Marshal(d)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, s.FileKey+".json"), data, 0644)
}

// loadSessionFromDisk loads a single session from its JSON file.
func loadSessionFromDisk(fileKey string) (*multipartSession, bool) {
	data, err := os.ReadFile(filepath.Join(sessionStorageDir(), fileKey+".json"))
	if err != nil {
		return nil, false
	}
	var d sessionDisk
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, false
	}
	return diskToSession(d), true
}

// LoadMultipartSessions loads all persisted sessions from disk into memory.
// Call once at startup after environment variables are loaded.
func LoadMultipartSessions() {
	dir := sessionStorageDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var d sessionDisk
		if err := json.Unmarshal(data, &d); err != nil {
			continue
		}
		s := diskToSession(d)
		store.mu.Lock()
		store.byUploadID[s.UploadID] = s
		store.byFileKey[s.FileKey] = s
		store.mu.Unlock()
	}
}

func diskToSession(d sessionDisk) *multipartSession {
	parts := make(map[int]string, len(d.PartFileIDs))
	for k, v := range d.PartFileIDs {
		if n, err := strconv.Atoi(k); err == nil {
			parts[n] = v
		}
	}
	partSHA := make(map[int]string, len(d.PartSHA256))
	for k, v := range d.PartSHA256 {
		if n, err := strconv.Atoi(k); err == nil {
			partSHA[n] = v
		}
	}
	partMD5 := make(map[int]string, len(d.PartMD5))
	for k, v := range d.PartMD5 {
		if n, err := strconv.Atoi(k); err == nil {
			partMD5[n] = v
		}
	}
	partBytes := make(map[int]int64, len(d.PartBytes))
	for k, v := range d.PartBytes {
		if n, err := strconv.Atoi(k); err == nil {
			partBytes[n] = v
		}
	}
	return &multipartSession{
		UploadID: d.UploadID, FileKey: d.FileKey, FileName: d.FileName,
		FileExt: d.FileExt, FileSize: d.FileSize, ChunkSize: d.ChunkSize,
		TotalParts: d.TotalParts, ContentType: d.ContentType,
		ExpectedFileSHA256: normalizeHexChecksum(d.ExpectedFileSHA256),
		ExpectedFileMD5: normalizeHexChecksum(d.ExpectedFileMD5),
		CompletedFileSHA256: normalizeHexChecksum(d.CompletedFileSHA256),
		CompletedFileMD5: normalizeHexChecksum(d.CompletedFileMD5),
		ChatID: d.ChatID,
		CreatedAt: d.CreatedAt, Completed: d.Completed, UploadedBy: d.UploadedBy,
		PartFileIDs: parts,
		PartSHA256: partSHA,
		PartMD5: partMD5,
		PartBytes: partBytes,
	}
}

func normalizeHexChecksum(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return ""
		}
	}
	return s
}

func checksumSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func checksumMD5(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
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
