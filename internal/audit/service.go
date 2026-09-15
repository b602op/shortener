package audit

import (
	"context"
	"log/slog"
	"sync"
)

// Service — издатель событий аудита (Subject в паттерне «Наблюдатель»)
type Service struct {
	mu        sync.RWMutex
	observers []Observer
}

// NewService создаёт пустой сервис аудита
func NewService() *Service {
	return &Service{}
}

// Subscribe добавляет наблюдателя
func (s *Service) Subscribe(o Observer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, o)
}

// NotifyAll уведомляет всех наблюдателей о событии.
// Ошибки наблюдателей логируются, но не прерывают рассылку.
func (s *Service) NotifyAll(ctx context.Context, event Event) {
	s.mu.RLock()
	observers := make([]Observer, len(s.observers))
	copy(observers, s.observers)
	s.mu.RUnlock()

	for _, o := range observers {
		if err := o.Notify(ctx, event); err != nil {
			slog.Error("Ошибка аудита", "error", err)
		}
	}
}

// HasObservers возвращает true, если есть хотя бы один наблюдатель
func (s *Service) HasObservers() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.observers) > 0
}
