package config

import (
	"flag"
	"net"
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

func TestApplyFlagConfig_OnlyExplicitlySet(t *testing.T) {
	t.Run("все строковые флаги заданы явно", func(t *testing.T) {
		cfg := &Config{}
		applyFlagConfig(cfg, map[string]bool{
			"a": true, "b": true, "f": true, "d": true,
			"audit-file": true, "audit-url": true,
			"tls-cert": true, "tls-key": true,
		}, flagValues{
			serverAddress:   ":9090",
			baseURL:         "http://localhost:9090",
			fileStoragePath: "/tmp/storage.json",
			databaseDSN:     "postgres://localhost/db",
			auditFile:       "/tmp/audit.log",
			auditURL:        "http://audit.local",
			tlsCert:         "/tmp/cert.pem",
			tlsKey:          "/tmp/key.pem",
		})

		assert.Equal(t, ":9090", cfg.ServerAddress)
		assert.Equal(t, "http://localhost:9090", cfg.BaseURL)
		assert.Equal(t, "/tmp/storage.json", cfg.FileStoragePath)
		assert.Equal(t, "postgres://localhost/db", cfg.DatabaseDSN)
		assert.Equal(t, "/tmp/audit.log", cfg.AuditFile)
		assert.Equal(t, "http://audit.local", cfg.AuditURL)
		assert.Equal(t, "/tmp/cert.pem", cfg.TLSCertFile)
		assert.Equal(t, "/tmp/key.pem", cfg.TLSKeyFile)
	})

	t.Run("флаги не заданы — конфиг не меняется", func(t *testing.T) {
		cfg := &Config{ServerAddress: "env-value:8080", EnableHTTPS: true}
		applyFlagConfig(cfg, map[string]bool{}, flagValues{})
		assert.Equal(t, "env-value:8080", cfg.ServerAddress)
		assert.True(t, cfg.EnableHTTPS)
	})
}

func TestParseFlags_ExplicitlySetVsNotSet(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	values, wasSet := parseFlags(fs, []string{"-s=false", "-a", "localhost:9090"})

	// -s задан явно, хоть и в false — Visit это фиксирует
	assert.Contains(t, wasSet, "s")
	assert.False(t, values.enableHTTPS)
	// -a задан явно
	assert.Contains(t, wasSet, "a")
	assert.Equal(t, "localhost:9090", values.serverAddress)
	// -b не задан
	assert.NotContains(t, wasSet, "b")
	assert.Empty(t, values.baseURL)
}

func TestFlagsOverrideEnv_EnableHTTPSFalse(t *testing.T) {
	// Симулируем: env=true, флаг -s=false задан явно.
	t.Setenv("ENABLE_HTTPS", "true")

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	values, wasSet := parseFlags(fs, []string{"-s=false"})

	cfg := &Config{EnableHTTPS: false} // дефолт
	applyEnvConfig(cfg)                // env перекрывает дефолт → true
	assert.True(t, cfg.EnableHTTPS)

	applyFlagConfig(cfg, wasSet, values) // флаг перекрывает всё → false
	assert.False(t, cfg.EnableHTTPS)
}

func TestFlagsNotSet_EnvWins(t *testing.T) {
	// Флаг -s не задан — env ENABLE_HTTPS=true действует.
	t.Setenv("ENABLE_HTTPS", "true")

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	values, wasSet := parseFlags(fs, []string{})

	cfg := &Config{EnableHTTPS: false}
	applyEnvConfig(cfg)
	applyFlagConfig(cfg, wasSet, values)
	assert.True(t, cfg.EnableHTTPS)
}

func TestApplyFileConfig_OverridesDefaults(t *testing.T) {
	enableHTTPS := true
	workerCount := 3
	queueSize := 512
	bufferSize := 50
	flushInterval := 2 * time.Second
	enqueueTimeout := 200 * time.Millisecond

	cfg := &Config{
		ServerAddress:        "localhost:8080",
		BaseURL:              "http://localhost:8080",
		FileStoragePath:      "data/storage.json",
		TLSCertFile:          defaultTLSCertFile,
		TLSKeyFile:           defaultTLSKeyFile,
		DeleteWorkerCount:    5,
		DeleteQueueSize:      1024,
		DeleteBufferSize:     100,
		DeleteFlushInterval:  time.Second,
		DeleteEnqueueTimeout: 100 * time.Millisecond,
	}

	applyFileConfig(cfg, &FileConfig{
		ServerAddress:        "localhost:9090",
		BaseURL:              "http://localhost:9090",
		FileStoragePath:      "/tmp/storage.json",
		DatabaseDSN:          "postgres://localhost/db",
		EnableHTTPS:          &enableHTTPS,
		TLSCertFile:          "/tmp/cert.pem",
		TLSKeyFile:           "/tmp/key.pem",
		AuditFile:            "/tmp/audit.log",
		AuditURL:             "http://audit.local",
		DeleteWorkerCount:    &workerCount,
		DeleteQueueSize:      &queueSize,
		DeleteBufferSize:     &bufferSize,
		DeleteFlushInterval:  &flushInterval,
		DeleteEnqueueTimeout: &enqueueTimeout,
	})

	assert.Equal(t, "localhost:9090", cfg.ServerAddress)
	assert.Equal(t, "http://localhost:9090", cfg.BaseURL)
	assert.Equal(t, "/tmp/storage.json", cfg.FileStoragePath)
	assert.Equal(t, "postgres://localhost/db", cfg.DatabaseDSN)
	assert.True(t, cfg.EnableHTTPS)
	assert.Equal(t, "/tmp/cert.pem", cfg.TLSCertFile)
	assert.Equal(t, "/tmp/key.pem", cfg.TLSKeyFile)
	assert.Equal(t, "/tmp/audit.log", cfg.AuditFile)
	assert.Equal(t, "http://audit.local", cfg.AuditURL)
	assert.Equal(t, 3, cfg.DeleteWorkerCount)
	assert.Equal(t, 512, cfg.DeleteQueueSize)
	assert.Equal(t, 50, cfg.DeleteBufferSize)
	assert.Equal(t, 2*time.Second, cfg.DeleteFlushInterval)
	assert.Equal(t, 200*time.Millisecond, cfg.DeleteEnqueueTimeout)
}

func TestApplyFileConfig_EmptyFileKeepsDefaults(t *testing.T) {
	cfg := &Config{
		ServerAddress:       "localhost:8080",
		TLSCertFile:         defaultTLSCertFile,
		TLSKeyFile:          defaultTLSKeyFile,
		DeleteWorkerCount:   5,
		DeleteFlushInterval: time.Second,
	}

	// Пустой файл не перекрывает ничего
	applyFileConfig(cfg, &FileConfig{})

	assert.Equal(t, "localhost:8080", cfg.ServerAddress)
	assert.Equal(t, defaultTLSCertFile, cfg.TLSCertFile)
	assert.Equal(t, defaultTLSKeyFile, cfg.TLSKeyFile)
	assert.Equal(t, 5, cfg.DeleteWorkerCount)
	assert.Equal(t, time.Second, cfg.DeleteFlushInterval)
}

func TestApplyEnvConfig_OverridesFile(t *testing.T) {
	t.Setenv("SERVER_ADDRESS", "env:9090")
	t.Setenv("ENABLE_HTTPS", "true")
	t.Setenv("DELETE_WORKER_COUNT", "7")
	t.Setenv("DELETE_FLUSH_INTERVAL", "3s")

	cfg := &Config{
		ServerAddress:       "file:8080",
		EnableHTTPS:         false,
		DeleteWorkerCount:   3,
		DeleteFlushInterval: time.Second,
	}

	applyEnvConfig(cfg)

	assert.Equal(t, "env:9090", cfg.ServerAddress)
	assert.True(t, cfg.EnableHTTPS)
	assert.Equal(t, 7, cfg.DeleteWorkerCount)
	assert.Equal(t, 3*time.Second, cfg.DeleteFlushInterval)
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
        "enable_https": true,
        "tls_cert_file": "/tmp/cert.pem",
        "tls_key_file": "/tmp/key.pem",
        "audit_file": "/tmp/audit.log",
        "audit_url": "http://audit.local",
        "delete_worker_count": 3,
        "delete_queue_size": 512,
        "delete_buffer_size": 50,
        "delete_flush_interval": 2000000000,
        "delete_enqueue_timeout": 200000000
    }`), 0644))

	cfg, err := loadFileConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "localhost:9090", cfg.ServerAddress)
	assert.Equal(t, "http://localhost:9090", cfg.BaseURL)
	assert.Equal(t, "/tmp/storage.json", cfg.FileStoragePath)
	assert.Equal(t, "postgres://localhost/db", cfg.DatabaseDSN)
	require.NotNil(t, cfg.EnableHTTPS)
	assert.True(t, *cfg.EnableHTTPS)
	assert.Equal(t, "/tmp/cert.pem", cfg.TLSCertFile)
	assert.Equal(t, "/tmp/key.pem", cfg.TLSKeyFile)
	assert.Equal(t, "/tmp/audit.log", cfg.AuditFile)
	assert.Equal(t, "http://audit.local", cfg.AuditURL)

	require.NotNil(t, cfg.DeleteWorkerCount)
	assert.Equal(t, 3, *cfg.DeleteWorkerCount)
	require.NotNil(t, cfg.DeleteQueueSize)
	assert.Equal(t, 512, *cfg.DeleteQueueSize)
	require.NotNil(t, cfg.DeleteBufferSize)
	assert.Equal(t, 50, *cfg.DeleteBufferSize)
	require.NotNil(t, cfg.DeleteFlushInterval)
	assert.Equal(t, 2*time.Second, *cfg.DeleteFlushInterval)
	require.NotNil(t, cfg.DeleteEnqueueTimeout)
	assert.Equal(t, 200*time.Millisecond, *cfg.DeleteEnqueueTimeout)
}

func TestLoadFileConfig_DeleteFieldsAbsent(t *testing.T) {
	// Поля воркера удаления не заданы — указатели остаются nil
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"server_address": ":8080"}`), 0644))

	cfg, err := loadFileConfig(path)
	require.NoError(t, err)
	assert.Nil(t, cfg.DeleteWorkerCount)
	assert.Nil(t, cfg.DeleteQueueSize)
	assert.Nil(t, cfg.DeleteBufferSize)
	assert.Nil(t, cfg.DeleteFlushInterval)
	assert.Nil(t, cfg.DeleteEnqueueTimeout)
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

