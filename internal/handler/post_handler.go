package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
)

func MethodPost(cfg *config.Config, storage *repository.Storage) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		slog.Info("Получен POST запрос", "uri", req.RequestURI)

		if req.Method != http.MethodPost {
			respondWithError(res, "Метод не разрешен", http.StatusMethodNotAllowed)
			return
		}

		defer req.Body.Close()

		body, err := io.ReadAll(req.Body)
		if err != nil {
			respondWithError(res, "Ошибка чтения тела", http.StatusBadRequest)
			return
		}

		slog.Debug("Тело запроса", "body", string(body))

		if len(body) == 0 {
			respondWithError(res, "Пустое тело запроса", http.StatusBadRequest)
			return
		}

		originalURL := string(body)

		// Генерируем короткий URL
		hash := sha256.Sum256([]byte(originalURL))
		shortHash := hex.EncodeToString(hash[:4])

		// ✅ Сохраняем только один раз
		if err := storage.Insert(originalURL, shortHash); err != nil {
			slog.Error("Ошибка сохранения", "error", err)
			respondWithError(res, "Failed to save URL", http.StatusInternalServerError)
			return
		}

		shortURL := cfg.GetBaseURL() + "/" + shortHash

		slog.Info("Сокращённый URL создан", "shortURL", shortURL)

		// ✅ Отправляем ответ
		res.Header().Set("Content-Type", "text/plain")
		res.WriteHeader(http.StatusCreated)
		res.Write([]byte(shortURL))
	}
}
