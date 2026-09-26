package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/b602op/shortener/internal/audit"
	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAudit_ShortenViaRoot(t *testing.T) {
	storage := repository.NewFileStorage()
	cfg := config.NewTest()
	mock := newMockAudit()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("http://example.com/audit-root"))
	rec := httptest.NewRecorder()

	MethodPost(newTestService(cfg, storage), mock)(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	require.Eventually(t, func() bool {
		return len(mock.Events()) == 1
	}, time.Second, 10*time.Millisecond)

	events := mock.Events()
	assert.Equal(t, audit.ActionShorten, events[0].Action)
	assert.Equal(t, "http://example.com/audit-root", events[0].URL)
}

func TestAudit_ShortenViaAPI(t *testing.T) {
	storage := repository.NewFileStorage()
	cfg := config.NewTest()
	mock := newMockAudit()

	body := `{"url":"http://example.com/audit-api"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	rec := httptest.NewRecorder()

	MethodPostAPI(newTestService(cfg, storage), mock)(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	require.Eventually(t, func() bool {
		return len(mock.Events()) == 1
	}, time.Second, 10*time.Millisecond)

	events := mock.Events()
	assert.Equal(t, audit.ActionShorten, events[0].Action)
	assert.Equal(t, "http://example.com/audit-api", events[0].URL)
}

func TestAudit_Follow(t *testing.T) {
	storage := repository.NewFileStorage()
	cfg := config.NewTest()
	mock := newMockAudit()

	// Создаём запись
	require.NoError(t, storage.Insert("", "http://example.com/audit-follow", "testshort"))

	req := httptest.NewRequest(http.MethodGet, "/testshort", nil)
	rec := httptest.NewRecorder()

	MethodGet(newTestService(cfg, storage), mock)(rec, req)
	require.Equal(t, http.StatusTemporaryRedirect, rec.Code)

	require.Eventually(t, func() bool {
		return len(mock.Events()) == 1
	}, time.Second, 10*time.Millisecond)

	events := mock.Events()
	assert.Equal(t, audit.ActionFollow, events[0].Action)
	assert.Equal(t, "http://example.com/audit-follow", events[0].URL)
}

func TestAudit_HTTPObserverIntegration(t *testing.T) {
	received := make(chan audit.Event, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var e audit.Event
		require.NoError(t, json.NewDecoder(r.Body).Decode(&e))
		received <- e
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	auditService := audit.NewService()
	auditService.Subscribe(audit.NewHTTPObserver(srv.URL))

	store := repository.NewFileStorage()
	cfg := config.NewTest()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("http://example.com/e2e"))
	rec := httptest.NewRecorder()

	MethodPost(newTestService(cfg, store), auditService)(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	select {
	case e := <-received:
		assert.Equal(t, audit.ActionShorten, e.Action)
		assert.Equal(t, "http://example.com/e2e", e.URL)
	case <-time.After(time.Second):
		t.Fatal("событие не дошло до HTTP-приёмника")
	}
}
