// Package config собирает параметры сервиса из флагов и переменных окружения
// и выбирает хранилище URL.
package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/b602op/shortener/internal/repository"
)

const (
	defaultFileStoragePath = "data/storage.json"
	defaultTLSCertFile     = "./certs/cert.pem"
	defaultTLSKeyFile      = "./certs/key.pem"
)

// Config — настройки HTTP-сервера, базового адреса, хранилища, аудита
// и воркера удаления. Собирается функцией New из флагов и переменных окружения.
type Config struct {
	AuditFile            string `env:"AUDIT_FILE"`
	AuditURL             string `env:"AUDIT_URL"`
	ServerAddress        string
	BaseURL              string
	FileStoragePath      string
	DatabaseDSN          string
	EnableHTTPS          bool
	TLSCertFile          string // путь к TLS-сертификату
	TLSKeyFile           string // путь к TLS-ключу
	storage              repository.Store
	DeleteWorkerCount    int           `env:"DELETE_WORKER_COUNT" envDefault:"5"`
	DeleteQueueSize      int           `env:"DELETE_QUEUE_SIZE" envDefault:"1024"`
	DeleteBufferSize     int           `env:"DELETE_BUFFER_SIZE" envDefault:"100"`
	DeleteFlushInterval  time.Duration `env:"DELETE_FLUSH_INTERVAL" envDefault:"1s"`
	DeleteEnqueueTimeout time.Duration `env:"DELETE_ENQUEUE_TIMEOUT" envDefault:"100ms"`
}

