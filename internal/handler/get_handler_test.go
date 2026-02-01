package handler

import (
	"net/http"
	"net/http/httptest"
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
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
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
}

func TestMethodGet_EmptyPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	MethodGet(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Short URL is required")
}
