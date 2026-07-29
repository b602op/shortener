package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/b602op/shortener/internal/repository"
)

const defaultFileStoragePath = "data/storage.json"

type Config struct {
	ServerAddress   string
	BaseURL         string
	FileStoragePath string
	storage         *repository.Storage
}

func New() (*Config, error) {
	serverAddress := flag.String("a", "localhost:8080", "адрес запуска HTTP-сервера")
	baseURL := flag.String("b", "http://localhost:8080", "базовый адрес результирующего сокращённого URL")
	fileStoragePath := flag.String("f", "", "путь до файла для хранения данных")

	flag.Parse()

	serverAddressValue := getEnvOrFlag("SERVER_ADDRESS", *serverAddress)
	baseURLValue := getEnvOrFlag("BASE_URL", *baseURL)

	fileStoragePathValue := getFileStoragePath(*fileStoragePath)

	slog.Info("Путь для сохранения данных: ", "fileStoragePath", fileStoragePathValue)

	config := &Config{
		ServerAddress:   serverAddressValue,
		BaseURL:         baseURLValue,
		FileStoragePath: fileStoragePathValue,
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
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

func NewTest() *Config {
	return &Config{
		ServerAddress:   "localhost:8080",
		BaseURL:         "http://localhost:8080",
		FileStoragePath: "test_storage.json",
		storage:         repository.NewStorage(),
	}
}

func (c *Config) SetStorage(s *repository.Storage) {
	c.storage = s
}

func (c *Config) GetStorage() *repository.Storage {
	if c.storage == nil {
		c.storage = repository.NewStorage()
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
