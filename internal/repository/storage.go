package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// URLRecord — структура записи URL
type URLRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id,omitempty"`
	DeletedFlag bool   `json:"is_deleted"`
}

// Store — интерфейс хранилища URL
type Store interface {
	Insert(userID string, originalURL string, shortURL string) error
	BatchInsert(userID string, records []URLRecord) ([]URLRecord, error)
	Select(shortURL string) (URLRecord, bool)
	SelectByUser(userID string) []URLRecord
	DeleteByUser(userID string, shortURLs []string) error
}

// FileStorage — хранение в памяти с опциональной записью в файл
type FileStorage struct {
	mu            sync.Mutex
	data          map[string]URLRecord // shortURL -> record
	originalIndex map[string]URLRecord // originalURL -> record (обратный индекс для O(1) поиска дубликатов)
	userURLs      map[string][]string  // userID -> []shortURL
	filePath      string
}

// NewFileStorage создаёт новое файловое хранилище
func NewFileStorage() *FileStorage {
	return &FileStorage{
		data:          make(map[string]URLRecord),
		originalIndex: make(map[string]URLRecord),
		userURLs:      make(map[string][]string),
	}
}

// TODO с оптимизацией // generateUUID создаёт случайный UUID v4.
func generateUUID() string {
	return uuid.New().String()
}

// func generateUUID() string {
// 	b := make([]byte, 16)
// 	_, _ = rand.Read(b)
// 	b[6] = (b[6] & 0x0f) | 0x40
// 	b[8] = (b[8] & 0x3f) | 0x80
// 	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
// }

// // TODO вернуть с оптимизацией // Init инициализирует хранилище, загружая данные из файла
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

	f, err := os.Open(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("ошибка открытия файла: %w", err)
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	for decoder.More() {
		var record URLRecord
		if err := decoder.Decode(&record); err != nil {
			return fmt.Errorf("ошибка парсинга JSON: %w", err)
		}
		s.data[record.ShortURL] = record
		s.originalIndex[record.OriginalURL] = record
		if record.UserID != "" {
			s.userURLs[record.UserID] = append(s.userURLs[record.UserID], record.ShortURL)
		}
	}
	return nil
}

// TODO это без оптимизации // Init инициализирует хранилище, загружая данные из файла
// func (s *FileStorage) Init(path string) error {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	// Пробуем определить: если путь существует и это папка
// 	if info, err := os.Stat(path); err == nil && info.IsDir() {
// 		s.filePath = filepath.Join(path, "storage.json")
// 	} else {
// 		// Во всех остальных случаях используем путь как есть
// 		s.filePath = path
// 	}

// 	if _, err := os.Stat(s.filePath); err == nil {
// 		fileData, err := os.ReadFile(s.filePath)
// 		if err != nil {
// 			return fmt.Errorf("ошибка чтения файла: %w", err)
// 		}

// 		if len(fileData) > 0 {
// 			var records []URLRecord
// 			if err := json.Unmarshal(fileData, &records); err != nil {
// 				return fmt.Errorf("ошибка парсинга JSON: %w", err)
// 			}

// 			for _, record := range records {
// 				s.data[record.ShortURL] = record
// 				s.originalIndex[record.OriginalURL] = record

// 				// восстанавливаем индекс пользователя
// 				if record.UserID != "" {
// 					s.userURLs[record.UserID] = append(s.userURLs[record.UserID], record.ShortURL)
// 				}
// 			}
// 		}
// 	}

// 	return nil
// }

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

	if s.userURLs == nil {
		s.userURLs = make(map[string][]string)
	}
	s.userURLs[userID] = append(s.userURLs[userID], shortURL)

	return s.saveFile()
}

// BatchInsert добавляет несколько записей в хранилище и возвращает результаты
// с actual short_url (для дубликатов — существующий, для новых — вставленный)
func (s *FileStorage) BatchInsert(userID string, records []URLRecord) ([]URLRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Инициализируем индекс пользователя, если его ещё нет
	if s.userURLs == nil {
		s.userURLs = make(map[string][]string)
	}

	results := make([]URLRecord, len(records))

	for i, record := range records {
		// Проверяем наличие дубликата по original_url через обратный индекс
		if existing, ok := s.originalIndex[record.OriginalURL]; ok {
			// Дубликат — возвращаем существующую запись целиком
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

		// Добавляем в индекс пользователя
		s.userURLs[userID] = append(s.userURLs[userID], record.ShortURL)

		// Возвращаем полную запись (с UUID и UserID)
		results[i] = entry
	}

	return results, s.saveFile()
}

// // TODO это с оптимизацией // saveFile сохраняет данные в файл
func (s *FileStorage) saveFile() error {
	if s.filePath == "" {
		return nil
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("ошибка создания папки: %w", err)
	}

	f, err := os.Create(s.filePath)
	if err != nil {
		return fmt.Errorf("ошибка создания файла: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, record := range s.data {
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("ошибка сериализации: %w", err)
		}
	}
	return nil
}

// TODO это без оптимизации// saveFile сохраняет данные в файл
// func (s *FileStorage) saveFile() error {
// 	if s.filePath == "" {
// 		return nil
// 	}

// 	dir := filepath.Dir(s.filePath)
// 	if err := os.MkdirAll(dir, 0755); err != nil {
// 		return fmt.Errorf("ошибка создания папки: %w", err)
// 	}

// 	records := make([]URLRecord, 0, len(s.data))
// 	for _, record := range s.data {
// 		records = append(records, record)
// 	}

// 	data, err := json.Marshal(records)
// 	if err != nil {
// 		return fmt.Errorf("ошибка сериализации JSON: %w", err)
// 	}

// 	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
// 		return fmt.Errorf("ошибка записи в файл: %w", err)
// 	}

// 	return nil
// }

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

// SelectByUser возвращает все URL, сокращённые указанным пользователем.
// Удалённые URL (DeletedFlag) не возвращаются.
func (s *FileStorage) SelectByUser(userID string) []URLRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	shortURLs, ok := s.userURLs[userID]
	if !ok || len(shortURLs) == 0 {
		return []URLRecord{}
	}

	records := make([]URLRecord, 0, len(shortURLs))
	for _, shortURL := range shortURLs {
		record, exists := s.data[shortURL]
		if !exists {
			continue
		}
		// Удалённые URL не показываем пользователю
		if record.DeletedFlag {
			continue
		}
		records = append(records, record)
	}

	return records
}

// DeleteByUser помечает URL удалёнными (только принадлежащие пользователю)
func (s *FileStorage) DeleteByUser(userID string, shortURLs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, shortURL := range shortURLs {
		record, ok := s.data[shortURL]
		// Удалить может только пользователь, создавший URL
		if !ok || record.UserID != userID {
			continue
		}

		record.DeletedFlag = true
		s.data[shortURL] = record
		s.originalIndex[record.OriginalURL] = record
	}

	return s.saveFile()
}
