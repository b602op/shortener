// Package worker асинхронно обрабатывает задачи на удаление ссылок,
// накапливая их в буфере и применяя батчами.
package worker

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/b602op/shortener/internal/repository"
)

// ErrQueueFull возвращается, когда очередь удаления переполнена или сервис закрыт,
// и задача не принята к обработке.
var ErrQueueFull = errors.New("delete queue is full")

// DeleteTask — задача на удаление одной короткой ссылки у указанного пользователя.
//
// generate:reset
type DeleteTask struct {
	UserID   string
	ShortURL string
}

// Config задаёт пропускную способность сервиса асинхронного удаления:
// число воркеров, размеры очереди и буфера, период сброса и ожидание слота.
type Config struct {
	WorkerCount    int
	QueueSize      int
	BufferSize     int
	FlushInterval  time.Duration
	EnqueueTimeout time.Duration
}

// DefaultConfig возвращает конфигурацию со значениями по умолчанию:
// 5 воркеров, очередь 1024, буфер 100, сброс раз в секунду, ожидание слота 100 мс.
func DefaultConfig() Config {
	return Config{
		WorkerCount:    5,
		QueueSize:      1024,
		BufferSize:     100,
		FlushInterval:  time.Second,
		EnqueueTimeout: 100 * time.Millisecond,
	}
}

// DeleteService принимает задачи на удаление и применяет их батчами:
// воркеры сходятся в один канал (fanIn), коллектор копит буфер и сбрасывает
// его в хранилище по заполнению или по таймеру.
type DeleteService struct {
	store repository.Store
	cfg   Config

	mu        sync.RWMutex
	closed    bool
	closeOnce sync.Once

	tasks       chan DeleteTask
	fanIn       chan DeleteTask
	workerWg    sync.WaitGroup
	collectorWg sync.WaitGroup
}

// NewDeleteService создаёт сервис удаления и сразу запускает воркеры и коллектор.
// Неположительные значения cfg заменяются безопасными минимумами, поэтому
// допустим и нулевой Config.
func NewDeleteService(store repository.Store, cfg Config) *DeleteService {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 1
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 1
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = time.Second
	}
	if cfg.EnqueueTimeout <= 0 {
		cfg.EnqueueTimeout = 100 * time.Millisecond
	}

	s := &DeleteService{
		store: store,
		cfg:   cfg,
		tasks: make(chan DeleteTask, cfg.QueueSize),
		fanIn: make(chan DeleteTask),
	}

	s.collectorWg.Add(1)
	go s.collect()

	for i := 0; i < cfg.WorkerCount; i++ {
		s.workerWg.Add(1)
		go s.work()
	}

	return s
}

// Delete ставит одну задачу на асинхронное удаление, ожидая свободный слот
// не дольше EnqueueTimeout. Возвращает ErrQueueFull, если сервис закрыт
// или очередь переполнена, — паника в закрытый канал исключена.
func (s *DeleteService) Delete(userID, shortURL string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		slog.Warn("Сервис удаления закрыт, задача пропущена", "short_url", shortURL)
		return ErrQueueFull
	}

	task := DeleteTask{UserID: userID, ShortURL: shortURL}

	select {
	case s.tasks <- task:
		return nil
	case <-time.After(s.cfg.EnqueueTimeout):
		slog.Warn("Очередь удаления переполнена, задача пропущена", "short_url", shortURL)
		return ErrQueueFull
	}
}

// DeleteBatch ставит в очередь все переданные короткие ссылки одного пользователя.
// Останавливается на первой ошибке и возвращает её, например ErrQueueFull.
func (s *DeleteService) DeleteBatch(userID string, shortURLs []string) error {
	for _, shortURL := range shortURLs {
		if err := s.Delete(userID, shortURL); err != nil {
			return err
		}
	}
	return nil
}

func (s *DeleteService) work() {
	defer s.workerWg.Done()

	for task := range s.tasks {
		s.fanIn <- task
	}
}

// collect — коллектор: наполняет буфер задачами от всех воркеров
// и сбрасывает его в хранилище при заполнении или по таймеру
func (s *DeleteService) collect() {
	defer s.collectorWg.Done()

	buffer := make([]DeleteTask, 0, s.cfg.BufferSize)
	ticker := time.NewTicker(s.cfg.FlushInterval)
	defer ticker.Stop()

	flush := func() {
		s.flush(buffer)
		buffer = buffer[:0]
	}

	for {
		select {
		case task, ok := <-s.fanIn:
			if !ok {
				flush()
				return
			}
			buffer = append(buffer, task)
			if len(buffer) >= s.cfg.BufferSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// flush выполняет батч-обновление, группируя задачи по пользователям
func (s *DeleteService) flush(buffer []DeleteTask) {
	if len(buffer) == 0 {
		return
	}

	idsByUser := make(map[string][]string)
	for _, task := range buffer {
		idsByUser[task.UserID] = append(idsByUser[task.UserID], task.ShortURL)
	}

	for userID, ids := range idsByUser {
		if err := s.store.DeleteByUser(userID, ids); err != nil {
			slog.Error("Ошибка удаления URL", "error", err)
		}
	}
}

// Close останавливает приём задач и дожидается обработки всех оставшихся.
// Идемпотентен: повторный вызов не паникует.
func (s *DeleteService) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		close(s.tasks)
		s.mu.Unlock()

		s.workerWg.Wait()
		close(s.fanIn)
		s.collectorWg.Wait()
	})
	return nil
}
