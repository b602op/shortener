package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/b602op/shortener/internal/repository"
)

const defaultFileStoragePath = "data/storage.json"

type Config struct {
	AuditFile            string `env:"AUDIT_FILE"`
	AuditURL             string `env:"AUDIT_URL"`
	ServerAddress        string
	BaseURL              string
	FileStoragePath      string
	DatabaseDSN          string
	storage              repository.Store
	DeleteWorkerCount    int           `env:"DELETE_WORKER_COUNT" envDefault:"5"`
	DeleteQueueSize      int           `env:"DELETE_QUEUE_SIZE" envDefault:"1024"`
	DeleteBufferSize     int           `env:"DELETE_BUFFER_SIZE" envDefault:"100"`
	DeleteFlushInterval  time.Duration `env:"DELETE_FLUSH_INTERVAL" envDefault:"1s"`
	DeleteEnqueueTimeout time.Duration `env:"DELETE_ENQUEUE_TIMEOUT" envDefault:"100ms"`
}

func New() (*Config, error) {
	serverAddress := flag.String("a", "localhost:8080", "адрес запуска HTTP-сервера")
	baseURL := flag.String("b", "http://localhost:8080", "базовый адрес результирующего сокращённого URL")
	fileStoragePath := flag.String("f", "", "путь до файла для хранения данных")
	databaseDSN := flag.String("d", "", "DSN для подключения к PostgreSQL")
	auditFile := flag.String("audit-file", "", "путь к файлу аудита")
	auditURL := flag.String("audit-url", "", "URL сервера аудита")

	flag.Parse()

	serverAddressValue := getEnvOrFlag("SERVER_ADDRESS", *serverAddress)
	baseURLValue := getEnvOrFlag("BASE_URL", *baseURL)
	databaseDSNValue := getEnvOrFlag("DATABASE_DSN", *databaseDSN)
	fileStoragePathValue := getFileStoragePath(*fileStoragePath)
	auditFileValue := getEnvOrFlag("AUDIT_FILE", *auditFile)
	auditURLValue := getEnvOrFlag("AUDIT_URL", *auditURL)

	config := &Config{
		ServerAddress:   serverAddressValue,
		BaseURL:         baseURLValue,
		FileStoragePath: fileStoragePathValue,
		DatabaseDSN:     databaseDSNValue,
		AuditFile:       auditFileValue,
		AuditURL:        auditURLValue,

		// Параметры воркера удаления. Значения по умолчанию,
		// переопределяются через env DELETE_*.
		DeleteWorkerCount:    getEnvInt("DELETE_WORKER_COUNT", 5),
		DeleteQueueSize:      getEnvInt("DELETE_QUEUE_SIZE", 1024),
		DeleteBufferSize:     getEnvInt("DELETE_BUFFER_SIZE", 100),
		DeleteFlushInterval:  getEnvDuration("DELETE_FLUSH_INTERVAL", time.Second),
		DeleteEnqueueTimeout: getEnvDuration("DELETE_ENQUEUE_TIMEOUT", 100*time.Millisecond),
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Выбираем хранилище: PostgreSQL → файл → память
	store, err := config.createStore()
	if err != nil {
		slog.Warn("Ошибка создания хранилища, используется память", "error", err)
		config.storage = repository.NewFileStorage()
	} else {
		config.storage = store
	}

	return config, nil
}

func (c *Config) createStore() (repository.Store, error) {
	// 1. Пробуем PostgreSQL
	if c.DatabaseDSN != "" {
		slog.Info("Используется хранилище PostgreSQL")
		dbStore, err := repository.NewDBStorage(c.DatabaseDSN)
		if err != nil {
			return nil, fmt.Errorf("ошибка создания хранилища PostgreSQL: %w", err)
		}
		if err := dbStore.Init(); err != nil {
			dbStore.Close()
			return nil, fmt.Errorf("ошибка инициализации хранилища PostgreSQL: %w", err)
		}
		return dbStore, nil
	}

	// 2. Пробуем файл
	if c.FileStoragePath != "" {
		slog.Info("Используется файловое хранилище", "path", c.FileStoragePath)
		fileStore := repository.NewFileStorage()
		if err := fileStore.Init(c.FileStoragePath); err != nil {
			return nil, fmt.Errorf("ошибка инициализации файлового хранилища: %w", err)
		}
		return fileStore, nil
	}

	// 3. Память
	slog.Info("Используется хранилище в памяти")
	return nil, fmt.Errorf("нет параметров для хранилища")
}

func getEnvOrFlag(envVar, flagValue string) string {
	if envValue := os.Getenv(envVar); envValue != "" {
		return envValue
	}
	return flagValue
}

func getFileStoragePath(flagValue string) string {
	if envValue := os.Getenv("FILE_STORAGE_PATH"); envValue != "" {
		return envValue
	}
	if flagValue != "" {
		return flagValue
	}
	return defaultFileStoragePath
}

// getEnvInt читает env-переменную как int. При ошибке парсинга возвращает defaultValue.
func getEnvInt(envVar string, defaultValue int) int {
	raw := os.Getenv(envVar)
	if raw == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		slog.Warn("Некорректное значение env, используется дефолт", "env", envVar, "value", raw, "default", defaultValue)
		return defaultValue
	}
	return v
}

// getEnvDuration читает env-переменную как time.Duration.
// При ошибке парсинга возвращает defaultValue.
func getEnvDuration(envVar string, defaultValue time.Duration) time.Duration {
	raw := os.Getenv(envVar)
	if raw == "" {
		return defaultValue
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("Некорректное значение env, используется дефолт", "env", envVar, "value", raw, "default", defaultValue)
		return defaultValue
	}
	return v
}

func NewTest() *Config {
	return &Config{
		ServerAddress:   "localhost:8080",
		BaseURL:         "http://localhost:8080",
		FileStoragePath: "test_storage.json",
		DatabaseDSN:     "",
		storage:         repository.NewFileStorage(),
	}
}

func (c *Config) SetStorage(s repository.Store) {
	c.storage = s
}

func (c *Config) GetStorage() repository.Store {
	if c.storage == nil {
		c.storage = repository.NewFileStorage()
	}
	return c.storage
}

func (c *Config) Validate() error {
	if c.ServerAddress == "" {
		return fmt.Errorf("адрес сервера не может быть пустым")
	}

	if c.BaseURL == "" {
		return fmt.Errorf("базовый URL не может быть пустым")
	}

	return nil
}

func (c *Config) GetServerAddress() string {
	return c.ServerAddress
}

func (c *Config) GetBaseURL() string {
	return c.BaseURL
}

func (c *Config) GetFileStoragePath() string {
	return c.FileStoragePath
}

func (c *Config) GetDatabaseDSN() string {
	return c.DatabaseDSN
}
