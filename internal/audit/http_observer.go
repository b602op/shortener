package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPObserver отправляет события аудита на удалённый сервер методом POST
type HTTPObserver struct {
	url    string
	client *http.Client
}

// NewHTTPObserver создаёт HTTP-приёмник
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		url: url,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Notify отправляет событие на удалённый сервер
func (o *HTTPObserver) Notify(ctx context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга события: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка отправки события: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("сервер аудита вернул статус %d", resp.StatusCode)
	}
	return nil
}
