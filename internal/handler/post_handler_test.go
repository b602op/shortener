package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
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

	actualURL := repository.SelectData(expectedHash)
	assert.Equal(t, testURL, actualURL)
}

func TestMethodPost_WrongMethod(t *testing.T) {
	testURL := "https://example.com"
	req := httptest.NewRequest(http.MethodGet, "/", bytes.NewBufferString(testURL))
	rec := httptest.NewRecorder()

	cfg := config.NewTest()

	MethodPost(cfg)(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Only POST requests are allowed!")
}

func TestMethodPost_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(""))
	rec := httptest.NewRecorder()

	cfg := config.NewTest()

	MethodPost(cfg)(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Пустое тело запроса")
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

	actualURL := repository.SelectData(expectedHash)
	assert.Equal(t, testURL, actualURL)
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
	assert.Contains(t, rec.Body.String(), "Ошибка чтения тела")
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
