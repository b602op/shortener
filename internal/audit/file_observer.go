package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// FileObserver пишет события аудита в файл, по одному JSON на строку
type FileObserver struct {
	mu       sync.Mutex
	filePath string
}

// NewFileObserver создаёт файловый приёмник.
// Файл открывается на дозапись, создаётся при необходимости.
func NewFileObserver(filePath string) (*FileObserver, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл аудита: %w", err)
	}
	f.Close()
	return &FileObserver{filePath: filePath}, nil
}

// Notify дописывает событие в конец файла на новой строке
func (o *FileObserver) Notify(_ context.Context, event Event) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга события: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(o.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла аудита: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("ошибка записи в файл аудита: %w", err)
	}
	return nil
}
