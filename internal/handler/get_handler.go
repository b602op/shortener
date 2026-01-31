package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/b602op/shortener/internal/repository"
)

func MethodGet(res http.ResponseWriter, req *http.Request) {
	log.Println("Получен GET запрос: ", req.RequestURI)

	if req.Method != http.MethodGet {
		http.Error(res, "Only Get requests are allowed!", http.StatusBadRequest)
		return
	}

	// Извлекаем shortID из URL вручную
	path := strings.Trim(req.URL.Path, "/")
	if path == "" {
		res.Header().Set("Location", "")
		res.Header().Set("Content-Type", "text/plain")
		res.WriteHeader(http.StatusTemporaryRedirect)
		return
	}

	// Берем первую часть пути как shortID
	parts := strings.Split(path, "/")
	shortURL := parts[0]

	log.Println("shortURL: ", shortURL)

	originalURL := repository.SelectData(shortURL)
	if originalURL == "" {
		res.Header().Set("Location", "")
		res.Header().Set("Content-Type", "text/plain")
		res.WriteHeader(http.StatusTemporaryRedirect)
		return
	}

	res.Header().Set("Location", originalURL)
	res.Header().Set("Content-Type", "text/plain")
	res.WriteHeader(http.StatusTemporaryRedirect)
}
