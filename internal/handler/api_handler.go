package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/b602op/shortener/internal/audit"
	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/domain"
	"github.com/b602op/shortener/internal/service"
)

// ShortenRequest — тело JSON-запроса к API сокращения.
type ShortenRequest struct {
	URL string `json:"url"`
}

// ShortenResponse — тело JSON-ответа с полным коротким URL.
type ShortenResponse struct {
	Result string `json:"result"`
}

// MethodPostAPI возвращает обработчик POST /api/shorten с JSON-телом.
// Ожидает объект с полем url; валидация и сохранение — в общем сервисе.
// Ответ: 201 с полем result, 409 с существующим адресом при дубликате,
// 400 при некорректном JSON, пустом или невалидном url, 500 при ошибке сохранения.
func MethodPostAPI(svc *service.ShortenerService, auditService AuditNotifier) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		slog.Info("Получен POST запрос к API", "uri", req.RequestURI)

		if req.Method != http.MethodPost {
			respondWithError(res, "Метод не разрешен", http.StatusMethodNotAllowed)
			return
		}

		defer func() { _ = req.Body.Close() }()

		body, err := io.ReadAll(req.Body)
		if err != nil {
			respondWithError(res, "Ошибка чтения тела", http.StatusBadRequest)
			return
		}

		var shortenReq ShortenRequest
		if err = json.Unmarshal(body, &shortenReq); err != nil {
			respondWithError(res, "Неверный формат JSON", http.StatusBadRequest)
			return
		}

		if shortenReq.URL == "" {
			respondWithError(res, "URL не может быть пустым", http.StatusBadRequest)
			return
		}

		slog.Debug("URL для сокращения", "url", shortenReq.URL)

		// Извлекаем userID из контекста (устанавливается AuthMiddleware)
		userID, _ := auth.GetUserIDFromContext(req.Context())

		// Валидация и сохранение — в общем сервисе
		shortURL, err := svc.ShortenURL(req.Context(), userID, shortenReq.URL)
		if err != nil {
			if errors.Is(err, domain.ErrDuplicateURL) {
				slog.Warn("Дубликат URL", "url", shortenReq.URL)
				res.Header().Set("Content-Type", "application/json")
				res.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(res).Encode(ShortenResponse{Result: shortURL})
				return
			}
			if errors.Is(err, domain.ErrInvalidURL) {
				respondWithError(res, "Невалидный URL", http.StatusBadRequest)
				return
			}
			slog.Error("Ошибка сохранения", "error", err)
			respondWithError(res, "Ошибка сохранения URL", http.StatusInternalServerError)
			return
		}

		notifyAudit(req, auditService, audit.ActionShorten, shortenReq.URL)

		slog.Info("Сокращённый URL создан", "shortURL", shortURL)

		shortenResp := ShortenResponse{Result: shortURL}
		respBody, err := json.Marshal(shortenResp)
		if err != nil {
			respondWithError(res, "Ошибка формирования ответа", http.StatusInternalServerError)
			return
		}

		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusCreated)
		_, _ = res.Write(respBody)
	}
}
