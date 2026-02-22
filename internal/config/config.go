package config

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	ServerAddress string
	BaseURL       string
}

func New() (*Config, error) {
	serverAddress := flag.String("a", "localhost:8080", "адрес запуска HTTP-сервера")
	baseURL := flag.String("b", "http://localhost:8080", "базовый адрес результирующего сокращённого URL")

	flag.Parse()

	// Приоритет: переменные окружения → флаги → значения по умолчанию
	serverAddressValue := getEnvOrFlag("SERVER_ADDRESS", *serverAddress)
	baseURLValue := getEnvOrFlag("BASE_URL", *baseURL)

	config := &Config{
		ServerAddress: serverAddressValue,
		BaseURL:       baseURLValue,
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// getEnvOrFlag возвращает значение переменной окружения, если она установлена,
// иначе возвращает значение флага
func getEnvOrFlag(envVar, flagValue string) string {
	if envValue := os.Getenv(envVar); envValue != "" {
		return envValue
	}
	return flagValue
}

func NewTest() *Config {
	return &Config{
		ServerAddress: "localhost:8080",
		BaseURL:       "http://localhost:8080",
	}
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
