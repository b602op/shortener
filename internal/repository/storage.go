// Package repository содержит хранилища сокращённых URL: в памяти с файлом
// и в PostgreSQL.
package repository

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// URLRecord — запись о сокращённой ссылке: идентификаторы, исходный адрес,
// владелец и признак удаления.
//
// generate:reset
type URLRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id,omitempty"`
	DeletedFlag bool   `json:"is_deleted"`
}

// Store описывает контракт хранилища сокращённых URL.
// Реализации: FileStorage (в памяти + файл) и DBStorage (PostgreSQL).
// Реализации должны быть безопасны для конкурентного вызова из нескольких горутин.
type Store interface {
	Insert(userID string, originalURL string, shortURL string) error
	BatchInsert(userID string, records []URLRecord) ([]URLRecord, error)
	Select(shortURL string) (URLRecord, bool)
	SelectByUser(userID string) []URLRecord
	DeleteByUser(userID string, shortURLs []string) error

	// CountURLs возвращает общее количество сокращённых URL.
	CountURLs() (int, error)

	// CountUsers возвращает количество уникальных пользователей,
	// которые сокращали URL.
	CountUsers() (int, error)
}

// FileStorage хранит записи в памяти и при наличии пути дублирует их в файл.
// Поиски по короткому адресу и по исходному выполняются за O(1) за счёт
// обратного индекса originalURL.
type FileStorage struct {
	mu            sync.Mutex
	data          map[string]URLRecord // shortURL -> record
	originalIndex map[string]URLRecord // originalURL -> record (обратный индекс для O(1) поиска дубликатов)
	userURLs      map[string][]string  // userID -> []shortURL
	filePath      string
}

// NewFileStorage создаёт хранилище в памяти без привязки к файлу.
// Для включения persistence нужен вызов Init.
func NewFileStorage() *FileStorage {
	return &FileStorage{
		data:          make(map[string]URLRecord),
		originalIndex: make(map[string]URLRecord),
		userURLs:      make(map[string][]string),
	}
}

// generateUUID создаёт случайный UUID v4.
func generateUUID() string {
	return uuid.New().String()
}

// Init инициализирует хранилище, загружая данные из файла по указанному пути.
//
// Формат файла: JSON-lines — по одному JSON-объекту URLRecord на строку.
// Пустые строки игнорируются.
//
// ВАЖНО: сервис не поддерживает старый формат (JSON-массив).
//
// Если path указывает на существующую директорию, используется файл storage.json в ней.
// Отсутствие файла не ошибка — хранилище остаётся пустым.
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
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("ошибка открытия файла: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Проверяем первый непробельный байт: '[' — старый формат (JSON-массив).
	reader := bufio.NewReader(f)
	firstByte, err := peekFirstNonSpace(reader)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil // пустой файл
		}
		return fmt.Errorf("ошибка чтения файла: %w", err)
	}
	if firstByte == '[' {
		return fmt.Errorf(
			"файл %s использует устаревший формат JSON-массив; "+
				"сервис поддерживает только JSON-lines — см. README.md",
			s.filePath,
		)
	}

	// Читаем JSON-lines через decoder — он сам обрабатывает поток объектов.
	decoder := json.NewDecoder(reader)
	lineNum := 0
	for decoder.More() {
		lineNum++

		var record URLRecord
		if err := decoder.Decode(&record); err != nil {
			return fmt.Errorf("ошибка парсинга JSON (запись #%d): %w", lineNum, err)
		}

		s.data[record.ShortURL] = record
		s.originalIndex[record.OriginalURL] = record
		if record.UserID != "" {
			s.userURLs[record.UserID] = append(s.userURLs[record.UserID], record.ShortURL)
		}
	}

	return nil
}

// peekFirstNonSpace возвращает первый непробельный байт из reader,
// не сдвигая позицию чтения. Если файл пуст — io.EOF.
func peekFirstNonSpace(reader *bufio.Reader) (byte, error) {
	for {
		b, err := reader.Peek(1)
		if err != nil {
			return 0, err
		}
		if b[0] == ' ' || b[0] == '\t' || b[0] == '\n' || b[0] == '\r' {
			if _, err := reader.ReadByte(); err != nil {
				return 0, err
			}
			continue
		}
		return b[0], nil
	}
}

// Insert добавляет пару originalURL→shortURL, привязывая её к userID.
// Возвращает ErrDuplicateURL, если originalURL уже есть в хранилище.
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

// BatchInsert добавляет несколько записей одним вызовом и возвращает результаты
// в том же порядке с фактическим short_url: для дубликатов — существующий,
// для новых — вставленный.
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

// saveFile сохраняет данные в файл
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
	defer func() { _ = f.Close() }()

	encoder := json.NewEncoder(f)
	for _, record := range s.data {
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("ошибка сериализации: %w", err)
		}
	}
	return nil
}

// Save записывает все текущие данные в файл; без установленного пути не делает ничего.
func (s *FileStorage) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveFile()
}

// Close сохраняет данные в файл; хранилище в памяти остаётся пригодным к использованию.
func (s *FileStorage) Close() error {
	return s.Save()
}

// Select возвращает запись по короткому адресу и false, если такой записи нет.
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

// CountURLs возвращает количество записей в хранилище.
func (s *FileStorage) CountURLs() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.data), nil
}

// CountUsers возвращает количество уникальных user_id.
func (s *FileStorage) CountUsers() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.userURLs), nil
}

// DeleteByUser помечает переданные URL удалёнными, но только принадлежащие userID.
// Неизвестные или чужие идентификаторы молча пропускаются.
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
