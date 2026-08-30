package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
)

// BatchShortenRequest — объект запроса для пакетного сокращения
type BatchShortenRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// BatchShortenResponse — объект ответа для пакетного сокращения
type BatchShortenResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

func MethodPostBatchAPI(cfg *config.Config, store repository.Store) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		slog.Info("Получен POST запрос к API batch", "uri", req.RequestURI)

		if req.Method != http.MethodPost {
			respondWithError(res, "Метод не разрешен", http.StatusMethodNotAllowed)
			return
		}

		res.Header().Set("Content-Type", "application/json")

		defer req.Body.Close()

		body, err := io.ReadAll(req.Body)
		if err != nil {
			respondWithError(res, "Ошибка чтения тела", http.StatusBadRequest)
			return
		}

		var batchReq []BatchShortenRequest
		if err := json.Unmarshal(body, &batchReq); err != nil {
			respondWithError(res, "Неверный формат JSON", http.StatusBadRequest)
			return
		}

		// Не отправлять пустые батчи
		if len(batchReq) == 0 {
			respondWithError(res, "Пустой батч не допускается", http.StatusBadRequest)
			return
		}

		// Генерируем short URLs и подготавливаем записи
		type batchEntry struct {
			record repository.URLRecord
			resp   BatchShortenResponse
		}

		batchEntries := make([]batchEntry, 0, len(batchReq))

		for _, item := range batchReq {
			if item.OriginalURL == "" {
				respondWithError(res, "URL не может быть пустым", http.StatusBadRequest)
				return
			}

			hash := sha256.Sum256([]byte(item.OriginalURL))
			shortHash := hex.EncodeToString(hash[:4])

			shortURL := cfg.GetBaseURL() + "/" + shortHash

			batchEntries = append(batchEntries, batchEntry{
				record: repository.URLRecord{
					OriginalURL: item.OriginalURL,
					ShortURL:    shortHash,
				},
				resp: BatchShortenResponse{
					CorrelationID: item.CorrelationID,
					ShortURL:      shortURL,
				},
			})

			slog.Debug("URL для сокращения", "url", item.OriginalURL)
		}

		// Извлекаем записи для сохранения
		records := make([]repository.URLRecord, len(batchEntries))
		for i, entry := range batchEntries {
			records[i] = entry.record
		}

		// Сохраняем все записи через BatchInsert
		if err := store.BatchInsert(records); err != nil {
			if errors.Is(err, repository.ErrDuplicateURL) {
				slog.Warn("Дубликат URL в батче")
				respondWithError(res, "Дубликат URL", http.StatusConflict)
				return
			}
			slog.Error("Ошибка сохранения батча", "error", err)
			respondWithError(res, "Ошибка сохранения URL", http.StatusInternalServerError)
			return
		}

		// Формируем ответ
		responses := make([]BatchShortenResponse, len(batchEntries))
		for i, entry := range batchEntries {
			responses[i] = entry.resp
		}

		respBody, err := json.Marshal(responses)
		if err != nil {
			respondWithError(res, "Ошибка формирования ответа", http.StatusInternalServerError)
			return
		}

		slog.Info("Батч URLs создан", "count", len(responses))

		res.WriteHeader(http.StatusCreated)
		res.Write(respBody)
	}
}
