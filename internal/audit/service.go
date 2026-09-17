package audit

import (
	"context"
	"io"
	"log/slog"
	"sync"
)

const (
	// observerQueueSize — размер буфера канала каждого observer'а.
	observerQueueSize = 100
)

// Service — издатель событий аудита (Subject в паттерне «Наблюдатель»).
//
// Каждый observer обрабатывается в отдельной горутине: NotifyAll кладёт
// событие в канал observer'а и не ждёт его обработки. Это защищает
// вызывающий код от медленных приёмников.
type Service struct {
	mu        sync.RWMutex
	observers []*observerWorker
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

var _ io.Closer = (*Service)(nil)

// observerWorker — внутренняя обёртка над Observer с каналом и горутиной.
type observerWorker struct {
	observer Observer
	events   chan Event
}

// NewService создаёт сервис аудита без подписчиков.
func NewService() *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Subscribe добавляет наблюдателя и запускает для него фоновую горутину.
// Дубликаты не отслеживаются.
func (s *Service) Subscribe(o Observer) {
	w := &observerWorker{
		observer: o,
		events:   make(chan Event, observerQueueSize),
	}

	s.mu.Lock()
	s.observers = append(s.observers, w)
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runObserver(w)
}

// runObserver читает события из канала и передаёт их observer'у.
// Завершается при закрытии канала или отмене контекста.
func (s *Service) runObserver(w *observerWorker) {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-w.events:
			if !ok {
				return
			}
			if err := w.observer.Notify(s.ctx, event); err != nil {
				slog.Error("Ошибка аудита", "error", err)
			}
		}
	}
}

// NotifyAll передаёт событие каждому подписчику асинхронно.
//
// Если канал observer'а переполнен, событие для него пропускается
// с логированием — это защищает от блокировки handler'а медленным приёмником.
func (s *Service) NotifyAll(_ context.Context, event Event) {
	s.mu.RLock()
	observers := make([]*observerWorker, len(s.observers))
	copy(observers, s.observers)
	s.mu.RUnlock()

	for _, w := range observers {
		select {
		case w.events <- event:
		default:
			slog.Warn("Канал аудита переполнен, событие пропущено",
				"observer", w.observer)
		}
	}
}

// HasObservers сообщает, подписан ли хотя бы один наблюдатель.
func (s *Service) HasObservers() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.observers) > 0
}

// Close останавливает фоновые горутины и ждёт их завершения.
// Реализует io.Closer.
func (s *Service) Close() error {
	s.cancel()
	s.wg.Wait()
	return nil
}
