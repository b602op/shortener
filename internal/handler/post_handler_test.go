package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMethodPost_Basic(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("https://example.com"))
	rec := httptest.NewRecorder()

	MethodPost(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.NotEmpty(t, rec.Header().Get("Content-Length"))

	response := rec.Body.String()
	assert.True(t, strings.HasPrefix(response, "http://"))
	assert.Contains(t, response, req.Host)
}

func TestMethodPost_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	MethodPost(rec, req)

	expectedMsg := "Only POST requests are allowed!"

	require.Equal(t, http.StatusBadRequest, rec.Code)
	actualMsg := strings.TrimSpace(rec.Body.String())
	assert.Equal(t, expectedMsg, actualMsg)
}

func TestMethodPost_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(""))
	rec := httptest.NewRecorder()

	MethodPost(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Пустое тело запроса")
}

func TestMethodPost_LongURL(t *testing.T) {
	longURL := "https://example.com/" + strings.Repeat("a", 1000)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(longURL))
	rec := httptest.NewRecorder()

	MethodPost(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	response := rec.Body.String()
	assert.True(t, strings.HasPrefix(response, "http://"+req.Host+"/"))
	assert.Equal(t, 8, len(strings.Split(response, "/")[3]))
}

func TestMethodPost_ErrorReadingBody(t *testing.T) {
	failingReader := &failingReadCloser{
		Reader: bytes.NewBufferString("test"),
	}

	req := httptest.NewRequest(http.MethodPost, "/", failingReader)

	rec := httptest.NewRecorder()

	MethodPost(rec, req)

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
