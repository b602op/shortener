package handler

import (
	"net/http"

	"github.com/b602op/shortener/internal/config"
	"github.com/go-chi/chi/v5"
)

func Handler(cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(GzipMiddleware)

	r.Post("/", MethodPost(cfg))

	r.Post("/api/shorten", MethodPostAPI(cfg))

	r.Get("/{id}", MethodGet)

	return r
}
