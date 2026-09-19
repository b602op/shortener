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
