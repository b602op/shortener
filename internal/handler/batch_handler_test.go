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

func TestMethodPostBatchAPI_Basic(t *testing.T) {
	cfg := config.NewTest()
	storage := repository.NewFileStorage()

	batchReq := []BatchShortenRequest{
		{
			CorrelationID: "corr-1",
			OriginalURL:   "https://example.com/1",
		},
		{
			CorrelationID: "corr-2",
			OriginalURL:   "https://example.com/2",
		},
	}
	bodyBytes, err := json.Marshal(batchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	MethodPostBatchAPI(cfg, storage)(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp []BatchShortenResponse
	err = json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	require.Len(t, resp, 2)

	// Проверяем, что все URL сокращены
	seenHashes := make(map[string]bool)
	for i, item := range batchReq {
		hash := sha256.Sum256([]byte(item.OriginalURL))
		expectedHash := hex.EncodeToString(hash[:4])
		expectedShortURL := cfg.GetBaseURL() + "/" + expectedHash

		assert.Equal(t, item.CorrelationID, resp[i].CorrelationID)
		assert.Equal(t, expectedShortURL, resp[i].ShortURL)
		assert.False(t, seenHashes[expectedHash], "Дубликат short URL")
		seenHashes[expectedHash] = true

		// Проверяем, что запись сохранена
		record, ok := storage.Select(expectedHash)
		assert.True(t, ok)
		assert.Equal(t, item.OriginalURL, record.OriginalURL)
	}
}

func TestMethodPostBatchAPI_EmptyBatch(t *testing.T) {
	cfg := config.NewTest()
	storage := repository.NewFileStorage()

	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewBufferString("[]"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	MethodPostBatchAPI(cfg, storage)(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Пустой батч не допускается", errResp.Error)
}

func TestMethodPostBatchAPI_EmptyURL(t *testing.T) {
	cfg := config.NewTest()
	storage := repository.NewFileStorage()

	batchReq := []BatchShortenRequest{
		{
			CorrelationID: "corr-1",
			OriginalURL:   "",
		},
	}
	bodyBytes, err := json.Marshal(batchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	MethodPostBatchAPI(cfg, storage)(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err = json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "URL не может быть пустым", errResp.Error)
}

func TestMethodPostBatchAPI_InvalidJSON(t *testing.T) {
	cfg := config.NewTest()
	storage := repository.NewFileStorage()

	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewBufferString(`{invalid json}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	MethodPostBatchAPI(cfg, storage)(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Неверный формат JSON", errResp.Error)
}

func TestMethodPostBatchAPI_WrongMethod(t *testing.T) {
	cfg := config.NewTest()
	storage := repository.NewFileStorage()

	req := httptest.NewRequest(http.MethodGet, "/api/shorten/batch", nil)
	rec := httptest.NewRecorder()

	MethodPostBatchAPI(cfg, storage)(rec, req)

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Метод не разрешен", errResp.Error)
}

func TestMethodPostBatchAPI_DuplicateURL(t *testing.T) {
	cfg := config.NewTest()
	storage := repository.NewFileStorage()

	// Сначала создаём URL
	batchReq1 := []BatchShortenRequest{
		{OriginalURL: "https://example.com/existing"},
	}
	bodyBytes1, _ := json.Marshal(batchReq1)
	req1 := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewBuffer(bodyBytes1))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	MethodPostBatchAPI(cfg, storage)(rec1, req1)
	require.Equal(t, http.StatusCreated, rec1.Code)

	var resp1 []BatchShortenResponse
	json.NewDecoder(rec1.Body).Decode(&resp1)
	existingShortURL := resp1[0].ShortURL

	// Теперь батч: один новый + один дубликат
	batchReq2 := []BatchShortenRequest{
		{OriginalURL: "https://example.com/new"},
		{OriginalURL: "https://example.com/existing"},
	}
	bodyBytes2, _ := json.Marshal(batchReq2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewBuffer(bodyBytes2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()

	MethodPostBatchAPI(cfg, storage)(rec2, req2)

	// Должен вернуться 201, а не 409
	require.Equal(t, http.StatusCreated, rec2.Code)

	var resp2 []BatchShortenResponse
	err := json.NewDecoder(rec2.Body).Decode(&resp2)
	require.NoError(t, err)
	require.Len(t, resp2, 2)

	// Дубликат возвращает тот же short_url, что и при первом создании
	assert.Equal(t, existingShortURL, resp2[1].ShortURL)

	// Новый URL сохранён и имеет корректный формат
	hash := sha256.Sum256([]byte(batchReq2[0].OriginalURL))
	expectedHash := hex.EncodeToString(hash[:4])
	assert.Equal(t, cfg.GetBaseURL()+"/"+expectedHash, resp2[0].ShortURL)
}
