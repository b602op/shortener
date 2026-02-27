package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/b602op/shortener/internal/repository"
)

func MethodGet(res http.ResponseWriter, req *http.Request) {
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

	originalURL := repository.SelectData(shortURL)
	if originalURL == "" {
		respondWithError(res, "Short URL not found", http.StatusNotFound)
		return
	}

	res.Header().Set("Location", originalURL)
	res.Header().Set("Content-Type", "text/plain")
	res.WriteHeader(http.StatusTemporaryRedirect)
}
