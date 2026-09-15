package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/b602op/shortener/internal/audit"
	"github.com/b602op/shortener/internal/auth"
)

// notifyAudit формирует и отправляет событие аудита во все подписанные приёмники.
// Если сервис не сконфигурирован или у него нет наблюдателей — ничего не делает.
func notifyAudit(r *http.Request, notifier AuditNotifier, action audit.Action, originalURL string) {
	if notifier == nil || !notifier.HasObservers() {
		return
	}

	userID, _ := auth.GetUserIDFromContext(r.Context())

	event := audit.Event{
		TS:     time.Now().Unix(),
		Action: action,
		UserID: userID,
		URL:    originalURL,
	}

	// Отправляем асинхронно, чтобы не задерживать ответ клиенту
	go notifier.NotifyAll(context.Background(), event)
}
