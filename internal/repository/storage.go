package repository

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type URLRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// Storage — инкапсулированное хранилище с потокобезопасным доступом
type Storage struct {
	mu       sync.Mutex
	data     map[string]URLRecord
	filePath string
}

// NewStorage создаёт новое хранилище
func NewStorage() *Storage {
	return &Storage{
		data: make(map[string]URLRecord),
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
func (s *Storage) Init(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.filePath = path

	if _, err := os.Stat(path); err == nil {
		fileData, err := os.ReadFile(path)
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
			}
		}
	}

	return nil
}

// Save сохраняет данные в файл
func (s *Storage) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.filePath == "" {
		return nil
	}

	records := make([]URLRecord, 0, len(s.data))
	for _, record := range s.data { // ✅ берем готовые записи
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

// Insert генерирует UUID автоматически
func (s *Storage) Insert(originalURL string, shortURL string) error {
	// Генерируем UUID
	uuid := generateUUID()

	record := URLRecord{
		UUID:        uuid,
		ShortURL:    shortURL,
		OriginalURL: originalURL,
	}

	s.mu.Lock()
	s.data[shortURL] = record
	s.mu.Unlock()

	if err := s.Save(); err != nil {
		return fmt.Errorf("ошибка сохранения: %w", err)
	}

	return nil
}

// Select возвращает оригинальный URL по короткому
func (s *Storage) Select(shortURL string) (URLRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.data[shortURL]
	return record, ok
}
