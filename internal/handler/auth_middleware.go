package handler

import (
	"net/http"

	"github.com/b602op/shortener/internal/auth"
)

// AuthMiddleware проверяет куку с идентификатором пользователя.
func AuthMiddleware(authService UserIDProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := authService.GetUserIDFromCookie(r)
			if !ok || userID == "" {
				// Кука есть, но невалидная — 401
				if authService.HasCookie(r) {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				// Куки нет вообще — создаём нового пользователя
				userID = authService.GenerateUserID()
				authService.SetUserIDCookieWithUserID(w, userID)
			}

			ctx := auth.ContextWithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
