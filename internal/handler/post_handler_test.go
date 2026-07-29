package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/b602op/shortener/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMethodPost_Basic(t *testing.T) {
	testURL := "https://example.com"
	hash := sha256.Sum256([]byte(testURL))
	expectedHash := hex.EncodeToString(hash[:4])

	cfg := config.NewTest()

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(testURL))
	rec := httptest.NewRecorder()

	MethodPost(cfg)(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))

	expectedShortURL := cfg.GetBaseURL() + "/" + expectedHash
	assert.Equal(t, expectedShortURL, rec.Body.String())

	record, ok := cfg.GetStorage().Select(expectedHash)
	assert.True(t, ok)
	assert.Equal(t, testURL, record.OriginalURL)
}

func TestMethodPost_WrongMethod(t *testing.T) {
	testURL := "https://example.com"
	req := httptest.NewRequest(http.MethodGet, "/", bytes.NewBufferString(testURL))
	rec := httptest.NewRecorder()

	cfg := config.NewTest()

	MethodPost(cfg)(rec, req)

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Метод не разрешен", errResp.Error)
}

func TestMethodPost_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(""))
	rec := httptest.NewRecorder()

	cfg := config.NewTest()

	MethodPost(cfg)(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Пустое тело запроса", errResp.Error)
}

func TestMethodPost_LongURL(t *testing.T) {
	testURL := "https://example.com/" + strings.Repeat("a", 1000)
	hash := sha256.Sum256([]byte(testURL))
	expectedHash := hex.EncodeToString(hash[:4])

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(testURL))
	rec := httptest.NewRecorder()

	cfg := config.NewTest()

	MethodPost(cfg)(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	expectedShortURL := cfg.GetBaseURL() + "/" + expectedHash
	assert.Equal(t, expectedShortURL, rec.Body.String())

	record, ok := cfg.GetStorage().Select(expectedHash)
	assert.True(t, ok)
	assert.Equal(t, testURL, record.OriginalURL)
}

func TestMethodPost_ErrorReadingBody(t *testing.T) {
	failingReader := &failingReadCloser{
		Reader: bytes.NewBufferString("test"),
	}

	req := httptest.NewRequest(http.MethodPost, "/", failingReader)
	rec := httptest.NewRecorder()

	cfg := config.NewTest()

	MethodPost(cfg)(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Ошибка чтения тела", errResp.Error)
}

type failingReadCloser struct {
	io.Reader
}

func (f *failingReadCloser) Close() error {
	return nil
}

func (f *failingReadCloser) Read(p []byte) (n int, err error) {
	return 0, io.ErrClosedPipe
}
