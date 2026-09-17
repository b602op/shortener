package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/b602op/shortener/internal/auth"
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

// MethodPostBatchAPI возвращает обработчик POST /api/shorten/batch.
// Принимает JSON-массив элементов с correlation_id и original_url, сохраняет
// их одним вызовом BatchInsert. Ответ: 201 с массивом short_url в том же
// порядке; 400 при некорректном JSON, пустом батче или пустом url элемента.
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

		records := make([]repository.URLRecord, 0, len(batchReq))
		for _, item := range batchReq {
			if item.OriginalURL == "" {
				respondWithError(res, "URL не может быть пустым", http.StatusBadRequest)
				return
			}

			hash := sha256.Sum256([]byte(item.OriginalURL))
			records = append(records, repository.URLRecord{
				OriginalURL: item.OriginalURL,
				ShortURL:    hex.EncodeToString(hash[:4]),
			})
		}

		// Извлекаем userID из контекста (устанавливается AuthMiddleware)
		userID, _ := auth.GetUserIDFromContext(req.Context())

		// Сохраняем все записи через BatchInsert
		results, err := store.BatchInsert(userID, records)
		if err != nil {
			slog.Error("Ошибка сохранения батча", "error", err)
			respondWithError(res, "Ошибка сохранения URL", http.StatusInternalServerError)
			return
		}

		responses := make([]BatchShortenResponse, len(batchReq))
		for i := range results {
			responses[i] = BatchShortenResponse{
				CorrelationID: batchReq[i].CorrelationID,
				ShortURL:      cfg.GetBaseURL() + "/" + results[i].ShortURL,
			}
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
