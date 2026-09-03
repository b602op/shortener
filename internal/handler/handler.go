package handler

import (
	"net/http"

	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
	"github.com/b602op/shortener/internal/worker"
	"github.com/go-chi/chi/v5"
)

func Handler(cfg *config.Config, store repository.Store, authService *auth.Service, deleteService *worker.DeleteService) http.Handler {
	r := chi.NewRouter()

	r.Use(GzipMiddleware)
	r.Use(AuthMiddleware(authService))

	// Передаем store в обработчики
	r.Post("/", MethodPost(cfg, store))
	r.Post("/api/shorten", MethodPostAPI(cfg, store))
	r.Post("/api/shorten/batch", MethodPostBatchAPI(cfg, store))
	r.Get("/api/user/urls", MethodGetUserURLs(cfg, store, authService))
	r.Delete("/api/user/urls", MethodDeleteUserURLs(deleteService, authService))
	r.Get("/{id}", MethodGet(cfg, store))

	// Добавляем /ping если используется PostgreSQL
	if dbStore, ok := store.(*repository.DBStorage); ok && dbStore.DB() != nil {
		r.Get("/ping", PingHandler(dbStore.DB()))
	}

	return r
}
