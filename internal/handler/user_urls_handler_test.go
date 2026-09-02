package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer поднимает полный роутер с middleware на файловом хранилище
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	cfg := config.NewTest()
	authService := auth.NewService("test-secret")
	router := Handler(cfg, cfg.GetStorage(), authService)

	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	return ts
}

func TestGetUserURLs_FetchURLs(t *testing.T) {
	ts := newTestServer(t)

	testURL := "http://vhfn4.net/ll3z6djlaroj3/fwx55qnnjgskd"

	// 1. Сокращаем URL и запоминаем куку пользователя
	resp, err := http.Post(ts.URL+"/", "text/plain", strings.NewReader(testURL))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode)
	cookies := resp.Cookies()
	require.Len(t, cookies, 1)

	shortURL, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NotEmpty(t, string(shortURL))

	// 2. Получаем список URL пользователя с той же кукой
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/user/urls", nil)
	require.NoError(t, err)
	req.AddCookie(cookies[0])

	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()

	require.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Contains(t, resp2.Header.Get("Content-Type"), "application/json")

	var pairs []map[string]string
	err = json.NewDecoder(resp2.Body).Decode(&pairs)
	require.NoError(t, err)
	require.Len(t, pairs, 1)
	assert.Equal(t, string(shortURL), pairs[0]["short_url"])
	assert.Equal(t, testURL, pairs[0]["original_url"])
}

func TestGetUserURLs_NoURLs(t *testing.T) {
	ts := newTestServer(t)

	// Запрос без куки — middleware выдаёт нового пользователя, URL у него нет
	resp, err := http.Get(ts.URL + "/api/user/urls")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestGetUserURLs_InvalidCookie(t *testing.T) {
	ts := newTestServer(t)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/user/urls", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "tampered.signature"})

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
