package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// FileConfig описывает конфигурацию из JSON-файла.
// Все поля опциональны — незаданные берутся из env/флагов/дефолтов.
type FileConfig struct {
	ServerAddress   string `json:"server_address"`
	BaseURL         string `json:"base_url"`
	FileStoragePath string `json:"file_storage_path"`
	DatabaseDSN     string `json:"database_dsn"`
	EnableHTTPS     *bool  `json:"enable_https"` // указатель, чтобы отличить false от отсутствия
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
