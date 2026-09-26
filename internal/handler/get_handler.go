package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/b602op/shortener/internal/audit"
	"github.com/b602op/shortener/internal/domain"
	"github.com/b602op/shortener/internal/service"
)

// MethodGet возвращает обработчик GET /{id} — редирект по короткой ссылке.
// Выборка — в общем сервисе.
// Ответ: 307 с заголовком Location, 404 если короткий адрес неизвестен,
// 410 если запись помечена удалённой, 400 при пустом идентификаторе.
func MethodGet(svc *service.ShortenerService, auditService AuditNotifier) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		slog.Info("Получен GET запрос", "uri", req.RequestURI)

		if req.Method != http.MethodGet {
			respondWithError(res, "Short URL is required", http.StatusBadRequest)
			return
		}

		path := strings.Trim(req.URL.Path, "/")
		if path == "" {
			respondWithError(res, "Short URL is required", http.StatusBadRequest)
			return
		}

		parts := strings.Split(path, "/")
		shortURL := parts[0]

		slog.Debug("shortURL", "shortURL", shortURL)

		originalURL, err := svc.ExpandURL(req.Context(), shortURL)
		if err != nil {
			if errors.Is(err, domain.ErrDeleted) {
				// URL помечен удалённым его создателем
				res.WriteHeader(http.StatusGone)
				return
			}
			if errors.Is(err, domain.ErrNotFound) {
				respondWithError(res, "Short URL not found", http.StatusNotFound)
				return
			}
			slog.Error("Ошибка выбора записи", "error", err)
			respondWithError(res, "Internal server error", http.StatusInternalServerError)
			return
		}

		notifyAudit(req, auditService, audit.ActionFollow, originalURL)

		res.Header().Set("Location", originalURL)
		res.Header().Set("Content-Type", "text/plain")
		res.WriteHeader(http.StatusTemporaryRedirect)
	}
}
