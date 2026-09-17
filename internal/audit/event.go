// Package audit публикует события пользовательских действий в подключаемые
// приёмники: файл или удалённый HTTP-сервис.
package audit

// Action — тип действия аудита
type Action string

const (
	// ActionShorten — действие аудита при создании короткой ссылки.
	ActionShorten Action = "shorten"
	// ActionFollow — действие аудита при переходе по короткой ссылке.
	ActionFollow Action = "follow"
)

// Event — событие аудита: момент, действие, пользователь и исходный URL.
type Event struct {
	TS     int64  `json:"ts"`
	Action Action `json:"action"`
	UserID string `json:"user_id,omitempty"`
	URL    string `json:"url"`
}
