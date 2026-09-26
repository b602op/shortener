package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// FileConfig описывает конфигурацию из JSON-файла.
// Все поля опциональны — незаданные берутся из env/флагов/дефолтов.
// Поля-указатели позволяют отличить явно заданное нулевое значение
// (false, 0) от отсутствия поля в файле.
type FileConfig struct {
	// Основные
	ServerAddress   string `json:"server_address"`
	BaseURL         string `json:"base_url"`
	FileStoragePath string `json:"file_storage_path"`
	DatabaseDSN     string `json:"database_dsn"`

	// HTTPS
	EnableHTTPS *bool  `json:"enable_https"` // указатель, чтобы отличить false от отсутствия
	TLSCertFile string `json:"tls_cert_file"`
	TLSKeyFile  string `json:"tls_key_file"`

	// Аудит
	AuditFile string `json:"audit_file"`
	AuditURL  string `json:"audit_url"`

	// Воркер удаления.
	// Поля *time.Duration задаются числом наносекунд (см. README).
	DeleteWorkerCount    *int           `json:"delete_worker_count"`
	DeleteQueueSize      *int           `json:"delete_queue_size"`
	DeleteBufferSize     *int           `json:"delete_buffer_size"`
	DeleteFlushInterval  *time.Duration `json:"delete_flush_interval"`
	DeleteEnqueueTimeout *time.Duration `json:"delete_enqueue_timeout"`

	// Доверенная подсеть для /api/internal/stats (CIDR, например "192.168.1.0/24").
	TrustedSubnet string `json:"trusted_subnet"`
}

// loadFileConfig читает и парсит JSON-файл конфигурации.
//
// Возвращает ошибку:
//   - если файл не существует;
//   - если JSON невалиден;
//   - если файл нечитаем.
func loadFileConfig(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл конфигурации %q: %w", path, err)
	}

	var cfg FileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("невалидный JSON в файле %q: %w", path, err)
	}

	return &cfg, nil
}
