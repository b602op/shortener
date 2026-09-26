// Package handler содержит HTTP-хендлеры и middleware сервиса сокращения URL.
package handler

import (
	"net/http"

	"github.com/b602op/shortener/internal/repository"
	"github.com/b602op/shortener/internal/service"
	"github.com/go-chi/chi/v5"
)

// Handler собирает http.Handler из переданных зависимостей: подключает gzip,
// аутентификацию и зарегистрированные маршруты.
func Handler(deps Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Use(GzipMiddleware)

	// Общий слой бизнес-логики; если не передан — собираем поверх Store.
	svc := deps.ShortenerService
	if svc == nil {
		svc = service.NewShortenerService(deps.Store, deps.Config.GetBaseURL())
	}

	// /ping — регистрируем ДО /{id}, иначе /{id} перехватит "ping"
	if dbStore, ok := deps.Store.(*repository.DBStorage); ok && dbStore.DB() != nil {
		r.Get("/ping", PingHandler(dbStore.DB()))
	}

	// Группа роутов, требующих аутентификации
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(deps.AuthService))

		r.Post("/", MethodPost(svc, deps.AuditService))
		r.Post("/api/shorten", MethodPostAPI(svc, deps.AuditService))
		r.Post("/api/shorten/batch", MethodPostBatchAPI(deps.Config, deps.Store))
		r.Get("/api/user/urls", MethodGetUserURLs(svc))
		r.Delete("/api/user/urls", MethodDeleteUserURLs(deps.DeleteService))
	})

	// Статистика сервиса — защищается только доверенной подсетью (X-Real-IP),
	// без аутентификации через JWT.
	if deps.Config != nil {
		statsHandler := NewStatsHandler(deps.Store, deps.Config.TrustedSubnetNet())
		r.Get("/api/internal/stats", statsHandler.GetStats)
	}

	// Редирект по короткой ссылке — без аутентификации
	r.Get("/{id}", MethodGet(svc, deps.AuditService))

	return r
}
