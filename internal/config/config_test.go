package config

import (
	"os"
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
	cfg := &Config{ServerAddress: "", BaseURL: "http://example.com"}
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for empty server address")
	}

	cfg = &Config{ServerAddress: "localhost:8080", BaseURL: ""}
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for empty base URL")
	}

	cfg = &Config{ServerAddress: "localhost:8080", BaseURL: "http://example.com"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected no validation error, got: %v", err)
	}
}

func TestGetEnvOrFlag(t *testing.T) {
	os.Setenv("TEST_VAR", "from_env")
	defer os.Unsetenv("TEST_VAR")

	result := getEnvOrFlag("TEST_VAR", "from_flag")
	if result != "from_env" {
		t.Errorf("getEnvOrFlag with env set = %q, want 'from_env'", result)
	}

	os.Unsetenv("TEST_VAR")
	result = getEnvOrFlag("TEST_VAR", "from_flag")
	if result != "from_flag" {
		t.Errorf("getEnvOrFlag without env = %q, want 'from_flag'", result)
	}
}
