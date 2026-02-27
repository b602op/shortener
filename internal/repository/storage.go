package repository

import (
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

var (
	Storage      = make(map[string]string)
	storageMutex sync.Mutex
	filePath     string
)

func InitStorage(path string) error {
	storageMutex.Lock()
	defer storageMutex.Unlock()

	filePath = path

	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("ошибка чтения файла: %w", err)
		}

		if len(data) > 0 {
			var records []URLRecord
			if err := json.Unmarshal(data, &records); err != nil {
				return fmt.Errorf("ошибка парсинга JSON: %w", err)
			}

			for _, record := range records {
				Storage[record.ShortURL] = record.OriginalURL
			}
		}
	}

	return nil
}

func SaveStorage() error {
	storageMutex.Lock()
	defer storageMutex.Unlock()

	if filePath == "" {
		return nil
	}

	records := make([]URLRecord, 0, len(Storage))
	for shortURL, originalURL := range Storage {
		records = append(records, URLRecord{
			UUID:        shortURL,
			ShortURL:    shortURL,
			OriginalURL: originalURL,
		})
	}

	data, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("ошибка сериализации JSON: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("ошибка записи в файл: %w", err)
	}

	return nil
}

func InsertData(url string, shortURL string) {
	storageMutex.Lock()
	defer storageMutex.Unlock()

	Storage[shortURL] = url

	go SaveStorage()
}

func SelectData(shortURL string) string {
	storageMutex.Lock()
	defer storageMutex.Unlock()

	return Storage[shortURL]
}
