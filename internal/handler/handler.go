package handler

import (
	"net/http"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
	"github.com/go-chi/chi/v5"
)

func Handler(cfg *config.Config, storage *repository.Storage) http.Handler {
	r := chi.NewRouter()

	r.Use(GzipMiddleware)

	// ✅ Передаем storage в обработчики
	r.Post("/", MethodPost(cfg, storage))
	r.Post("/api/shorten", MethodPostAPI(cfg, storage))
	r.Get("/{id}", MethodGet(cfg, storage))

	return r
}
