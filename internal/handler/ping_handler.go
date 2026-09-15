package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"
)

// PingHandler проверяет доступность базы данных и отвечает 200 OK.
// При недоступности БД или истечении 3-секундного таймаута отвечает 500.
func PingHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "Database connection failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}
