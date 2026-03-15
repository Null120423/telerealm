package services

import (
	"io"
	"sync"
	"telerealm/models"
	"time"

	"telerealm/repositories"
)

type FileService interface {
	SendFile(botToken, chatID string, file io.Reader, fileName string) (string, error)
	GetFileInfo(botToken, fileID string) (string, int, error)
	CheckBotAndChat(botToken, chatID string) (botInfo, chatInfo interface{}, botInChat, botIsAdmin bool, err error)
	CreateFileRecord(record models.FileRecord) models.FileRecord
	GetFileRecord(recordID string) (models.FileRecord, bool)
	ListFileRecords() []models.FileRecord
	ListFileRecordsByScope(botToken, chatID string) []models.FileRecord
	GetScopedFileRecord(botToken, chatID, recordID string) (models.FileRecord, bool)
	UpdateFileRecord(recordID string, req models.FileRecordUpdateRequest) (models.FileRecord, bool)
	UpdateScopedFileRecord(botToken, chatID, recordID string, req models.FileRecordUpdateRequest) (models.FileRecord, bool)
	DeleteFileRecord(recordID string) bool
	DeleteScopedFileRecord(botToken, chatID, recordID string) bool
}

type fileService struct {
	repo     repositories.FileRepository
	mu       sync.RWMutex
	records  map[string]models.FileRecord
}

func NewFileService(repo repositories.FileRepository) FileService {
	return &fileService{repo: repo, records: make(map[string]models.FileRecord)}
}

func (s *fileService) SendFile(botToken, chatID string, file io.Reader, fileName string) (string, error) {
	return s.repo.SendDocument(botToken, chatID, file, fileName)
}

func (s *fileService) GetFileInfo(botToken, fileID string) (string, int, error) {
	fileURL, fileSize, err := s.repo.GetFileInfo(botToken, fileID)
	if err != nil {
		return "", 0, err
	}

	return fileURL, fileSize, nil
}

func (s *fileService) CheckBotAndChat(botToken, chatID string) (botInfo, chatInfo interface{}, botInChat, botIsAdmin bool, err error) {
	return s.repo.CheckBotAndChat(botToken, chatID)
}

func (s *fileService) CreateFileRecord(record models.FileRecord) models.FileRecord {
	now := time.Now().UTC()
	record.CreatedAt = now
	record.UpdatedAt = now

	s.mu.Lock()
	s.records[record.RecordID] = record
	s.mu.Unlock()

	return record
}

func (s *fileService) GetFileRecord(recordID string) (models.FileRecord, bool) {
	s.mu.RLock()
	record, exists := s.records[recordID]
	s.mu.RUnlock()
	return record, exists
}

func (s *fileService) ListFileRecords() []models.FileRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]models.FileRecord, 0, len(s.records))
	for _, record := range s.records {
		records = append(records, record)
	}

	return records
}

func (s *fileService) ListFileRecordsByScope(botToken, chatID string) []models.FileRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]models.FileRecord, 0)
	for _, record := range s.records {
		if record.BotToken == botToken && record.ChatID == chatID {
			records = append(records, record)
		}
	}

	return records
}

func (s *fileService) GetScopedFileRecord(botToken, chatID, recordID string) (models.FileRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.records[recordID]
	if !exists || record.BotToken != botToken || record.ChatID != chatID {
		return models.FileRecord{}, false
	}

	return record, true
}

func (s *fileService) UpdateFileRecord(recordID string, req models.FileRecordUpdateRequest) (models.FileRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.records[recordID]
	if !exists {
		return models.FileRecord{}, false
	}

	if req.ChatID != nil {
		record.ChatID = *req.ChatID
	}
	if req.OriginalName != nil {
		record.OriginalName = *req.OriginalName
	}
	record.UpdatedAt = time.Now().UTC()

	s.records[recordID] = record
	return record, true
}

func (s *fileService) UpdateScopedFileRecord(botToken, chatID, recordID string, req models.FileRecordUpdateRequest) (models.FileRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.records[recordID]
	if !exists || record.BotToken != botToken || record.ChatID != chatID {
		return models.FileRecord{}, false
	}

	if req.ChatID != nil {
		record.ChatID = *req.ChatID
	}
	if req.OriginalName != nil {
		record.OriginalName = *req.OriginalName
	}
	record.UpdatedAt = time.Now().UTC()

	s.records[recordID] = record
	return record, true
}

func (s *fileService) DeleteFileRecord(recordID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.records[recordID]; !exists {
		return false
	}

	delete(s.records, recordID)
	return true
}

func (s *fileService) DeleteScopedFileRecord(botToken, chatID, recordID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.records[recordID]
	if !exists || record.BotToken != botToken || record.ChatID != chatID {
		return false
	}

	delete(s.records, recordID)
	return true
}