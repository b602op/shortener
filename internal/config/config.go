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
// Флаги имеют абсолютный приоритет: явно заданный -s=false перекрывает ENABLE_HTTPS=true.
// Возвращает ошибку при некорректных параметрах или проблемах с файлом конфигурации;
// сбой подключения к хранилищу не фатален — используется хранилище в памяти.
func New() (*Config, error) {
	// 1. Регистрируем флаги и парсим аргументы командной строки (без применения).
	values, wasSet := parseFlags(flag.CommandLine, os.Args[1:])

	// 2. Определяем путь к конфигу: короткий флаг -c, длинный --config или env CONFIG.
	cfgPath := values.configPath
	if cfgPath == "" {
		cfgPath = values.configPathLong
	}
	if cfgPath == "" {
		cfgPath = os.Getenv("CONFIG")
	}

	// 3. Читаем файл конфигурации (если путь задан). Ошибки — фатальные.
	var fileCfg FileConfig
	if cfgPath != "" {
		loaded, err := loadFileConfig(cfgPath)
		if err != nil {
			return nil, err
		}
		fileCfg = *loaded
	}

	// 4. Собираем конфиг в порядке приоритета: дефолт → файл → env → флаги.
	cfg := &Config{
		// 1. Дефолты (самый низкий приоритет).
		ServerAddress:        "localhost:8080",
		BaseURL:              "http://localhost:8080",
		FileStoragePath:      defaultFileStoragePath,
		DatabaseDSN:          "",
		AuditFile:            "",
		AuditURL:             "",
		EnableHTTPS:          false,
		TLSCertFile:          defaultTLSCertFile,
		TLSKeyFile:           defaultTLSKeyFile,
		DeleteWorkerCount:    5,
		DeleteQueueSize:      1024,
		DeleteBufferSize:     100,
		DeleteFlushInterval:  time.Second,
		DeleteEnqueueTimeout: 100 * time.Millisecond,
	}

	// 2. Файл (перекрывает дефолты).
	applyFileConfig(cfg, &fileCfg)

	// 3. Env (перекрывает файл).
	applyEnvConfig(cfg)

	// 4. Флаги (перекрывают всё) — применяются только явно заданные.
	applyFlagConfig(cfg, wasSet, values)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Выбираем хранилище: PostgreSQL → файл → память
	store, err := cfg.createStore()
	if err != nil {
		slog.Warn("Ошибка создания хранилища, используется память", "error", err)
		cfg.storage = repository.NewFileStorage()
	} else {
		cfg.storage = store
	}

	return cfg, nil
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

// flagValues — значения флагов для применения.
type flagValues struct {
	serverAddress   string
	baseURL         string
	fileStoragePath string
	databaseDSN     string
	auditFile       string
	auditURL        string
	enableHTTPS     bool
	tlsCert         string
	tlsKey          string
	configPath      string
	configPathLong  string
}

// parseFlags регистрирует флаги в fs, парсит args и возвращает значения флагов
// вместе с множеством имён, заданных явно. flag.Visit позволяет отличить
// "флаг не задан" от "флаг задан явно в false" (важно для -s=false).
func parseFlags(fs *flag.FlagSet, args []string) (flagValues, map[string]bool) {
	serverAddress := fs.String("a", "", "адрес запуска HTTP-сервера")
	baseURL := fs.String("b", "", "базовый адрес результирующего сокращённого URL")
	fileStoragePath := fs.String("f", "", "путь до файла для хранения данных")
	databaseDSN := fs.String("d", "", "DSN для подключения к PostgreSQL")
	auditFile := fs.String("audit-file", "", "путь к файлу аудита")
	auditURL := fs.String("audit-url", "", "URL сервера аудита")
	enableHTTPS := fs.Bool("s", false, "включить HTTPS")
	tlsCert := fs.String("tls-cert", "", "путь к TLS-сертификату")
	tlsKey := fs.String("tls-key", "", "путь к TLS-ключу")
	configPath := fs.String("c", "", "путь к JSON-файлу конфигурации")
	configPathLong := fs.String("config", "", "путь к JSON-файлу конфигурации (длинная форма)")

	// Для flag.CommandLine (ExitOnError) некорректные аргументы завершают процесс,
	// для тестовых FlagSet (ContinueOnError) ошибка парсинга просто игнорируется.
	_ = fs.Parse(args)

	values := flagValues{
		serverAddress:   *serverAddress,
		baseURL:         *baseURL,
		fileStoragePath: *fileStoragePath,
		databaseDSN:     *databaseDSN,
		auditFile:       *auditFile,
		auditURL:        *auditURL,
		enableHTTPS:     *enableHTTPS,
		tlsCert:         *tlsCert,
		tlsKey:          *tlsKey,
		configPath:      *configPath,
		configPathLong:  *configPathLong,
	}

	return values, flagWasSet(fs)
}

// flagWasSet возвращает множество имён флагов, которые пользователь задал явно.
func flagWasSet(fs *flag.FlagSet) map[string]bool {
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	return set
}

// applyFlagConfig применяет значения флагов, которые были заданы явно.
// Флаги имеют абсолютный приоритет — даже -s=false перекрывает env и файл.
func applyFlagConfig(cfg *Config, wasSet map[string]bool, values flagValues) {
	if wasSet["a"] {
		cfg.ServerAddress = values.serverAddress
	}
	if wasSet["b"] {
		cfg.BaseURL = values.baseURL
	}
	if wasSet["f"] {
		cfg.FileStoragePath = values.fileStoragePath
	}
	if wasSet["d"] {
		cfg.DatabaseDSN = values.databaseDSN
	}
	if wasSet["audit-file"] {
		cfg.AuditFile = values.auditFile
	}
	if wasSet["audit-url"] {
		cfg.AuditURL = values.auditURL
	}
	if wasSet["s"] {
		cfg.EnableHTTPS = values.enableHTTPS // ← даже false!
	}
	if wasSet["tls-cert"] {
		cfg.TLSCertFile = values.tlsCert
	}
	if wasSet["tls-key"] {
		cfg.TLSKeyFile = values.tlsKey
	}
}

// applyFileConfig применяет поля из JSON-файла (перекрывают дефолты).
func applyFileConfig(cfg *Config, fc *FileConfig) {
	if fc.ServerAddress != "" {
		cfg.ServerAddress = fc.ServerAddress
	}
	if fc.BaseURL != "" {
		cfg.BaseURL = fc.BaseURL
	}
	if fc.FileStoragePath != "" {
		cfg.FileStoragePath = fc.FileStoragePath
	}
	if fc.DatabaseDSN != "" {
		cfg.DatabaseDSN = fc.DatabaseDSN
	}
	if fc.AuditFile != "" {
		cfg.AuditFile = fc.AuditFile
	}
	if fc.AuditURL != "" {
		cfg.AuditURL = fc.AuditURL
	}
	if fc.EnableHTTPS != nil {
		cfg.EnableHTTPS = *fc.EnableHTTPS
	}
	if fc.TLSCertFile != "" {
		cfg.TLSCertFile = fc.TLSCertFile
	}
	if fc.TLSKeyFile != "" {
		cfg.TLSKeyFile = fc.TLSKeyFile
	}
	if fc.DeleteWorkerCount != nil {
		cfg.DeleteWorkerCount = *fc.DeleteWorkerCount
	}
	if fc.DeleteQueueSize != nil {
		cfg.DeleteQueueSize = *fc.DeleteQueueSize
	}
	if fc.DeleteBufferSize != nil {
		cfg.DeleteBufferSize = *fc.DeleteBufferSize
	}
	if fc.DeleteFlushInterval != nil {
		cfg.DeleteFlushInterval = *fc.DeleteFlushInterval
	}
	if fc.DeleteEnqueueTimeout != nil {
		cfg.DeleteEnqueueTimeout = *fc.DeleteEnqueueTimeout
	}
}

// applyEnvConfig применяет env-переменные (перекрывают файл).
func applyEnvConfig(cfg *Config) {
	if v := os.Getenv("SERVER_ADDRESS"); v != "" {
		cfg.ServerAddress = v
	}
	if v := os.Getenv("BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("FILE_STORAGE_PATH"); v != "" {
		cfg.FileStoragePath = v
	}
	if v := os.Getenv("DATABASE_DSN"); v != "" {
		cfg.DatabaseDSN = v
	}
	if v := os.Getenv("AUDIT_FILE"); v != "" {
		cfg.AuditFile = v
	}
	if v := os.Getenv("AUDIT_URL"); v != "" {
		cfg.AuditURL = v
	}
	if v := os.Getenv("ENABLE_HTTPS"); v != "" {
		cfg.EnableHTTPS = parseBoolEnv("ENABLE_HTTPS")
	}
	if v := os.Getenv("TLS_CERT_FILE"); v != "" {
		cfg.TLSCertFile = v
	}
	if v := os.Getenv("TLS_KEY_FILE"); v != "" {
		cfg.TLSKeyFile = v
	}
	// Параметры воркера удаления.
	cfg.DeleteWorkerCount = getEnvInt("DELETE_WORKER_COUNT", cfg.DeleteWorkerCount)
	cfg.DeleteQueueSize = getEnvInt("DELETE_QUEUE_SIZE", cfg.DeleteQueueSize)
	cfg.DeleteBufferSize = getEnvInt("DELETE_BUFFER_SIZE", cfg.DeleteBufferSize)
	cfg.DeleteFlushInterval = getEnvDuration("DELETE_FLUSH_INTERVAL", cfg.DeleteFlushInterval)
	cfg.DeleteEnqueueTimeout = getEnvDuration("DELETE_ENQUEUE_TIMEOUT", cfg.DeleteEnqueueTimeout)
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
