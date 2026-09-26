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
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/b602op/shortener/gen/pb/shortener/v1"
	"github.com/b602op/shortener/internal/audit"
	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/grpcserver"
	"github.com/b602op/shortener/internal/handler"
	"github.com/b602op/shortener/internal/repository"
	"github.com/b602op/shortener/internal/service"
	"github.com/b602op/shortener/internal/worker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
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
		log.Fatalf("Ошибка: %v", err)
	}
}

func run() error {
	fmt.Println(formatBuildInfo(buildVersion, buildDate, buildCommit))

	cfg, err := config.New()
	if err != nil {
		return err
	}

	// === Ресурсы, требующие Close ===
	// Все append в closers живут только в run(), здесь — в одном месте.
	// Замыкание вместо defer closeAll(closers): слайс передаётся по значению
	// в момент defer, поэтому позднее добавление ресурсов было бы потеряно.
	var closers []io.Closer
	defer func() { closeAll(closers) }()

	// 1. Хранилище
	store := cfg.GetStorage()
	if closer, ok := store.(io.Closer); ok {
		closers = append(closers, closer)
	}

	addr := cfg.GetServerAddress()
	baseURL := cfg.GetBaseURL()

	log.Printf("Сервер запускается на %s", addr)
	log.Printf("Базовый URL: %s", baseURL)

	// 2. Секретный ключ
	secretKey, err := getSecretKey()
	if err != nil {
		return err
	}
	authService := auth.NewService(secretKey)

	// 3. Общий слой бизнес-логики (фасад для HTTP и gRPC)
	shortenerService := service.NewShortenerService(store, baseURL)

	// 4. Аудит
	auditService, auditClosers, err := setupAudit(cfg)
	if err != nil {
		return err
	}
	closers = append(closers, auditClosers...)

	// 5. Worker
	deleteService, workerClosers := setupWorker(cfg, store)
	closers = append(closers, workerClosers...)

	// === HTTP-сервер ===
	httpHandler := setupHandler(cfg, store, shortenerService, authService, deleteService, auditService)
	server := &http.Server{Addr: addr, Handler: httpHandler}

	// === gRPC-сервер (параллельно с HTTP, отдельный порт) ===
	grpcServer, grpcListener, err := setupGRPC(cfg, shortenerService, authService)
	if err != nil {
		return err
	}

	// === Контекст с сигналом ===
	// SIGQUIT обрабатывается так же, как SIGINT/SIGTERM — graceful shutdown
	// важнее дампа стека.
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer stop()

	// === Запуск серверов ===
	serverErr := make(chan error, 1)
	go serveServer(server, cfg, addr, serverErr)

	grpcErr := make(chan error, 1)
	if grpcServer != nil {
		go serveGRPC(grpcServer, grpcListener, cfg.GetGRPCAddress(), grpcErr)
	}

	// === Ожидание сигнала или ошибки серверов ===
	select {
	case srvErr := <-serverErr:
		stop()
		return srvErr
	case srvErr := <-grpcErr:
		stop()
		return srvErr
	case <-ctx.Done():
		log.Println("Получен сигнал завершения, начинаем graceful shutdown...")
	}

	// === Graceful shutdown ===
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown ждёт завершения активных запросов (до 5s).
	err = server.Shutdown(shutdownCtx)
	if err != nil {
		log.Printf("Ошибка завершения сервера: %v", err)
	}
	log.Println("HTTP-сервер остановлен")

	if grpcServer != nil {
		grpcServer.GracefulStop()
		log.Println("gRPC-сервер остановлен")
	}

	return nil
}

// closeAll закрывает ресурсы в обратном порядке (LIFO), логируя каждый шаг.
// Все ресурсы регистрируются в run() — порядок в closers определяет порядок закрытия.
func closeAll(closers []io.Closer) {
	for i := len(closers) - 1; i >= 0; i-- {
		log.Printf("Закрытие ресурса: %T", closers[i])
		if err := closers[i].Close(); err != nil {
			log.Printf("Ошибка закрытия ресурса: %v", err)
		}
	}
}

// setupAudit создаёт сервис аудита и подписывает observers согласно конфигу.
// Возвращает сервис и созданные ресурсы, требующие Close:
// сначала svc, потом fileObs — при LIFO-закрытии fileObs закроется раньше svc.
func setupAudit(cfg *config.Config) (*audit.Service, []io.Closer, error) {
	var closers []io.Closer

	svc := audit.NewService()
	closers = append(closers, svc)

	if cfg.AuditFile != "" {
		fileObs, err := audit.NewFileObserver(cfg.AuditFile)
		if err != nil {
			return nil, nil, err
		}
		closers = append(closers, fileObs)
		svc.Subscribe(fileObs)
		log.Printf("Аудит в файл: %s", cfg.AuditFile)
	}

	if cfg.AuditURL != "" {
		httpObs := audit.NewHTTPObserver(cfg.AuditURL)
		svc.Subscribe(httpObs)
		log.Printf("Аудит на сервер: %s", cfg.AuditURL)
	}

	if cfg.AuditFile == "" && cfg.AuditURL == "" {
		log.Println("Аудит отключён")
	}

	return svc, closers, nil
}

