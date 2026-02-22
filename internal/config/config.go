package config

import (
	"flag"
	"fmt"
)

type Config struct {
	ServerAddress string
	BaseURL       string
}

func New() (*Config, error) {
	serverAddress := flag.String("a", "localhost:8080", "адрес запуска HTTP-сервера")
	baseURL := flag.String("b", "http://localhost:8080", "базовый адрес результирующего сокращённого URL")

	flag.Parse()

	config := &Config{
		ServerAddress: *serverAddress,
		BaseURL:       *baseURL,
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
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
