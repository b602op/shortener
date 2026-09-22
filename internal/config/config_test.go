package config

import (
	"os"
	"testing"
	"time"

	"github.com/b602op/shortener/internal/repository"
	"github.com/stretchr/testify/assert"
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
	_ = os.Setenv("TEST_VAR", "from_env")
	defer func() { _ = os.Unsetenv("TEST_VAR") }()

	result := getEnvOrFlag("TEST_VAR", "from_flag")
	if result != "from_env" {
		t.Errorf("getEnvOrFlag with env set = %q, want 'from_env'", result)
	}

	_ = os.Unsetenv("TEST_VAR")
	result = getEnvOrFlag("TEST_VAR", "from_flag")
	if result != "from_flag" {
		t.Errorf("getEnvOrFlag without env = %q, want 'from_flag'", result)
	}
}

func TestGetFileStoragePath(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		flag     string
		want     string
	}{
		{name: "env приоритетнее флага", envValue: "/tmp/env.json", flag: "/tmp/flag.json", want: "/tmp/env.json"},
		{name: "флаг при пустом env", envValue: "", flag: "/tmp/flag.json", want: "/tmp/flag.json"},
		{name: "дефолт при пустых env и флаге", envValue: "", flag: "", want: defaultFileStoragePath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				_ = os.Setenv("FILE_STORAGE_PATH", tt.envValue)
				defer func() { _ = os.Unsetenv("FILE_STORAGE_PATH") }()
			} else {
				_ = os.Unsetenv("FILE_STORAGE_PATH")
			}

			if got := getFileStoragePath(tt.flag); got != tt.want {
				t.Errorf("getFileStoragePath(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     int
	}{
		{name: "корректное значение", envValue: "42", want: 42},
		{name: "пустое значение — дефолт", envValue: "", want: 7},
		{name: "некорректное значение — дефолт", envValue: "not-a-number", want: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				_ = os.Setenv("TEST_INT", tt.envValue)
				defer func() { _ = os.Unsetenv("TEST_INT") }()
			} else {
				_ = os.Unsetenv("TEST_INT")
			}

			if got := getEnvInt("TEST_INT", 7); got != tt.want {
				t.Errorf("getEnvInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetEnvDuration(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     time.Duration
	}{
		{name: "корректное значение", envValue: "5s", want: 5 * time.Second},
		{name: "пустое значение — дефолт", envValue: "", want: time.Minute},
		{name: "некорректное значение — дефолт", envValue: "oops", want: time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				_ = os.Setenv("TEST_DURATION", tt.envValue)
				defer func() { _ = os.Unsetenv("TEST_DURATION") }()
			} else {
				_ = os.Unsetenv("TEST_DURATION")
			}

			if got := getEnvDuration("TEST_DURATION", time.Minute); got != tt.want {
				t.Errorf("getEnvDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigStorageAccessors(t *testing.T) {
	cfg := &Config{}

	// GetStorage создаёт хранилище в памяти при первом обращении
	store := cfg.GetStorage()
	if store == nil {
		t.Error("GetStorage не должен возвращать nil")
	}

	// SetStorage подменяет хранилище
	custom := repository.NewFileStorage()
	cfg.SetStorage(custom)
	if cfg.GetStorage() != custom {
		t.Error("SetStorage должен подменять хранилище")
	}
}

func TestConfigSimpleGetters(t *testing.T) {
	cfg := &Config{
		ServerAddress:   "localhost:9090",
		BaseURL:         "http://localhost:9090",
		FileStoragePath: "/tmp/storage.json",
		DatabaseDSN:     "postgres://localhost/db",
	}

	if got := cfg.GetServerAddress(); got != "localhost:9090" {
		t.Errorf("GetServerAddress() = %q, want 'localhost:9090'", got)
	}
	if got := cfg.GetBaseURL(); got != "http://localhost:9090" {
		t.Errorf("GetBaseURL() = %q, want 'http://localhost:9090'", got)
	}
	if got := cfg.GetFileStoragePath(); got != "/tmp/storage.json" {
		t.Errorf("GetFileStoragePath() = %q, want '/tmp/storage.json'", got)
	}
	if got := cfg.GetDatabaseDSN(); got != "postgres://localhost/db" {
		t.Errorf("GetDatabaseDSN() = %q, want 'postgres://localhost/db'", got)
	}
}

func TestParseBoolEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"true", "true", true},
		{"TRUE", "TRUE", true},
		{"1", "1", true},
		{"yes", "yes", true},
		{"YES", "YES", true},
		{"false", "false", false},
		{"0", "0", false},
		{"no", "no", false},
		{"empty", "", false},
		{"garbage", "maybe", false},
		{"with spaces", "  true  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEST_BOOL_ENV", tt.value)
			got := parseBoolEnv("TEST_BOOL_ENV")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetEnableHTTPS(t *testing.T) {
	t.Run("флаг -s перекрывает env со значением false", func(t *testing.T) {
		t.Setenv("ENABLE_HTTPS", "false")
		assert.True(t, getEnableHTTPS(true), "включённый флаг -s должен перекрыть env")
	})

	t.Run("env включает при выключенном флаге", func(t *testing.T) {
		t.Setenv("ENABLE_HTTPS", "true")
		assert.True(t, getEnableHTTPS(false))
	})

	t.Run("всё выключено", func(t *testing.T) {
		t.Setenv("ENABLE_HTTPS", "false")
		assert.False(t, getEnableHTTPS(false))
	})
}

func TestGetTLSFilePath(t *testing.T) {
	tests := []struct {
		name         string
		flagValue    string
		envValue     string
		defaultValue string
		want         string
	}{
		{"флаг перекрывает env", "/tmp/flag.pem", "/tmp/env.pem", "/tmp/default.pem", "/tmp/flag.pem"},
		{"env при пустом флаге", "", "/tmp/env.pem", "/tmp/default.pem", "/tmp/env.pem"},
		{"дефолт при пустых флаге и env", "", "", "/tmp/default.pem", "/tmp/default.pem"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("TEST_TLS_FILE", tt.envValue)
			} else {
				t.Setenv("TEST_TLS_FILE", "")
			}

			got := getTLSFilePath(tt.flagValue, "TEST_TLS_FILE", tt.defaultValue)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTLSDefaultPaths(t *testing.T) {
	// Дефолтные пути к сертификатам при незаданных флагах и env
	t.Setenv("TLS_CERT_FILE", "")
	t.Setenv("TLS_KEY_FILE", "")

	assert.Equal(t, defaultTLSCertFile, getTLSFilePath("", "TLS_CERT_FILE", defaultTLSCertFile))
	assert.Equal(t, defaultTLSKeyFile, getTLSFilePath("", "TLS_KEY_FILE", defaultTLSKeyFile))
}

func TestHTTPSGetters(t *testing.T) {
	cfg := &Config{
		EnableHTTPS: true,
		TLSCertFile: "/tmp/cert.pem",
		TLSKeyFile:  "/tmp/key.pem",
	}

	assert.True(t, cfg.GetEnableHTTPS())
	assert.Equal(t, "/tmp/cert.pem", cfg.GetTLSCertFile())
	assert.Equal(t, "/tmp/key.pem", cfg.GetTLSKeyFile())
}
