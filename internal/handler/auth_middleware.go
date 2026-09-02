package handler

import (
	"net/http"

	"github.com/b602op/shortener/internal/auth"
)

// AuthMiddleware проверяет куку с идентификатором пользователя.
// Если куки нет или она не проходит проверку подлинности,
// генерируется новый идентификатор и пользователю выдаётся
// симметрично подписанная кука. Идентификатор пользователя
// добавляется в контекст запроса.
func AuthMiddleware(authService *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Пытаемся получить userID из куки
			userID, ok := authService.GetUserIDFromCookie(r)

			// Если куки нет или она невалидная — создаём нового пользователя
			if !ok || userID == "" {
				// Генерируем новый userID
				userID = authService.GenerateUserID()
				// Устанавливаем куку с этим userID
				authService.SetUserIDCookieWithUserID(w, userID)
			}

			// Сохраняем userID в контексте запроса
			ctx := auth.ContextWithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
