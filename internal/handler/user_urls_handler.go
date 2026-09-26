package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/service"
)

type userURLResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// MethodGetUserURLs возвращает все URL, сокращённые пользователем.
// Выборка — в общем сервисе.
// 401 — если кука присутствует, но не содержит валидный ID пользователя.
// 204 — если пользователь ещё не сокращал URL.
// 200 — список сокращённых URL в формате JSON.
func MethodGetUserURLs(svc *service.ShortenerService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		slog.Info("Получен GET запрос к API", "uri", req.RequestURI)

		userID, ok := auth.GetUserIDFromContext(req.Context())
		if !ok || userID == "" {
			respondWithError(res, "Пользователь не авторизован", http.StatusUnauthorized)
			return
		}

		records, err := svc.ListUserURLs(req.Context(), userID)
		if err != nil {
			slog.Error("Ошибка выбора записей", "error", err)
			respondWithError(res, "Internal server error", http.StatusInternalServerError)
			return
		}

		if len(records) == 0 {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		response := make([]userURLResponse, 0, len(records))
		for _, record := range records {
			response = append(response, userURLResponse{
				ShortURL:    svc.FullShortURL(record.ShortURL),
				OriginalURL: record.OriginalURL,
			})
		}

		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(res).Encode(response); err != nil {
			slog.Error("Ошибка формирования ответа", "error", err)
		}
	}
}
