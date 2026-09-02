package repository

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// URLRecord — структура записи URL
type URLRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id,omitempty"`
}

// Store — интерфейс хранилища URL
type Store interface {
	Insert(userID string, originalURL string, shortURL string) error
	BatchInsert(userID string, records []URLRecord) ([]URLRecord, error)
	Select(shortURL string) (URLRecord, bool)
	SelectByUser(userID string) []URLRecord
}

// FileStorage — хранение в памяти с опциональной записью в файл
type FileStorage struct {
	mu            sync.Mutex
	data          map[string]URLRecord // shortURL -> record
	originalIndex map[string]URLRecord // originalURL -> record (обратный индекс для O(1) поиска дубликатов)
	filePath      string
}

// NewFileStorage создаёт новое файловое хранилище
func NewFileStorage() *FileStorage {
	return &FileStorage{
		data:          make(map[string]URLRecord),
		originalIndex: make(map[string]URLRecord),
	}
}

// generateUUID создаёт случайный UUID v4.
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Init инициализирует хранилище, загружая данные из файла
func (s *FileStorage) Init(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Пробуем определить: если путь существует и это папка
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		s.filePath = filepath.Join(path, "storage.json")
	} else {
		// Во всех остальных случаях используем путь как есть
		s.filePath = path
	}

	if _, err := os.Stat(s.filePath); err == nil {
		fileData, err := os.ReadFile(s.filePath)
		if err != nil {
			return fmt.Errorf("ошибка чтения файла: %w", err)
		}

		if len(fileData) > 0 {
			var records []URLRecord
			if err := json.Unmarshal(fileData, &records); err != nil {
				return fmt.Errorf("ошибка парсинга JSON: %w", err)
			}

			for _, record := range records {
				s.data[record.ShortURL] = record
				s.originalIndex[record.OriginalURL] = record
			}
		}
	}

	return nil
}

// Insert добавляет запись в хранилище
func (s *FileStorage) Insert(userID string, originalURL string, shortURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Проверяем наличие дубликата по original_url через обратный индекс
	if _, ok := s.originalIndex[originalURL]; ok {
		return ErrDuplicateURL
	}

	uuid := generateUUID()

	record := URLRecord{
		UUID:        uuid,
		ShortURL:    shortURL,
		OriginalURL: originalURL,
		UserID:      userID,
	}
	s.data[shortURL] = record
	s.originalIndex[originalURL] = record

	return s.saveFile()
}

// BatchInsert добавляет несколько записей в хранилище и возвращает результаты
// с actual short_url (для дубликатов — существующий, для новых — вставленный)
func (s *FileStorage) BatchInsert(userID string, records []URLRecord) ([]URLRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make([]URLRecord, len(records))

	for i, record := range records {
		// Проверяем наличие дубликата по original_url через обратный индекс
		if existing, ok := s.originalIndex[record.OriginalURL]; ok {
			results[i] = existing
			continue
		}

		uuid := generateUUID()
		entry := URLRecord{
			UUID:        uuid,
			ShortURL:    record.ShortURL,
			OriginalURL: record.OriginalURL,
			UserID:      userID,
		}
		s.data[record.ShortURL] = entry
		s.originalIndex[record.OriginalURL] = entry
		results[i] = URLRecord{
			ShortURL:    record.ShortURL,
			OriginalURL: record.OriginalURL,
		}
	}

	return results, s.saveFile()
}

// saveFile сохраняет данные в файл
func (s *FileStorage) saveFile() error {
	if s.filePath == "" {
		return nil
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("ошибка создания папки: %w", err)
	}

	records := make([]URLRecord, 0, len(s.data))
	for _, record := range s.data {
		records = append(records, record)
	}

	data, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("ошибка сериализации JSON: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("ошибка записи в файл: %w", err)
	}

	return nil
}

// Save сохраняет данные в файл
func (s *FileStorage) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveFile()
}

// Close сохраняет данные и закрывает хранилище
func (s *FileStorage) Close() error {
	return s.Save()
}

// Select возвращает оригинальный URL по короткому
func (s *FileStorage) Select(shortURL string) (URLRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.data[shortURL]
	return record, ok
}

// SelectByUser возвращает все URL, сокращённые указанным пользователем
func (s *FileStorage) SelectByUser(userID string) []URLRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	records := make([]URLRecord, 0)
	for _, record := range s.data {
		if record.UserID == userID {
			records = append(records, record)
		}
	}

	return records
}