// New читает флаги, переменные окружения и JSON-файл конфигурации,
// валидирует результат и поднимает хранилище по приоритету: PostgreSQL → файл → память.
// Приоритет источников: флаги > env > JSON-файл > дефолты.
// Возвращает ошибку при некорректных параметрах или проблемах с файлом конфигурации;
// сбой подключения к хранилищу не фатален — используется хранилище в памяти.
func New() (*Config, error) {
	serverAddress := flag.String("a", "", "адрес запуска HTTP-сервера")
	baseURL := flag.String("b", "", "базовый адрес результирующего сокращённого URL")
	fileStoragePath := flag.String("f", "", "путь до файла для хранения данных")
	databaseDSN := flag.String("d", "", "DSN для подключения к PostgreSQL")
	auditFile := flag.String("audit-file", "", "путь к файлу аудита")
	auditURL := flag.String("audit-url", "", "URL сервера аудита")
	enableHTTPS := flag.Bool("s", false, "включить HTTPS")
	tlsCert := flag.String("tls-cert", "", "путь к TLS-сертификату")
	tlsKey := flag.String("tls-key", "", "путь к TLS-ключу")
	configPath := flag.String("c", "", "путь к JSON-файлу конфигурации")
	configPathLong := flag.String("config", "", "путь к JSON-файлу конфигурации (длинная форма)")

	flag.Parse()

	// Определяем путь к конфигу: короткий флаг -c, длинный --config или env CONFIG.
	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = *configPathLong
	}
	if cfgPath == "" {
		cfgPath = os.Getenv("CONFIG")
	}

	// Читаем файл конфигурации (если путь задан). Ошибки — фатальные.
	var fileCfg FileConfig
	if cfgPath != "" {
		loaded, err := loadFileConfig(cfgPath)
		if err != nil {
			return nil, err
		}
		fileCfg = *loaded
	}

	config := &Config{
		// Приоритет: флаг > env > JSON-файл > дефолт.
		ServerAddress:   resolveString(*serverAddress, "SERVER_ADDRESS", fileCfg.ServerAddress, "localhost:8080"),
		BaseURL:         resolveString(*baseURL, "BASE_URL", fileCfg.BaseURL, "http://localhost:8080"),
		FileStoragePath: resolveString(*fileStoragePath, "FILE_STORAGE_PATH", fileCfg.FileStoragePath, defaultFileStoragePath),
		DatabaseDSN:     resolveString(*databaseDSN, "DATABASE_DSN", fileCfg.DatabaseDSN, ""),
		AuditFile:       resolveString(*auditFile, "AUDIT_FILE", "", ""),
		AuditURL:        resolveString(*auditURL, "AUDIT_URL", "", ""),
		EnableHTTPS:     resolveBool(*enableHTTPS, "ENABLE_HTTPS", fileCfg.EnableHTTPS, false),

		// Пути к сертификатам: флаг → env → дефолт.
		TLSCertFile: getTLSFilePath(*tlsCert, "TLS_CERT_FILE", defaultTLSCertFile),
		TLSKeyFile:  getTLSFilePath(*tlsKey, "TLS_KEY_FILE", defaultTLSKeyFile),

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
			_ = dbStore.Close()
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

// resolveString возвращает значение с приоритетом: флаг > env > файл > дефолт.
func resolveString(flagValue, envKey, fileValue, defaultValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envValue := os.Getenv(envKey); envValue != "" {
		return envValue
	}
	if fileValue != "" {
		return fileValue
	}
	return defaultValue
}

// resolveBool возвращает bool с приоритетом: флаг > env > файл > дефолт.
// fileValue — указатель, чтобы отличить false от отсутствия поля в файле.
func resolveBool(flagValue bool, envKey string, fileValue *bool, defaultValue bool) bool {
	if flagValue {
		return true
	}
	if envValue := os.Getenv(envKey); envValue != "" {
		return parseBoolEnv(envKey)
	}
	if fileValue != nil {
		return *fileValue
	}
	return defaultValue
}

// parseBoolEnv читает переменную окружения как bool.
// Возвращает true для "true", "1", "yes" (регистр не важен).
func parseBoolEnv(envVar string) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(envVar)))
	switch raw {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

// getTLSFilePath разрешает путь к TLS-файлу по приоритету:
// флаг → переменная окружения → значение по умолчанию.
func getTLSFilePath(flagValue, envVar, defaultValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envValue := os.Getenv(envVar); envValue != "" {
		return envValue
	}
	return defaultValue
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

// NewTest возвращает конфигурацию с фиксированными значениями и хранилищем
// в памяти, пригодную для тестов без флагов и окружения.
func NewTest() *Config {
	return &Config{
		ServerAddress:   "localhost:8080",
		BaseURL:         "http://localhost:8080",
		FileStoragePath: "test_storage.json",
		DatabaseDSN:     "",
		storage:         repository.NewFileStorage(),
	}
}

// SetStorage подменяет хранилище, например для инъекции фейка в тестах.
func (c *Config) SetStorage(s repository.Store) {
	c.storage = s
}

// GetStorage возвращает активное хранилище, при необходимости создавая
// хранилище в памяти.
func (c *Config) GetStorage() repository.Store {
	if c.storage == nil {
		c.storage = repository.NewFileStorage()
	}
	return c.storage
}

// Validate проверяет обязательные поля: адрес сервера и базовый URL.
// Возвращает ошибку с указанием пустого поля.
func (c *Config) Validate() error {
	if c.ServerAddress == "" {
		return fmt.Errorf("адрес сервера не может быть пустым")
	}

	if c.BaseURL == "" {
		return fmt.Errorf("базовый URL не может быть пустым")
	}

	return nil
}

// GetServerAddress возвращает адрес, на котором поднимается HTTP-сервер.
func (c *Config) GetServerAddress() string {
	return c.ServerAddress
}

// GetBaseURL возвращает префикс, с которым формируются короткие ссылки.
func (c *Config) GetBaseURL() string {
	return c.BaseURL
}

// GetFileStoragePath возвращает путь к файлу файлового хранилища.
func (c *Config) GetFileStoragePath() string {
	return c.FileStoragePath
}

// GetDatabaseDSN возвращает строку подключения к PostgreSQL; пустая — если БД не задана.
func (c *Config) GetDatabaseDSN() string {
	return c.DatabaseDSN
}

// GetEnableHTTPS возвращает признак запуска сервера в режиме HTTPS.
func (c *Config) GetEnableHTTPS() bool {
	return c.EnableHTTPS
}

// GetTLSCertFile возвращает путь к TLS-сертификату.
func (c *Config) GetTLSCertFile() string {
	return c.TLSCertFile
}

// GetTLSKeyFile возвращает путь к TLS-ключу.
func (c *Config) GetTLSKeyFile() string {
	return c.TLSKeyFile
}
