package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/b602op/shortener/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatsHandler_Access покрывает все сценарии проверки доступа
// по заголовку X-Real-IP и доверенной подсети.
func TestStatsHandler_Access(t *testing.T) {
	_, trustedNet, err := net.ParseCIDR("192.168.1.0/24")
	require.NoError(t, err)

	tests := []struct {
		name       string
		trustedNet *net.IPNet
		realIP     string
		wantStatus int
	}{
		// Блок 1: проверка доступа
		{"IP в подсети", trustedNet, "192.168.1.5", http.StatusOK},
		{"граница подсети", trustedNet, "192.168.1.255", http.StatusOK},
		{"IP не в подсети", trustedNet, "10.0.0.5", http.StatusForbidden},
		{"соседняя подсеть", trustedNet, "192.168.2.1", http.StatusForbidden},
		{"пустой конфиг", nil, "192.168.1.5", http.StatusForbidden},
		{"нет X-Real-IP", trustedNet, "", http.StatusForbidden},
		{"невалидный IP", trustedNet, "abc123", http.StatusBadRequest},
		{"невалидный IP 2", trustedNet, "999.999.999.999", http.StatusBadRequest},
		{"пустой X-Real-IP", trustedNet, " ", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := repository.NewFileStorage()
			h := NewStatsHandler(store, tt.trustedNet)

			req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
			}
			rec := httptest.NewRecorder()

			h.GetStats(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

// TestStatsHandler_Data проверяет содержимое ответа при разрешённом доступе.
func TestStatsHandler_Data(t *testing.T) {
	_, trustedNet, err := net.ParseCIDR("192.168.1.0/24")
	require.NoError(t, err)

	t.Run("счётчики URL и users", func(t *testing.T) {
		store := repository.NewFileStorage()
		// 3 URL от 2 разных пользователей
		require.NoError(t, store.Insert("user1", "https://example.com/1", "s1"))
		require.NoError(t, store.Insert("user1", "https://example.com/2", "s2"))
		require.NoError(t, store.Insert("user2", "https://example.com/3", "s3"))

		h := NewStatsHandler(store, trustedNet)

		req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
		req.Header.Set("X-Real-IP", "192.168.1.5")
		rec := httptest.NewRecorder()

		h.GetStats(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var resp StatsResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.Equal(t, 3, resp.URLs)
		assert.Equal(t, 2, resp.Users)
	})

	t.Run("пустое хранилище", func(t *testing.T) {
		store := repository.NewFileStorage()
		h := NewStatsHandler(store, trustedNet)

		req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
		req.Header.Set("X-Real-IP", "192.168.1.5")
		rec := httptest.NewRecorder()

		h.GetStats(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp StatsResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.Equal(t, 0, resp.URLs)
		assert.Equal(t, 0, resp.Users)
	})
}
