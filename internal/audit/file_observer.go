package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// FileObserver пишет события аудита в файл, по одному JSON-объекту на строку.
// Потокобезопасен за счёт мьютекса.
//
// После использования необходимо вызвать Close, чтобы освободить файловый
// дескриптор.
type FileObserver struct {
	mu       sync.Mutex
	file     *os.File
	closeErr error
}

// NewFileObserver создаёт приёмник, пишущий в файл filePath.
// Файл открывается на дозапись и создаётся при необходимости; ошибка возвращается,
// если путь недоступен для записи.
//
// Вызывающий код обязан вызвать Close после завершения работы.
func NewFileObserver(filePath string) (*FileObserver, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл аудита: %w", err)
	}
	return &FileObserver{file: f}, nil
}

// Notify дописывает событие в конец файла отдельной строкой.
// Контекст не используется; ошибка возвращается при сбое маршалинга или записи.
func (o *FileObserver) Notify(_ context.Context, event Event) error {
	// 1. Маршалинг — вне мьютекса (работает только с event)
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга события: %w", err)
	}
	data = append(data, '\n')

	// 2. Запись — под мьютексом (единственная операция с общим ресурсом)
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.file == nil {
		return fmt.Errorf("file observer is closed")
	}

	if _, err := o.file.Write(data); err != nil {
		return fmt.Errorf("ошибка записи в файл аудита: %w", err)
	}
	return nil
}

// Close закрывает файл аудита и освобождает файловый дескриптор.
// Безопасно вызывать повторно.
func (o *FileObserver) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.file == nil {
		return o.closeErr
	}

	o.closeErr = o.file.Close()
	o.file = nil

	if o.closeErr != nil {
		return fmt.Errorf("не удалось закрыть файл аудита: %w", o.closeErr)
	}

	return nil
}
