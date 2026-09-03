package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/worker"
)

// MethodDeleteUserURLs асинхронно помечает сокращённые URL пользователя удалёнными.
// Тело запроса — JSON-массив идентификаторов сокращённых URL.
// 401 — пользователь не авторизован.
// 400 — некорректное тело запроса.
// 202 — запрос принят, фактическое удаление произойдёт позже.
func MethodDeleteUserURLs(deleteService *worker.DeleteService, authService *auth.Service) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		slog.Info("Получен DELETE запрос к API", "uri", req.RequestURI)

		// Кука присутствует, но не проходит проверку подлинности
		if _, err := req.Cookie(auth.CookieName); err == nil {
			if _, ok := authService.GetUserIDFromCookie(req); !ok {
				respondWithError(res, "Пользователь не авторизован", http.StatusUnauthorized)
				return
			}
		}

		userID, ok := auth.GetUserIDFromContext(req.Context())
		if !ok || userID == "" {
			respondWithError(res, "Пользователь не авторизован", http.StatusUnauthorized)
			return
		}

		var ids []string
		if err := json.NewDecoder(req.Body).Decode(&ids); err != nil {
			respondWithError(res, "Некорректное тело запроса", http.StatusBadRequest)
			return
		}

		for _, id := range ids {
			deleteService.Delete(userID, id)
		}

		res.WriteHeader(http.StatusAccepted)
	}
}
