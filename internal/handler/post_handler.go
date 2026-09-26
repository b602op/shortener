package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/b602op/shortener/internal/audit"
	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/domain"
	"github.com/b602op/shortener/internal/service"
)

// MethodPost возвращает обработчик POST / для тела в формате text/plain.
// Тело запроса считается исходным URL; валидация и сохранение — в общем сервисе.
// Ответ: 201 с полным коротким URL, 409 с уже существующим адресом при
// дубликате, 400 при пустом теле или невалидном URL, 500 при ошибке сохранения.
func MethodPost(svc *service.ShortenerService, auditService AuditNotifier) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		slog.Info("Получен POST запрос", "uri", req.RequestURI)

		if req.Method != http.MethodPost {
			respondWithError(res, "Метод не разрешен", http.StatusMethodNotAllowed)
			return
		}

		defer func() { _ = req.Body.Close() }()

		body, err := io.ReadAll(req.Body)
		if err != nil {
			respondWithError(res, "Ошибка чтения тела", http.StatusBadRequest)
			return
		}

		slog.Debug("Тело запроса", "body", string(body))

		if len(body) == 0 {
			respondWithError(res, "Пустое тело запроса", http.StatusBadRequest)
			return
		}

		originalURL := string(body)

		// Извлекаем userID из контекста (устанавливается AuthMiddleware)
		userID, _ := auth.GetUserIDFromContext(req.Context())

		// Валидация и сохранение — в общем сервисе
		shortURL, err := svc.ShortenURL(req.Context(), userID, originalURL)
		if err != nil {
			if errors.Is(err, domain.ErrDuplicateURL) {
				slog.Warn("Дубликат URL", "url", originalURL)
				res.Header().Set("Content-Type", "text/plain")
				res.WriteHeader(http.StatusConflict)
				_, _ = res.Write([]byte(shortURL))
				return
			}
			if errors.Is(err, domain.ErrInvalidURL) {
				respondWithError(res, "Невалидный URL", http.StatusBadRequest)
				return
			}
			slog.Error("Ошибка сохранения", "error", err)
			respondWithError(res, "Failed to save URL", http.StatusInternalServerError)
			return
		}

		notifyAudit(req, auditService, audit.ActionShorten, originalURL)

		slog.Info("Сокращённый URL создан", "shortURL", shortURL)

		// Отправляем ответ
		res.Header().Set("Content-Type", "text/plain")
		res.WriteHeader(http.StatusCreated)
		_, _ = res.Write([]byte(shortURL))
	}
}
