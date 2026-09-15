package audit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPObserver_Notify(t *testing.T) {
	var gotMethod string
	var gotContentType string
	var gotEvent Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		if err := json.Unmarshal(body, &gotEvent); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	obs := NewHTTPObserver(srv.URL)
	event := Event{TS: 42, Action: ActionFollow, UserID: "u1", URL: "http://example.com"}

	require.NoError(t, obs.Notify(context.Background(), event))

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "application/json", gotContentType)
	assert.Equal(t, event, gotEvent)
}

func TestHTTPObserver_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	obs := NewHTTPObserver(srv.URL)
	err := obs.Notify(context.Background(), Event{Action: ActionShorten, URL: "http://example.com"})
	assert.Error(t, err)
}

func TestHTTPObserver_Unreachable(t *testing.T) {
	obs := NewHTTPObserver("http://127.0.0.1:1")
	err := obs.Notify(context.Background(), Event{Action: ActionShorten, URL: "http://example.com"})
	assert.Error(t, err)
}
