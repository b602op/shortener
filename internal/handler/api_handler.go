package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
)

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}

func MethodPostAPI(cfg *config.Config, store repository.Store) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		slog.Info("Получен POST запрос к API", "uri", req.RequestURI)

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

		var shortenReq ShortenRequest
		if err := json.Unmarshal(body, &shortenReq); err != nil {
			respondWithError(res, "Неверный формат JSON", http.StatusBadRequest)
			return
		}

		if shortenReq.URL == "" {
			respondWithError(res, "URL не может быть пустым", http.StatusBadRequest)
			return
		}

		slog.Debug("URL для сокращения", "url", shortenReq.URL)

		hash := sha256.Sum256([]byte(shortenReq.URL))
		shortHash := hex.EncodeToString(hash[:4])

		shortURL := cfg.GetBaseURL() + "/" + shortHash

		// Извлекаем userID из контекста (устанавливается AuthMiddleware)
		userID, _ := auth.GetUserIDFromContext(req.Context())

		// Сохраняем через переданный store
		if err := store.Insert(userID, shortenReq.URL, shortHash); err != nil {
			if errors.Is(err, repository.ErrDuplicateURL) {
				slog.Warn("Дубликат URL", "url", shortenReq.URL)
				res.Header().Set("Content-Type", "application/json")
				res.WriteHeader(http.StatusConflict)
				json.NewEncoder(res).Encode(ShortenResponse{Result: cfg.GetBaseURL() + "/" + shortHash})
				return
			}
			slog.Error("Ошибка сохранения", "error", err)
			respondWithError(res, "Ошибка сохранения URL", http.StatusInternalServerError)
			return
		}

		slog.Info("Сокращённый URL создан", "shortURL", shortURL)

		shortenResp := ShortenResponse{Result: shortURL}
		respBody, err := json.Marshal(shortenResp)
		if err != nil {
			respondWithError(res, "Ошибка формирования ответа", http.StatusInternalServerError)
			return
		}

		res.WriteHeader(http.StatusCreated)
		res.Write(respBody)
	}
}
