package audit

// Action — тип действия аудита
type Action string

const (
	ActionShorten Action = "shorten"
	ActionFollow  Action = "follow"
)

// Event — событие аудита
type Event struct {
	TS     int64  `json:"ts"`
	Action Action `json:"action"`
	UserID string `json:"user_id,omitempty"`
	URL    string `json:"url"`
}
