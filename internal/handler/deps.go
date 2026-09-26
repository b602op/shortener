package handler

import (
	"context"
	"net/http"

	"github.com/b602op/shortener/internal/audit"
	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/repository"
	"github.com/b602op/shortener/internal/service"
)

// UserIDProvider описывает контракт источника идентификатора пользователя.
// Реализатор должен уметь читать userID из куки запроса, создавать и
// устанавливать куку, а также различать отсутствие куки и невалидную куку.
type UserIDProvider interface {
	GetUserIDFromCookie(r *http.Request) (string, bool)
	SetUserIDCookie(w http.ResponseWriter)
	GenerateUserID() string
	SetUserIDCookieWithUserID(w http.ResponseWriter, userID string)
	HasCookie(r *http.Request) bool
}

// URLDeleter описывает контракт приёмника задач на удаление URL.
// Реализатор принимает идентификатор владельца и список коротких ссылок,
// которые требуется удалить.
type URLDeleter interface {
	DeleteBatch(userID string, shortURLs []string) error
}

// AuditNotifier описывает контракт рассылки событий аудита.
// Реализатор должен уметь уведомить всех подписчиков об одном событии и
// сообщить, есть ли хотя бы один подписчик.
type AuditNotifier interface {
	NotifyAll(ctx context.Context, event audit.Event)
	HasObservers() bool
}

// Dependencies группирует все зависимости, необходимые для сборки роутера.
// Обязательны Config и Store; ShortenerService — общий слой бизнес-логики
// (если nil, собирается поверх Store); AuthService, DeleteService и
// AuditService могут быть равны nil, если соответствующая функциональность
// не используется.
type Dependencies struct {
	Config           *config.Config
	Store            repository.Store
	ShortenerService *service.ShortenerService
	AuthService      UserIDProvider
	DeleteService    URLDeleter
	AuditService     AuditNotifier
}
