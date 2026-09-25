package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/b602op/shortener/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestResolveString(t *testing.T) {
	tests := []struct {
		name         string
		flagValue    string
		envKey       string
		envValue     string
		fileValue    string
		defaultValue string
		want         string
	}{
		{"флаг перекрывает env и файл", "flag", "TEST_RESOLVE_ENV", "env", "file", "default", "flag"},
		{"env перекрывает файл", "", "TEST_RESOLVE_ENV", "env", "file", "default", "env"},
		{"файл перекрывает дефолт", "", "TEST_RESOLVE_ENV", "", "file", "default", "file"},
		{"дефолт при пустых флаге, env и файле", "", "TEST_RESOLVE_ENV", "", "", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(tt.envKey, tt.envValue)
			} else {
				t.Setenv(tt.envKey, "")
			}

			got := resolveString(tt.flagValue, tt.envKey, tt.fileValue, tt.defaultValue)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveBool(t *testing.T) {
	fileTrue := true
	fileFalse := false

	t.Run("флаг перекрывает env и файл", func(t *testing.T) {
		t.Setenv("TEST_RESOLVE_BOOL", "false")
		got := resolveBool(true, "TEST_RESOLVE_BOOL", &fileFalse, false)
		assert.True(t, got)
	})

	t.Run("env перекрывает файл", func(t *testing.T) {
		t.Setenv("TEST_RESOLVE_BOOL", "true")
		got := resolveBool(false, "TEST_RESOLVE_BOOL", &fileFalse, false)
		assert.True(t, got)
	})

	t.Run("env false перекрывает файл true", func(t *testing.T) {
		t.Setenv("TEST_RESOLVE_BOOL", "false")
		got := resolveBool(false, "TEST_RESOLVE_BOOL", &fileTrue, false)
		assert.False(t, got)
	})

	t.Run("файл true при пустом env", func(t *testing.T) {
		t.Setenv("TEST_RESOLVE_BOOL", "")
		got := resolveBool(false, "TEST_RESOLVE_BOOL", &fileTrue, false)
		assert.True(t, got)
	})

	t.Run("файл false при пустом env", func(t *testing.T) {
		t.Setenv("TEST_RESOLVE_BOOL", "")
		got := resolveBool(false, "TEST_RESOLVE_BOOL", &fileFalse, true)
		assert.False(t, got)
	})

	t.Run("дефолт при пустом env и отсутствии поля в файле", func(t *testing.T) {
		t.Setenv("TEST_RESOLVE_BOOL", "")
		got := resolveBool(false, "TEST_RESOLVE_BOOL", nil, true)
		assert.True(t, got)
	})
}

func TestLoadFileConfig_NotFound(t *testing.T) {
	_, err := loadFileConfig("/nonexistent/config.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "не удалось прочитать файл конфигурации")
}

func TestLoadFileConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{invalid`), 0644))

	_, err := loadFileConfig(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "невалидный JSON")
}

func TestLoadFileConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
        "server_address": "example.com:9090",
        "enable_https": true
    }`), 0644))

	cfg, err := loadFileConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "example.com:9090", cfg.ServerAddress)
	assert.NotNil(t, cfg.EnableHTTPS)
	assert.True(t, *cfg.EnableHTTPS)
}

func TestLoadFileConfig_EnableHTTPSFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"enable_https": false}`), 0644))

	cfg, err := loadFileConfig(path)
	require.NoError(t, err)
	assert.NotNil(t, cfg.EnableHTTPS)
	assert.False(t, *cfg.EnableHTTPS)
}

func TestLoadFileConfig_EnableHTTPSAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"server_address": ":8080"}`), 0644))

	cfg, err := loadFileConfig(path)
	require.NoError(t, err)
	assert.Nil(t, cfg.EnableHTTPS)
}

func TestLoadFileConfig_UnknownFieldsIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
        "server_address": ":8080",
        "unknown_field": "value"
    }`), 0644))

	cfg, err := loadFileConfig(path)
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.ServerAddress)
}

func TestLoadFileConfig_AllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
        "server_address": "localhost:9090",
        "base_url": "http://localhost:9090",
        "file_storage_path": "/tmp/storage.json",
        "database_dsn": "postgres://localhost/db",
        "enable_https": true
    }`), 0644))

	cfg, err := loadFileConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "localhost:9090", cfg.ServerAddress)
	assert.Equal(t, "http://localhost:9090", cfg.BaseURL)
	assert.Equal(t, "/tmp/storage.json", cfg.FileStoragePath)
	assert.Equal(t, "postgres://localhost/db", cfg.DatabaseDSN)
	assert.NotNil(t, cfg.EnableHTTPS)
	assert.True(t, *cfg.EnableHTTPS)
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
