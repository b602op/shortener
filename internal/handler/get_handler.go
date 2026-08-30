package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
)

func MethodGet(cfg *config.Config, store repository.Store) http.HandlerFunc {
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

		record, ok := store.Select(shortURL)
		if !ok {
			respondWithError(res, "Short URL not found", http.StatusNotFound)
			return
		}

		res.Header().Set("Location", record.OriginalURL)
		res.Header().Set("Content-Type", "text/plain")
		res.WriteHeader(http.StatusTemporaryRedirect)
	}
}
