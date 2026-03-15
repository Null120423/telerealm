package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"telerealm/models"
	"telerealm/services"
	"telerealm/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handlers struct {
	service services.FileService
}

var (
	mu                  sync.Mutex // Keep this if shared mutable state is added later.
	defaultMaxUploadMB             = int64(2048)
)

func NewHandlers(service services.FileService) *Handlers {
	return &Handlers{service: service}
}

func (h *Handlers) Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}

func buildSecureURL(c *gin.Context, botToken, fileID string) (string, error) {
	scheme := c.Request.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}

	encryptedToken, err := utils.EncryptFileInfo(botToken, fileID)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s://%s/drive/%s", scheme, c.Request.Host, encryptedToken), nil
}

func (h *Handlers) uploadAndCreateRecord(c *gin.Context, botToken, chatID string) (models.FileRecord, int, error) {
	if strings.TrimSpace(chatID) == "" {
		return models.FileRecord{}, http.StatusBadRequest, fmt.Errorf("chat_id is required")
	}

	fileHeader, err := c.FormFile("document")
	if err != nil {
		return models.FileRecord{}, http.StatusBadRequest, fmt.Errorf("document is required")
	}

	if sizeErr := validateUploadSize(fileHeader.Size); sizeErr != nil {
		return models.FileRecord{}, http.StatusRequestEntityTooLarge, sizeErr
	}

	file, err := fileHeader.Open()
	if err != nil {
		return models.FileRecord{}, http.StatusInternalServerError, fmt.Errorf("failed to open file")
	}
	defer file.Close()

	fileID, err := h.service.SendFile(botToken, chatID, file, fileHeader.Filename)
	if err != nil {
		return models.FileRecord{}, resolveSendDocumentErrorStatus(err), fmt.Errorf("failed to send document: %v", err)
	}

	fileURL, fileSize, err := h.service.GetFileInfo(botToken, fileID)
	if err != nil {
		return models.FileRecord{}, http.StatusInternalServerError, fmt.Errorf("failed to get file info: %v", err)
	}

	secureURL, err := buildSecureURL(c, botToken, fileID)
	if err != nil {
		return models.FileRecord{}, http.StatusInternalServerError, fmt.Errorf("failed to create secure URL")
	}

	fileExt := filepath.Ext(fileHeader.Filename)
	if fileExt != "" {
		fileExt = fileExt[1:]
	}

	record := models.FileRecord{
		BotToken:     botToken,
		RecordID:     uuid.New().String(),
		FileID:       fileID,
		ChatID:       chatID,
		URL:          fileURL,
		SecureURL:    secureURL,
		Bytes:        fileSize,
		Format:       fileExt,
		OriginalName: fileHeader.Filename,
	}

	return h.service.CreateFileRecord(record), http.StatusCreated, nil
}

func (h *Handlers) PublicCreateLinkRecord(c *gin.Context) {
	botToken := c.Param("botToken")
	chatID := c.Param("chatID")

	record, status, err := h.uploadAndCreateRecord(c, botToken, chatID)
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, models.Response{
		Success: true,
		Message: "Public upload completed successfully!",
		Data:    record,
	})
}

func (h *Handlers) PublicListLinkRecords(c *gin.Context) {
	botToken := c.Param("botToken")
	chatID := c.Param("chatID")

	records := h.service.ListFileRecordsByScope(botToken, chatID)
	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Scoped file records retrieved successfully!",
		Data:    records,
	})
}

func (h *Handlers) PublicGetLinkRecord(c *gin.Context) {
	botToken := c.Param("botToken")
	chatID := c.Param("chatID")
	recordID := c.Param("id")

	record, exists := h.service.GetScopedFileRecord(botToken, chatID, recordID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "file record not found"})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Scoped file record retrieved successfully!",
		Data:    record,
	})
}

func (h *Handlers) PublicUpdateLinkRecord(c *gin.Context) {
	botToken := c.Param("botToken")
	chatID := c.Param("chatID")
	recordID := c.Param("id")

	var req models.FileRecordUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	if req.ChatID == nil && req.OriginalName == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one updatable field is required"})
		return
	}

	record, exists := h.service.UpdateScopedFileRecord(botToken, chatID, recordID, req)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "file record not found"})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Scoped file record updated successfully!",
		Data:    record,
	})
}

func (h *Handlers) PublicDeleteLinkRecord(c *gin.Context) {
	botToken := c.Param("botToken")
	chatID := c.Param("chatID")
	recordID := c.Param("id")

	if !h.service.DeleteScopedFileRecord(botToken, chatID, recordID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file record not found"})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Scoped file record deleted successfully!",
		Data:    gin.H{"record_id": recordID},
	})
}

func (h *Handlers) CreateFileRecord(c *gin.Context) {
	botToken := c.MustGet("bot_token").(string)
	chatID := c.PostForm("chat_id")
	created, status, err := h.uploadAndCreateRecord(c, botToken, chatID)
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(status, models.Response{
		Success: true,
		Message: "Upload and create file record successfully!",
		Data:    created,
	})
}

