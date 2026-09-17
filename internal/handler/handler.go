// Package handler содержит HTTP-хендлеры и middleware сервиса сокращения URL.
package handler

import (
	"net/http"

	"github.com/b602op/shortener/internal/repository"
	"github.com/go-chi/chi/v5"
)

// Handler собирает http.Handler из переданных зависимостей: подключает gzip,
// аутентификацию и зарегистрированные маршруты.
func Handler(deps Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Use(GzipMiddleware)

	// /ping — регистрируем ДО /{id}, иначе /{id} перехватит "ping"
	if dbStore, ok := deps.Store.(*repository.DBStorage); ok && dbStore.DB() != nil {
		r.Get("/ping", PingHandler(dbStore.DB()))
	}

	// Группа роутов, требующих аутентификации
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(deps.AuthService))

		r.Post("/", MethodPost(deps.Config, deps.Store, deps.AuditService))
		r.Post("/api/shorten", MethodPostAPI(deps.Config, deps.Store, deps.AuditService))
		r.Post("/api/shorten/batch", MethodPostBatchAPI(deps.Config, deps.Store))
		r.Get("/api/user/urls", MethodGetUserURLs(deps.Config, deps.Store))
		r.Delete("/api/user/urls", MethodDeleteUserURLs(deps.DeleteService))
	})

	// Редирект по короткой ссылке — без аутентификации
	r.Get("/{id}", MethodGet(deps.Config, deps.Store, deps.AuditService))

	return r
}
