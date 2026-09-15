package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/b602op/shortener/internal/audit"
	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/handler"
	"github.com/b602op/shortener/internal/repository"
	"github.com/b602op/shortener/internal/worker"
)

func getSecretKey() string {
	if key := os.Getenv("AUTH_SECRET_KEY"); key != "" {
		return key
	}
	return "super-secret-key-change-me"
}

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Ошибка конфигурации: %v", err)
	}

	store := cfg.GetStorage()

	// Сохраняем данные при завершении сервера
	if fileStore, ok := store.(*repository.FileStorage); ok {
		defer fileStore.Close()
	}

	// Если используется DBStorage, закрываем подключение при завершении
	if dbStore, ok := store.(*repository.DBStorage); ok {
		defer dbStore.Close()
	}

	addr := cfg.GetServerAddress()
	baseURL := cfg.GetBaseURL()

	log.Printf("Сервер запускается на %s", addr)
	log.Printf("Базовый URL: %s", baseURL)

	authService := auth.NewService(getSecretKey())
	auditService := audit.NewService()

	if cfg.AuditFile != "" {
		fileObs, err := audit.NewFileObserver(cfg.AuditFile)
		if err != nil {
			log.Fatalf("Ошибка инициализации файлового аудита: %v", err)
		}
		auditService.Subscribe(fileObs)
		log.Printf("Аудит в файл: %s", cfg.AuditFile)
	}

	if cfg.AuditURL != "" {
		httpObs := audit.NewHTTPObserver(cfg.AuditURL)
		auditService.Subscribe(httpObs)
		log.Printf("Аудит на сервер: %s", cfg.AuditURL)
	}

	if cfg.AuditFile == "" && cfg.AuditURL == "" {
		log.Println("Аудит отключён")
	}

	// Асинхронное удаление URL по паттерну fanIn
	workerCfg := worker.Config{
		WorkerCount:    cfg.DeleteWorkerCount,
		QueueSize:      cfg.DeleteQueueSize,
		BufferSize:     cfg.DeleteBufferSize,
		FlushInterval:  cfg.DeleteFlushInterval,
		EnqueueTimeout: cfg.DeleteEnqueueTimeout,
	}
	deleteService := worker.NewDeleteService(store, workerCfg)
	defer deleteService.Close()

	deps := handler.Dependencies{
		Config:        cfg,
		Store:         store,
		AuthService:   authService,
		DeleteService: deleteService,
		AuditService:  auditService,
	}

	httpHandler := handler.Handler(deps)

	server := &http.Server{
		Addr:    addr,
		Handler: httpHandler,
	}

	// Запуск в горутине
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	// Ожидание сигнала завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Завершение работы сервера...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Ошибка завершения сервера: %v", err)
	}

	log.Println("Сервер завершил работу")
}
