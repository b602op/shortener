package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
)

func MethodPost(cfg *config.Config) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		slog.Info("Получен POST запрос", "uri", req.RequestURI)

		if req.Method != http.MethodPost {
			respondWithError(res, "Метод не разрешен", http.StatusMethodNotAllowed)
			return
		}

		defer req.Body.Close()

		body, err := io.ReadAll(req.Body)
		slog.Debug("Тело запроса", "body", string(body))

		if err != nil {
			respondWithError(res, "Ошибка чтения тела", http.StatusBadRequest)
			return
		}
		slog.Debug("Длина тела", "length", len(body))
		if len(body) == 0 {
			respondWithError(res, "Пустое тело запроса", http.StatusBadRequest)
			return
		}

		//сокращаем url
		hash := sha256.Sum256([]byte(body))
		slog.Debug("Хеш", "hash", hex.EncodeToString(hash[:4]))

		shortURL := cfg.GetBaseURL() + "/" + hex.EncodeToString(hash[:4])

		slog.Info("Сокращённый URL создан", "shortURL", shortURL)

		res.Header().Set("content-type", "text/plain")
		res.Header().Set("Content-Length", strconv.Itoa(len(shortURL)))
		res.WriteHeader(http.StatusCreated)
		res.Write([]byte(shortURL))

		repository.InsertData(string(body), hex.EncodeToString(hash[:4]))
	}
}
