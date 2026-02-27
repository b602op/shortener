package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
)

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}

func MethodPostAPI(cfg *config.Config) http.HandlerFunc {
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

		slog.Info("Сокращённый URL создан", "shortURL", shortURL)

		repository.InsertData(shortenReq.URL, shortHash)

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
