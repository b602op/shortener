package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
)

type userURLResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// MethodGetUserURLs возвращает все URL, сокращённые пользователем.
// 401 — если кука присутствует, но не содержит валидный ID пользователя.
// 204 — если пользователь ещё не сокращал URL.
// 200 — список сокращённых URL в формате JSON.
func MethodGetUserURLs(cfg *config.Config, store repository.Store) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		slog.Info("Получен GET запрос к API", "uri", req.RequestURI)

		userID, ok := auth.GetUserIDFromContext(req.Context())
		if !ok || userID == "" {
			respondWithError(res, "Пользователь не авторизован", http.StatusUnauthorized)
			return
		}

		records := store.SelectByUser(userID)
		if len(records) == 0 {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		response := make([]userURLResponse, 0, len(records))
		for _, record := range records {
			response = append(response, userURLResponse{
				ShortURL:    cfg.GetBaseURL() + "/" + record.ShortURL,
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
