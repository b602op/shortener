package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMethodPostAPI_Basic(t *testing.T) {
	testURL := "https://practicum.yandex.ru"
	hash := sha256.Sum256([]byte(testURL))
	expectedHash := hex.EncodeToString(hash[:4])

	cfg := config.NewTest()

	storage := repository.NewStorage()
	cfg.SetStorage(storage)

	reqBody := ShortenRequest{URL: testURL}
	bodyBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	MethodPostAPI(cfg)(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp ShortenResponse
	err = json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	expectedShortURL := cfg.GetBaseURL() + "/" + expectedHash
	assert.Equal(t, expectedShortURL, resp.Result)

	record, _ := storage.Select(expectedHash)
	assert.Equal(t, testURL, record.OriginalURL)
}

func TestMethodPostAPI_EmptyBody(t *testing.T) {
	cfg := config.NewTest()

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	MethodPostAPI(cfg)(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Неверный формат JSON", errResp.Error)
}

func TestMethodPostAPI_EmptyURL(t *testing.T) {
	cfg := config.NewTest()

	reqBody := ShortenRequest{URL: ""}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	MethodPostAPI(cfg)(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "URL не может быть пустым", errResp.Error)
}

func TestMethodPostAPI_InvalidJSON(t *testing.T) {
	cfg := config.NewTest()

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBufferString(`{invalid json}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	MethodPostAPI(cfg)(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Неверный формат JSON", errResp.Error)
}

func TestMethodPostAPI_WrongMethod(t *testing.T) {
	cfg := config.NewTest()

	req := httptest.NewRequest(http.MethodGet, "/api/shorten", nil)
	rec := httptest.NewRecorder()

	MethodPostAPI(cfg)(rec, req)

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Метод не разрешен", errResp.Error)
}