func (h *Handlers) ListFileRecords(c *gin.Context) {
	records := h.service.ListFileRecords()
	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "File records retrieved successfully!",
		Data:    records,
	})
}

func (h *Handlers) GetFileRecord(c *gin.Context) {
	recordID := c.Param("id")
	record, exists := h.service.GetFileRecord(recordID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "file record not found"})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "File record retrieved successfully!",
		Data:    record,
	})
}

func (h *Handlers) UpdateFileRecord(c *gin.Context) {
	recordID := c.Param("id")

	var req models.FileRecordUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	if req.ChatID == nil && req.OriginalName == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one updatable field is required"})
		return
	}

	record, exists := h.service.UpdateFileRecord(recordID, req)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "file record not found"})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "File record updated successfully!",
		Data:    record,
	})
}

func (h *Handlers) DeleteFileRecord(c *gin.Context) {
	recordID := c.Param("id")
	if !h.service.DeleteFileRecord(recordID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file record not found"})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "File record deleted successfully!",
		Data:    gin.H{"record_id": recordID},
	})
}

func (h *Handlers) SendFile(c *gin.Context) {
	botToken := c.MustGet("bot_token").(string)
	chatID := c.PostForm("chat_id")

	if strings.TrimSpace(chatID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chat_id is required"})
		return
	}

	var fileID string
	var fileURL string
	var fileSize int
	var err error
	var fileExt string

	fileHeader, err := c.FormFile("document")
	if err == nil {
		if sizeErr := validateUploadSize(fileHeader.Size); sizeErr != nil {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": sizeErr.Error()})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("[send] failed to open file chat_id=%s file=%s err=%v", chatID, fileHeader.Filename, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
			return
		}
		defer file.Close()

		fileID, err = h.service.SendFile(botToken, chatID, file, fileHeader.Filename)
		if err != nil {
			log.Printf("[send] Telegram send failed chat_id=%s file=%s err=%v", chatID, fileHeader.Filename, err)
			status := resolveSendDocumentErrorStatus(err)
			c.JSON(status, gin.H{"error": fmt.Sprintf("Failed to send document: %v", err)})
			return
		}

		fileURL, fileSize, err = h.service.GetFileInfo(botToken, fileID)
		if err != nil {
			log.Printf("[send] getFile info failed chat_id=%s file_id=%s err=%v", chatID, fileID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get file info: %v", err)})
			return
		}

		fileExt = filepath.Ext(fileHeader.Filename)
		if fileExt != "" {
			fileExt = fileExt[1:] // Remove the leading dot
		}
	} else {
		if !errors.Is(err, http.ErrMissingFile) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document field"})
			return
		}

		fileURL = c.PostForm("document")
		if strings.TrimSpace(fileURL) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "document is required"})
			return
		}

		fileSize = 0

		isFile, contentType, contentLength, err := isURLFile(fileURL)

		if err != nil {
			log.Printf("[send] URL file check failed chat_id=%s url=%s err=%v", chatID, fileURL, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to check URL: %v", err)})
			return
		}

		if sizeErr := validateUploadSize(contentLength); sizeErr != nil {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": sizeErr.Error()})
			return
		}

		if !isFile {
			c.JSON(http.StatusBadRequest, gin.H{"error": "URL does not point to a file"})
			return
		}

		fileExt = getExtensionFromContentType(contentType)
	}

	scheme := c.Request.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}

	encryptedToken, err := utils.EncryptFileInfo(botToken, fileID)
	if err != nil {
		log.Printf("[send] secure URL encryption failed chat_id=%s file_id=%s err=%v", chatID, fileID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create secure URL"})
		return
	}

	secureURL := fmt.Sprintf("%s://%s/drive/%s", scheme, c.Request.Host, encryptedToken)

	response := models.Response{
		Success: true,
		Message: "Upload file successfully!",
		Data: models.FileData{
			ID:        fileID,
			URL:       fileURL,
			SecureURL: secureURL,
			Bytes:     fileSize,
			Format:    fileExt,
		},
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handlers) GetFileURL(c *gin.Context) {
	botToken := c.MustGet("bot_token").(string)
	fileID := c.Query("file_id")

	fileURL, _, err := h.service.GetFileInfo(botToken, fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get file info: %v", err)})
		return
	}

	scheme := c.Request.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}

	// Use the current encryption method for secure links.
	encryptedToken, err := utils.EncryptFileInfo(botToken, fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create secure URL"})
		return
	}

	secureURL := fmt.Sprintf("%s://%s/drive/%s", scheme, c.Request.Host, encryptedToken)

	response := models.Response{
		Success: true,
		Message: "File URL retrieved successfully!",
		Data: models.FileData{
			URL:       fileURL,
			SecureURL: secureURL,
		},
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handlers) DownloadFile(c *gin.Context) {
	encryptedToken := c.Param("key")

	botToken, fileID, err := utils.DecryptFileInfo(encryptedToken)
	if err != nil {
		fmt.Printf("Decryption error: %v\n", err) // Add this logging
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token"})
		return
	}

	fileURL, _, err := h.service.GetFileInfo(botToken, fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get file info: %v", err)})
		return
	}

	resp, err := http.Get(fileURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch file"})
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Read the extension from the source URL.
	fileExt := filepath.Ext(fileURL)
	if fileExt != "" {
		fileExt = fileExt[1:] // Remove the leading dot.
	} else {
		// If the URL has no extension, derive one from the content type.
		fileExt = getExtensionFromContentType(contentType)
	}

	// Generate a short file name from the first 8 characters of fileID.
	shortID := fileID
	if len(fileID) > 8 {
		shortID = fileID[:8]
	}

	filename := fmt.Sprintf("file_%s", shortID)
	if fileExt != "" {
		filename += "." + fileExt
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

func (h *Handlers) GetFileInfo(c *gin.Context) {
	botToken := c.MustGet("bot_token").(string)
	fileID := c.Query("file_id")

	fileURL, fileSize, err := h.service.GetFileInfo(botToken, fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to get file info: %v", err),
		})
		return
	}

	scheme := c.Request.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}

	fileExt := filepath.Ext(fileURL)
	if fileExt != "" {
		fileExt = fileExt[1:] // Remove the leading dot
	}

	// Use the current encryption method for secure links.
	encryptedToken, err := utils.EncryptFileInfo(botToken, fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create secure URL"})
		return
	}

	secureURL := fmt.Sprintf("%s://%s/drive/%s", scheme, c.Request.Host, encryptedToken)

	response := models.Response{
		Success: true,
		Message: "Get file information successfully!",
		Data: models.FileData{
			ID:        fileID,
			URL:       fileURL,
			SecureURL: secureURL,
			Bytes:     fileSize,
			Format:    fileExt,
		},
	}

	c.JSON(http.StatusOK, response)
}

func getExtensionFromContentType(contentType string) string {
	// Strip parameters such as charset.
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = contentType[:idx]
	}
	contentType = strings.TrimSpace(contentType)

	switch contentType {
	case "application/zip":
		return "zip"
	case "application/x-7z-compressed":
		return "7z"
	case "application/pdf":
		return "pdf"
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "text/plain":
		return "txt"
	case "text/html":
		return "html"
	case "application/json":
		return "json"
	case "application/xml", "text/xml":
		return "xml"
	case "application/msword":
		return "doc"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "docx"
	case "application/vnd.ms-excel":
		return "xls"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return "xlsx"
	case "application/vnd.ms-powerpoint":
		return "ppt"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return "pptx"
	case "video/mp4":
		return "mp4"
	case "video/webm":
		return "webm"
	case "audio/mpeg":
		return "mp3"
	case "audio/ogg":
		return "ogg"
	case "application/vnd.rar":
		return "rar"
	default:
		return ""
	}
}