// setupWorker создаёт сервис асинхронного удаления.
// Возвращает сервис и слайс ресурсов, требующих Close, — сам сервис.
func setupWorker(cfg *config.Config, store repository.Store) (*worker.DeleteService, []io.Closer) {
	workerCfg := worker.Config{
		WorkerCount:    cfg.DeleteWorkerCount,
		QueueSize:      cfg.DeleteQueueSize,
		BufferSize:     cfg.DeleteBufferSize,
		FlushInterval:  cfg.DeleteFlushInterval,
		EnqueueTimeout: cfg.DeleteEnqueueTimeout,
	}
	svc := worker.NewDeleteService(store, workerCfg)
	return svc, []io.Closer{svc}
}

// setupHandler создаёт HTTP-обработчик со всеми зависимостями.
func setupHandler(
	cfg *config.Config,
	store repository.Store,
	shortenerService *service.ShortenerService,
	authService *auth.Service,
	deleteService *worker.DeleteService,
	auditService *audit.Service,
) http.Handler {
	deps := handler.Dependencies{
		Config:           cfg,
		Store:            store,
		ShortenerService: shortenerService,
		AuthService:      authService,
		DeleteService:    deleteService,
		AuditService:     auditService,
	}
	return handler.Handler(deps)
}

// setupGRPC создаёт gRPC-сервер и слушатель на отдельном порту.
// Если GRPCAddress пуст — gRPC отключён, возвращается (nil, nil, nil).
// TLS включается теми же настройками, что и у HTTP-сервера.
func setupGRPC(
	cfg *config.Config,
	shortenerService *service.ShortenerService,
	authService *auth.Service,
) (*grpc.Server, net.Listener, error) {
	if cfg.GetGRPCAddress() == "" {
		log.Println("gRPC-сервер отключён (grpc_address пуст)")
		return nil, nil, nil
	}

	listener, err := net.Listen("tcp", cfg.GetGRPCAddress())
	if err != nil {
		// Занятый порт не должен ронять HTTP-сервер: логируем и работаем без gRPC.
		log.Printf("gRPC-сервер не запущен, ошибка слушателя %s: %v", cfg.GetGRPCAddress(), err)
		return nil, nil, nil
	}

	var opts []grpc.ServerOption

	// TLS — те же настройки, что у HTTP
	if cfg.GetEnableHTTPS() {
		creds, err := credentials.NewServerTLSFromFile(cfg.GetTLSCertFile(), cfg.GetTLSKeyFile())
		if err != nil {
			_ = listener.Close()
			return nil, nil, fmt.Errorf("gRPC TLS: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	// Интерцептор логирования всех unary-запросов
	opts = append(opts, grpc.UnaryInterceptor(grpcLoggingInterceptor))

	server := grpc.NewServer(opts...)
	pb.RegisterShortenerServiceServer(server, grpcserver.NewServer(shortenerService, authService))

	return server, listener, nil
}

// serveGRPC запускает gRPC-сервер на переданном слушателе.
// Ошибки отправляются в канал grpcErr.
func serveGRPC(server *grpc.Server, listener net.Listener, addr string, grpcErr chan<- error) {
	log.Printf("gRPC-сервер слушает %s", addr)
	if err := server.Serve(listener); err != nil {
		grpcErr <- err
	}
}

// grpcLoggingInterceptor логирует unary-запросы gRPC: метод и итоговый статус.
func grpcLoggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	log.Printf("gRPC %s %s %v", info.FullMethod, statusString(err), time.Since(start))
	return resp, err
}

// statusString возвращает строковый код статуса gRPC-ошибки или "OK".
func statusString(err error) string {
	if err == nil {
		return "OK"
	}
	if s, ok := status.FromError(err); ok {
		return s.Code().String()
	}
	return "Unknown"
}

// serveServer запускает HTTP или HTTPS сервер в зависимости от конфигурации.
// Ошибки отправляются в канал serverErr.
func serveServer(server *http.Server, cfg *config.Config, addr string, serverErr chan<- error) {
	if cfg.GetEnableHTTPS() {
		certFile := cfg.GetTLSCertFile()
		keyFile := cfg.GetTLSKeyFile()

		if _, statErr := os.Stat(certFile); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				serverErr <- fmt.Errorf(
					"TLS-сертификат не найден по пути %q. Запустите 'make certs' для генерации: %w",
					certFile, statErr,
				)
			} else {
				serverErr <- fmt.Errorf("TLS-сертификат %q недоступен: %w", certFile, statErr)
			}
			return
		}
		if _, statErr := os.Stat(keyFile); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				serverErr <- fmt.Errorf(
					"TLS-ключ не найден по пути %q. Запустите 'make certs' для генерации: %w",
					keyFile, statErr,
				)
			} else {
				serverErr <- fmt.Errorf("TLS-ключ %q недоступен: %w", keyFile, statErr)
			}
			return
		}

		log.Printf("HTTPS-сервер слушает %s (cert: %s)", addr, certFile)
		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		return
	}

	log.Printf("HTTP-сервер слушает %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		serverErr <- err
	}
}
