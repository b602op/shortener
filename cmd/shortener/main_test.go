package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.T) {
	t.Skip("Integration tests not implemented")
}

// TestGetSecretKey_FromEnv проверяет приоритет переменной окружения.
func TestGetSecretKey_FromEnv(t *testing.T) {
	t.Setenv("AUTH_SECRET_KEY", "env-secret")

	key, err := getSecretKey()
	require.NoError(t, err)
	assert.Equal(t, "env-secret", key)
}

// TestGetSecretKey_Generated проверяет генерацию случайного ключа.
func TestGetSecretKey_Generated(t *testing.T) {
	t.Setenv("AUTH_SECRET_KEY", "")

	key, err := getSecretKey()
	require.NoError(t, err)
	assert.Len(t, key, 64, "32 байта в hex-кодировке — 64 символа")

	// Повторная генерация даёт другой ключ
	key2, err := getSecretKey()
	require.NoError(t, err)
	assert.NotEqual(t, key, key2, "ключи должны быть случайными")
}

// TestFormatBuildInfo_AllSet проверяет вывод при полностью заданной сборке.
func TestFormatBuildInfo_AllSet(t *testing.T) {
	got := formatBuildInfo("1.0.0", "2025-01-15T10:30:00Z", "abc1234")
	want := "Build version: 1.0.0\nBuild date: 2025-01-15T10:30:00Z\nBuild commit: abc1234"
	assert.Equal(t, want, got)
}

// TestFormatBuildInfo_AllEmpty проверяет подстановку N/A для пустых значений.
func TestFormatBuildInfo_AllEmpty(t *testing.T) {
	got := formatBuildInfo("", "", "")
	want := "Build version: N/A\nBuild date: N/A\nBuild commit: N/A"
	assert.Equal(t, want, got)
}

// TestFormatBuildInfo_PartialSet проверяет частично заданную сборку.
func TestFormatBuildInfo_PartialSet(t *testing.T) {
	got := formatBuildInfo("1.0.0", "", "")
	want := "Build version: 1.0.0\nBuild date: N/A\nBuild commit: N/A"
	assert.Equal(t, want, got)
}

// TestOrNA проверяет подстановку N/A для пустой строки.
func TestOrNA(t *testing.T) {
	assert.Equal(t, "N/A", orNA(""))
	assert.Equal(t, "value", orNA("value"))
}
