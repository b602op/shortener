package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer поднимает полный роутер с middleware на файловом хранилище
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	cfg := config.NewTest()
	authService := auth.NewService("test-secret")
	store := cfg.GetStorage()
	deleteService := worker.NewDeleteService(store)
	t.Cleanup(deleteService.Close)
	router := Handler(cfg, store, authService, deleteService)

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

func TestDeleteUserURLs(t *testing.T) {
	ts := newTestServer(t)

	testURL := "http://example.com/to-be-deleted"

	// 1. Сокращаем URL и запоминаем куку пользователя
	resp, err := http.Post(ts.URL+"/", "text/plain", strings.NewReader(testURL))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	cookies := resp.Cookies()
	require.Len(t, cookies, 1)

	// 2. Извлекаем идентификатор сокращённого URL
	shortURL, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	shortID := strings.TrimPrefix(string(shortURL), "http://localhost:8080/")

	// 3. Удаляем URL асинхронно
	body := fmt.Sprintf(`["%s"]`, shortID)
	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/user/urls", strings.NewReader(body))
	require.NoError(t, err)
	req.AddCookie(cookies[0])

	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp2.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp2.StatusCode)

	// 4. Ждём асинхронного удаления и проверяем 410 Gone
	require.Eventually(t, func() bool {
		resp3, err := http.Get(ts.URL + "/" + shortID)
		if err != nil {
			return false
		}
		defer resp3.Body.Close()
		return resp3.StatusCode == http.StatusGone
	}, 3*time.Second, 50*time.Millisecond)

	// 5. Удалённый URL исчезает из списка пользователя
	req4, err := http.NewRequest(http.MethodGet, ts.URL+"/api/user/urls", nil)
	require.NoError(t, err)
	req4.AddCookie(cookies[0])

	resp4, err := http.DefaultClient.Do(req4)
	require.NoError(t, err)
	defer resp4.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp4.StatusCode)
}

func TestDeleteUserURLs_ForeignUser(t *testing.T) {
	ts := newTestServer(t)

	testURL := "http://example.com/owner-only"

	// 1. Первый пользователь сокращает URL
	resp, err := http.Post(ts.URL+"/", "text/plain", strings.NewReader(testURL))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	ownerCookies := resp.Cookies()
	require.Len(t, ownerCookies, 1)

	shortURL, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	shortID := strings.TrimPrefix(string(shortURL), "http://localhost:8080/")

	// 2. Другой пользователь пытается удалить чужой URL
	body := fmt.Sprintf(`["%s"]`, shortID)
	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/user/urls", strings.NewReader(body))
	require.NoError(t, err)

	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp2.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp2.StatusCode)

	// 3. Чужой URL не удаляется — остаётся доступным
	require.Never(t, func() bool {
		resp3, err := http.Get(ts.URL + "/" + shortID)
		if err != nil {
			return false
		}
		defer resp3.Body.Close()
		return resp3.StatusCode == http.StatusGone
	}, 500*time.Millisecond, 50*time.Millisecond)
}

func TestDeleteUserURLs_InvalidBody(t *testing.T) {
	ts := newTestServer(t)

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/user/urls", strings.NewReader("not-json"))
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestDeleteUserURLs_InvalidCookie(t *testing.T) {
	ts := newTestServer(t)

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/user/urls", strings.NewReader(`["6qxTVvsy"]`))
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "tampered.signature"})

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
