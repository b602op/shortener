package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log"

	"github.com/b602op/shortener/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMethodGet_Basic(t *testing.T) {
	testShort := "a1b2c3d4"
	testOriginal := "https://example.com/very/long/url"
	repository.InsertData(testOriginal, testShort)

	req := httptest.NewRequest(http.MethodGet, "/"+testShort, nil)
	rec := httptest.NewRecorder()

	MethodGet(rec, req)

	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	location := rec.Header().Get("Location")

	log.Println("что тут происходит?", testOriginal, location)

	assert.Equal(t, testOriginal, location)

	assert.True(t, strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain"))

	MethodGet(rec, req)
	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)

	assert.Equal(t, testOriginal, location)
	contentType := rec.Header().Get("Content-Type")
	assert.Equal(t, "text/plain", contentType)
	assert.Empty(t, rec.Body.String())
}

func TestMethodGet_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/a1b2c3d4", nil)
	rec := httptest.NewRecorder()

	MethodGet(rec, req)

	log.Println(rec.Body.String(), rec.Code)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Short URL is required")
}

func TestMethodGet_NotFound(t *testing.T) {
	testShort := "unknown123"
	req := httptest.NewRequest(http.MethodGet, "/"+testShort, nil)
	rec := httptest.NewRecorder()

	MethodGet(rec, req)

	log.Println(rec.Body.String())

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Short URL is required")
	MethodGet(rec, req)
	expectedMsg := "Only Get requests are allowed!"
	require.Equal(t, http.StatusBadRequest, rec.Code)
	actualMsg := strings.TrimSpace(rec.Body.String())
	assert.Equal(t, expectedMsg, actualMsg)
}

func TestMethodGet_EmptyPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Short URL is required")
	MethodGet(rec, req)

	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	location := rec.Header().Get("Location")
	assert.Empty(t, location)
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
}
