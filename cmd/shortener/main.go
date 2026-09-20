//go:generate go run ../../cmd/reset -dir ../..

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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
	"github.com/b602op/shortener/internal/worker"
)

// Информация о сборке. Заполняется через -ldflags при сборке:
//
//	go build -ldflags "-X main.buildVersion=1.0.0 -X main.buildDate=... -X main.buildCommit=..."
//
// Если переменные не заданы, при старте выводится "N/A".
var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

// formatBuildInfo формирует строку с информацией о сборке.
// Для пустых значений подставляет "N/A".
func formatBuildInfo(version, date, commit string) string {
	return fmt.Sprintf(
		"Build version: %s\nBuild date: %s\nBuild commit: %s",
		orNA(version),
		orNA(date),
		orNA(commit),
	)
}

// orNA возвращает value или "N/A", если value пустое.
func orNA(value string) string {
	if value == "" {
		return "N/A"
	}
	return value
}

// getSecretKey возвращает секретный ключ для JWT.
//
// Приоритет:
//  1. Переменная окружения AUTH_SECRET_KEY (для production).
//  2. Случайный ключ, сгенерированный для текущей сессии (для разработки
//     и автотестов). Токены не переживут перезапуск.
func getSecretKey() (string, error) {
	if key := os.Getenv("AUTH_SECRET_KEY"); key != "" {
		return key, nil
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("не удалось сгенерировать AUTH_SECRET_KEY: %w", err)
	}

	key := hex.EncodeToString(buf)
	log.Println("ВНИМАНИЕ: AUTH_SECRET_KEY не задан. " +
		"Сгенерирован случайный ключ для текущей сессии. " +
		"Все выданные токены станут недействительными после перезапуска. " +
		"Для production задайте AUTH_SECRET_KEY.")

	return key, nil
}

func main() {
	if err := run(); err != nil {
		log.Printf("Ошибка: %v", err)
	}
}

func run() error {
	fmt.Println(formatBuildInfo(buildVersion, buildDate, buildCommit))

	cfg, err := config.New()
	if err != nil {
		return err
	}

	// === Ресурсы, требующие Close ===
	var closers []io.Closer
	defer func() {
		for i := len(closers) - 1; i >= 0; i-- {
			if cerr := closers[i].Close(); cerr != nil {
				log.Printf("Ошибка закрытия ресурса: %v", cerr)
			}
		}
	}()

	store := cfg.GetStorage()
	if closer, ok := store.(io.Closer); ok {
		closers = append(closers, closer)
	}

	addr := cfg.GetServerAddress()
	baseURL := cfg.GetBaseURL()

	log.Printf("Сервер запускается на %s", addr)
	log.Printf("Базовый URL: %s", baseURL)

	// === Секретный ключ ===
	secretKey, err := getSecretKey()
	if err != nil {
		return err
	}
	authService := auth.NewService(secretKey)

	// === Аудит ===
	auditService := audit.NewService()
	closers = append(closers, auditService)

	if cfg.AuditFile != "" {
		fileObs, obsErr := audit.NewFileObserver(cfg.AuditFile)
		if obsErr != nil {
			return obsErr
		}
		closers = append(closers, fileObs)
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

	// === Worker ===
	workerCfg := worker.Config{
		WorkerCount:    cfg.DeleteWorkerCount,
		QueueSize:      cfg.DeleteQueueSize,
		BufferSize:     cfg.DeleteBufferSize,
		FlushInterval:  cfg.DeleteFlushInterval,
		EnqueueTimeout: cfg.DeleteEnqueueTimeout,
	}
	deleteService := worker.NewDeleteService(store, workerCfg)
	closers = append(closers, deleteService)

	// === HTTP-сервер ===
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

	// === Контекст с сигналом ===
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// === Запуск сервера ===
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("HTTP-сервер слушает %s", addr)
		if srvErr := server.ListenAndServe(); srvErr != nil && !errors.Is(srvErr, http.ErrServerClosed) {
			serverErr <- srvErr
		}
	}()

	// === Ожидание сигнала или ошибки сервера ===
	select {
	case srvErr := <-serverErr:
		return srvErr
	case <-ctx.Done():
		log.Println("Завершение работы сервера...")
	}

	// === Graceful shutdown ===
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		log.Printf("Ошибка завершения сервера: %v", err)
	}

	log.Println("Сервер завершил работу")
	return nil
}