func TestTLSDefaultPaths(t *testing.T) {
	// Дефолтные пути к сертификатам при незаданных флагах и env
	t.Setenv("TLS_CERT_FILE", "")
	t.Setenv("TLS_KEY_FILE", "")

	cfg := &Config{
		TLSCertFile: defaultTLSCertFile,
		TLSKeyFile:  defaultTLSKeyFile,
	}

	applyEnvConfig(cfg)

	assert.Equal(t, defaultTLSCertFile, cfg.TLSCertFile)
	assert.Equal(t, defaultTLSKeyFile, cfg.TLSKeyFile)
}

func TestTLSEnvOverridesDefault(t *testing.T) {
	// Env перекрывает дефолтные пути к сертификатам
	t.Setenv("TLS_CERT_FILE", "/env/cert.pem")
	t.Setenv("TLS_KEY_FILE", "/env/key.pem")

	cfg := &Config{
		TLSCertFile: defaultTLSCertFile,
		TLSKeyFile:  defaultTLSKeyFile,
	}

	applyEnvConfig(cfg)

	assert.Equal(t, "/env/cert.pem", cfg.TLSCertFile)
	assert.Equal(t, "/env/key.pem", cfg.TLSKeyFile)
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

func TestTrustedSubnet_FileConfig(t *testing.T) {
	// Парсинг trusted_subnet из JSON
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"trusted_subnet": "192.168.1.0/24"}`), 0644))

	cfg, err := loadFileConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.0/24", cfg.TrustedSubnet)
}

func TestTrustedSubnet_ApplyFileConfig(t *testing.T) {
	// Файл перекрывает дефолт
	cfg := &Config{}
	applyFileConfig(cfg, &FileConfig{TrustedSubnet: "10.0.0.0/8"})
	assert.Equal(t, "10.0.0.0/8", cfg.TrustedSubnet)

	// Пустое поле в файле не перекрывает
	cfg = &Config{TrustedSubnet: "192.168.1.0/24"}
	applyFileConfig(cfg, &FileConfig{})
	assert.Equal(t, "192.168.1.0/24", cfg.TrustedSubnet)
}

func TestTrustedSubnet_ApplyEnvConfig(t *testing.T) {
	// Env перекрывает файл
	t.Setenv("TRUSTED_SUBNET", "172.16.0.0/12")

	cfg := &Config{TrustedSubnet: "192.168.1.0/24"}
	applyEnvConfig(cfg)
	assert.Equal(t, "172.16.0.0/12", cfg.TrustedSubnet)

	// Пустой env не перекрывает
	t.Setenv("TRUSTED_SUBNET", "")
	cfg = &Config{TrustedSubnet: "192.168.1.0/24"}
	applyEnvConfig(cfg)
	assert.Equal(t, "192.168.1.0/24", cfg.TrustedSubnet)
}

func TestTrustedSubnet_Flags(t *testing.T) {
	// Короткий флаг -t
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	values, wasSet := parseFlags(fs, []string{"-t", "192.168.1.0/24"})
	assert.Contains(t, wasSet, "t")
	assert.Equal(t, "192.168.1.0/24", values.trustedSubnet)

	cfg := &Config{}
	applyFlagConfig(cfg, wasSet, values)
	assert.Equal(t, "192.168.1.0/24", cfg.TrustedSubnet)

	// Длинный флаг --trusted-subnet
	fs = flag.NewFlagSet("test", flag.ContinueOnError)
	values, wasSet = parseFlags(fs, []string{"--trusted-subnet", "10.0.0.0/8"})
	assert.Contains(t, wasSet, "trusted-subnet")
	assert.Equal(t, "10.0.0.0/8", values.trustedSubnetLong)

	cfg = &Config{}
	applyFlagConfig(cfg, wasSet, values)
	assert.Equal(t, "10.0.0.0/8", cfg.TrustedSubnet)

	// Флаги не заданы — конфиг не меняется
	cfg = &Config{TrustedSubnet: "env-value"}
	applyFlagConfig(cfg, map[string]bool{}, flagValues{})
	assert.Equal(t, "env-value", cfg.TrustedSubnet)
}

func TestTrustedSubnet_Priority(t *testing.T) {
	// Приоритет: флаг > env > файл
	t.Setenv("TRUSTED_SUBNET", "172.16.0.0/12")

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	values, wasSet := parseFlags(fs, []string{"-t", "192.168.1.0/24"})

	cfg := &Config{TrustedSubnet: "10.0.0.0/8"} // значение из файла
	applyEnvConfig(cfg)                         // env перекрывает файл
	assert.Equal(t, "172.16.0.0/12", cfg.TrustedSubnet)

	applyFlagConfig(cfg, wasSet, values) // флаг перекрывает всё
	assert.Equal(t, "192.168.1.0/24", cfg.TrustedSubnet)
}

func TestTrustedSubnet_CIDRValidation(t *testing.T) {
	// Валидный CIDR парсится и сохраняется
	cfg := &Config{TrustedSubnet: "192.168.1.0/24"}
	_, parsed, err := net.ParseCIDR(cfg.TrustedSubnet)
	require.NoError(t, err)
	cfg.trustedSubnetNet = parsed

	require.NotNil(t, cfg.TrustedSubnetNet())
	assert.True(t, cfg.TrustedSubnetNet().Contains(net.ParseIP("192.168.1.5")))
	assert.False(t, cfg.TrustedSubnetNet().Contains(net.ParseIP("10.0.0.5")))

	// Пустая подсеть — nil
	cfg = &Config{}
	assert.Nil(t, cfg.TrustedSubnetNet())

	// Невалидный CIDR — ошибка парсинга
	_, _, err = net.ParseCIDR("not-a-cidr")
	assert.Error(t, err)
}

func TestTrustedSubnet_InvalidCIDRRejected(t *testing.T) {
	// Невалидный CIDR не проходит парсинг — сервис должен падать на старте
	for _, invalid := range []string{"not-a-cidr", "192.168.1.0", "192.168.1.0/33", "999.0.0.0/8"} {
		_, _, err := net.ParseCIDR(invalid)
		assert.Error(t, err, "CIDR %q должен быть невалидным", invalid)
	}
}
