package audit

import "context"

// Observer описывает контракт приёмника событий аудита.
// Реализатор должен обработать одно событие и вернуть ошибку, если приём
// недоступен; ошибка не прерывает рассылку остальным наблюдателям.
type Observer interface {
	Notify(ctx context.Context, event Event) error
}
