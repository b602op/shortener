package handler

import (
	"encoding/json"
	"net/http"

	"github.com/b602op/shortener/internal/auth"
)

// MethodDeleteUserURLs асинхронно помечает сокращённые URL пользователя удалёнными.
// Тело запроса — JSON-массив идентификаторов сокращённых URL.
// 401 — пользователь не авторизован.
// 400 — некорректное тело запроса.
// 202 — запрос принят, фактическое удаление произойдёт позже.
func MethodDeleteUserURLs(deleteService URLDeleter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.GetUserIDFromContext(r.Context())
		if !ok || userID == "" {
			respondWithError(w, "Пользователь не авторизован", http.StatusUnauthorized)
			return
		}

		var shortURLs []string
		if err := json.NewDecoder(r.Body).Decode(&shortURLs); err != nil {
			respondWithError(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if err := deleteService.DeleteBatch(userID, shortURLs); err != nil {
			respondWithError(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}
