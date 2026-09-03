package worker

import (
	"log/slog"
	"sync"
	"time"

	"github.com/b602op/shortener/internal/repository"
)

// DeleteTask — задача на удаление сокращённого URL
type DeleteTask struct {
	UserID   string
	ShortURL string
}

const (
	workerCount   = 5               // количество воркеров, читающих входную очередь
	taskQueueSize = 1024            // размер входной очереди задач
	bufferSize    = 100             // размер буфера для батч-обновления
	flushInterval = 1 * time.Second // интервал принудительного сброса буфера
)

// DeleteService — асинхронное удаление URL по паттерну fanIn:
// несколько воркеров читают задачи из входной очереди и пересылают их
// в единый канал; коллектор наполняет буфер и сбрасывает его в хранилище
// одним батч-обновлением.
type DeleteService struct {
	store repository.Store

	tasks       chan DeleteTask // входная очередь задач
	fanIn       chan DeleteTask // единый канал от всех воркеров
	workerWg    sync.WaitGroup
	collectorWg sync.WaitGroup
}

// NewDeleteService создаёт и запускает сервис асинхронного удаления
func NewDeleteService(store repository.Store) *DeleteService {
	s := &DeleteService{
		store: store,
		tasks: make(chan DeleteTask, taskQueueSize),
		fanIn: make(chan DeleteTask),
	}

	s.collectorWg.Add(1)
	go s.collect()

	for i := 0; i < workerCount; i++ {
		s.workerWg.Add(1)
		go s.work()
	}

	return s
}

// Delete ставит задачу на асинхронное удаление и не блокирует вызывающего
func (s *DeleteService) Delete(userID, shortURL string) {
	select {
	case s.tasks <- DeleteTask{UserID: userID, ShortURL: shortURL}:
	default:
		slog.Warn("Очередь удаления переполнена, задача пропущена", "short_url", shortURL)
	}
}

// work — воркер: читает задачи из очереди и пересылает их в канал fanIn
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

	buffer := make([]DeleteTask, 0, bufferSize)
	ticker := time.NewTicker(flushInterval)
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
			if len(buffer) >= bufferSize {
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

// Close останавливает приём задач и дожидается обработки всех оставшихся
func (s *DeleteService) Close() {
	close(s.tasks)
	s.workerWg.Wait()
	close(s.fanIn)
	s.collectorWg.Wait()
}
