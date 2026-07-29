package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"log"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMethodGet_Basic(t *testing.T) {
	testShort := "a1b2c3d4"
	testOriginal := "https://example.com/very/long/url"

	storage := repository.NewStorage()
	storage.Insert(testOriginal, testShort)

	cfg := config.NewTest()
	cfg.SetStorage(storage)

	req := httptest.NewRequest(http.MethodGet, "/"+testShort, nil)
	rec := httptest.NewRecorder()

	MethodGet(cfg)(rec, req)

	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	location := rec.Header().Get("Location")

	log.Println("что тут происходит?", testOriginal, location)

	assert.Equal(t, testOriginal, location)
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Empty(t, rec.Body.String())
}

func TestMethodGet_WrongMethod(t *testing.T) {
	cfg := config.NewTest()

	req := httptest.NewRequest(http.MethodPost, "/a1b2c3d4", nil)
	rec := httptest.NewRecorder()

	MethodGet(cfg)(rec, req)

	log.Println(rec.Body.String(), rec.Code)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Short URL is required", errResp.Error)
}

func TestMethodGet_NotFound(t *testing.T) {
	cfg := config.NewTest()

	req := httptest.NewRequest(http.MethodGet, "/unknown123", nil)
	rec := httptest.NewRecorder()

	MethodGet(cfg)(rec, req)

	log.Println(rec.Body.String())

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Short URL not found", errResp.Error)
}

func TestMethodGet_EmptyPath(t *testing.T) {
	cfg := config.NewTest()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	MethodGet(cfg)(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var errResp ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	require.NoError(t, err)
	assert.Equal(t, "Short URL is required", errResp.Error)
}
