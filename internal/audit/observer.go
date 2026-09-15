package audit

import "context"

// Observer — приёмник событий аудита
type Observer interface {
	Notify(ctx context.Context, event Event) error
}