func isURLFile(url string) (bool, string, int64, error) {
	resp, err := http.Head(url)
	if err != nil {
		return false, "", 0, err
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	contentLength := resp.ContentLength

	isFile := strings.HasPrefix(contentType, "image/") ||
		strings.HasPrefix(contentType, "application/") ||
		strings.HasPrefix(contentType, "video/") ||
		strings.HasPrefix(contentType, "audio/")

	return isFile, contentType, contentLength, nil
}

func getMaxUploadBytes() int64 {
	raw := strings.TrimSpace(os.Getenv("TELEGRAM_MAX_UPLOAD_MB"))
	if raw == "" {
		return defaultMaxUploadMB * 1024 * 1024
	}

	mb, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || mb <= 0 {
		return defaultMaxUploadMB * 1024 * 1024
	}

	return mb * 1024 * 1024
}

func validateUploadSize(size int64) error {
	if size <= 0 {
		return nil
	}

	maxBytes := getMaxUploadBytes()
	if size <= maxBytes {
		return nil
	}

	maxMB := maxBytes / (1024 * 1024)
	actualMB := float64(size) / 1024.0 / 1024.0
	return fmt.Errorf("file is too large (%.2f MB). Current limit is %d MB. Reduce file size or set TELEGRAM_MAX_UPLOAD_MB if your Telegram endpoint supports larger uploads", actualMB, maxMB)
}

func resolveSendDocumentErrorStatus(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "http 413") ||
		strings.Contains(msg, "error_code=413") ||
		strings.Contains(msg, "request entity too large") {
		return http.StatusRequestEntityTooLarge
	}

	return http.StatusInternalServerError
}

func (h *Handlers) CheckBotAndChat(c *gin.Context) {
	botToken := c.MustGet("bot_token").(string)
	chatID := c.Query("chat_id")

	botInfo, chatInfo, botInChat, botIsAdmin, err := h.service.CheckBotAndChat(botToken, chatID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to check bot and chat info: %v", err),
		})
		return
	}

	response := models.Response{
		Success: true,
		Message: "Bot and chat information retrieved successfully!",
		Data: gin.H{
			"bot_info":     botInfo,
			"chat_info":    chatInfo,
			"bot_in_chat":  botInChat,
			"bot_is_admin": botIsAdmin,
		},
	}

	c.JSON(http.StatusOK, response)
}
