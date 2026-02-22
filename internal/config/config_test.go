package config

import (
	"testing"
)

func TestNewTest(t *testing.T) {
	cfg := NewTest()
	
	if cfg.GetServerAddress() != "localhost:8080" {
		t.Errorf("Expected server address 'localhost:8080', got '%s'", cfg.GetServerAddress())
	}
	
	if cfg.GetBaseURL() != "http://localhost:8080" {
		t.Errorf("Expected base URL 'http://localhost:8080', got '%s'", cfg.GetBaseURL())
	}
}

func TestConfigValidation(t *testing.T) {
	// Тест валидации пустого адреса сервера
	cfg := &Config{ServerAddress: "", BaseURL: "http://example.com"}
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for empty server address")
	}
	
	// Тест валидации пустого базового URL
	cfg = &Config{ServerAddress: "localhost:8080", BaseURL: ""}
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for empty base URL")
	}
	
	// Тест валидной конфигурации
	cfg = &Config{ServerAddress: "localhost:8080", BaseURL: "http://example.com"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected no validation error, got: %v", err)
	}
}