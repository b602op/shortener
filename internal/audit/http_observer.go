package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"
)

const (
	// maxRetries — максимальное число повторных попыток при 5xx.
	maxRetries = 3

	// baseRetryDelay — базовая задержка для экспоненциального backoff.
	baseRetryDelay = 200 * time.Millisecond

	// maxRetryDelay — верхняя граница задержки между попытками.
	maxRetryDelay = 2 * time.Second
)

// HTTPObserver отправляет события аудита на удалённый сервер методом POST
// с таймаутом 5 секунд.
//
// При транзиентных ошибках (5xx, сетевые ошибки) выполняет до maxRetries
// повторных попыток с экспоненциальной задержкой. Ошибки 4xx не ретраятся —
// это постоянные ошибки.
type HTTPObserver struct {
	url    string
	client *http.Client
}

// NewHTTPObserver создаёт приёмник, публикующий события JSON-телом на url.
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		url: url,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Notify отправляет событие как JSON на настроенный URL.
//
// При ошибках транспорта или ответах 5xx выполняет повторные попытки
// с экспоненциальной задержкой. Ошибки 4xx и ошибки маршалинга не ретраятся.
// Контекст проверяется перед каждой попыткой — если он отменён, возвращается ошибка.
func (o *HTTPObserver) Notify(ctx context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга события: %w", err)
	}

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := backoffDelay(attempt)
			slog.Debug("Повторная отправка аудита",
				"attempt", attempt,
				"max", maxRetries,
				"delay", delay,
				"url", o.url,
			)

			select {
			case <-ctx.Done():
				return fmt.Errorf("контекст отменён во время ожидания: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		err := o.doRequest(ctx, data)
		if err == nil {
			return nil
		}

		// 4xx — постоянная ошибка, не ретраим
		var statusErr *httpStatusError
		if errors.As(err, &statusErr) && statusErr.Code < 500 {
			return err
		}

		lastErr = err
	}

	return fmt.Errorf("не удалось отправить событие после %d попыток: %w",
		maxRetries+1, lastErr)
}

// doRequest выполняет один HTTP-запрос и проверяет статус ответа.
func (o *HTTPObserver) doRequest(ctx context.Context, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка отправки события: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return &httpStatusError{Code: resp.StatusCode}
	}
	return nil
}

// httpStatusError — ошибка ответа сервера с кодом статуса.
type httpStatusError struct {
	Code int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("сервер аудита вернул статус %d", e.Code)
}

// backoffDelay рассчитывает задержку для попытки attempt (attempt >= 1)
// по формуле min(baseRetryDelay * 2^(attempt-1), maxRetryDelay).
func backoffDelay(attempt int) time.Duration {
	delay := time.Duration(float64(baseRetryDelay) * math.Pow(2, float64(attempt-1)))
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	return delay
}
