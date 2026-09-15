package handler

import (
	"context"
	"net/http"

	"github.com/b602op/shortener/internal/audit"
	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
)

// UserIDProvider — интерфейс для получения userID из запроса
type UserIDProvider interface {
	GetUserIDFromCookie(r *http.Request) (string, bool)
	SetUserIDCookie(w http.ResponseWriter)
	GenerateUserID() string
	SetUserIDCookieWithUserID(w http.ResponseWriter, userID string)
	HasCookie(r *http.Request) bool
}

// URLDeleter — интерфейс для удаления URL пользователя
type URLDeleter interface {
	DeleteBatch(userID string, shortURLs []string) error
}

type AuditNotifier interface {
	NotifyAll(ctx context.Context, event audit.Event)
	HasObservers() bool
}

// Dependencies — все зависимости, которые нужны роутеру
type Dependencies struct {
	Config        *config.Config
	Store         repository.Store
	AuthService   UserIDProvider
	DeleteService URLDeleter
	AuditService  AuditNotifier
}
