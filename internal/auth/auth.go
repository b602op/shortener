// Package auth выдаёт и проверяет JWT-идентификаторы пользователей
// и переносит их между HTTP-кукой и контекстом запроса.
package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

const (
	// CookieName — имя куки, в которой хранится JWT с идентификатором пользователя.
	CookieName = "user_id"
	// CookieMaxAge — время жизни куки и токена, 30 дней в секундах.
	CookieMaxAge = 3600 * 24 * 30 // 30 дней
)

// contextKey — тип для ключей контекста
type contextKey string

// UserIDKey — ключ контекста, под которым лежит строковый идентификатор пользователя.
const UserIDKey contextKey = "user_id"

// Service подписывает и проверяет JWT пользователей.
// Потокобезопасен: секрет только читается.
type Service struct {
	secretKey []byte
}

// NewService создаёт сервис аутентификации, подписывающий токены секретом secretKey.
func NewService(secretKey string) *Service {
	return &Service{secretKey: []byte(secretKey)}
}

// Claims — набор утверждений токена: идентификатор пользователя плюс
// стандартные регистрации JWT (срок действия, время выпуска).
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// GenerateUserID возвращает новый случайный UUID v4 для нового пользователя.
func (s *Service) GenerateUserID() string {
	return uuid.New().String()
}

// BuildJWTString создаёт подписанный токен с новым случайно сгенерированным userID.
// Возвращает ошибку подписи, если секрет не подходит алгоритму HS256.
func (s *Service) BuildJWTString() (string, error) {
	userID := s.GenerateUserID()
	return s.BuildJWTStringWithUserID(userID)
}

// BuildJWTStringWithUserID создаёт подписанный токен HS256 с указанным userID
// и сроком действия CookieMaxAge. Возвращает ошибку подписи.
func (s *Service) BuildJWTStringWithUserID(userID string) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(CookieMaxAge * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey) // ← secretKey
}

// Verify проверяет подпись и срок действия токена и возвращает userID из claims.
// Ошибкой считается чужой алгоритм подписи, неверная подпись, истёкший
// или некорректно собранный токен.
func (s *Service) Verify(tokenString string) (string, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secretKey, nil // ← secretKey
	})

	if err != nil {
		return "", err
	}

	if !token.Valid {
		return "", errors.New("invalid token")
	}

	return claims.UserID, nil
}

// GetUserIDFromCookie читает куку CookieName из запроса и проверяет её подпись.
// Возвращает false, если куки нет или токен не прошёл Verify.
func (s *Service) GetUserIDFromCookie(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return "", false
	}

	userID, err := s.Verify(cookie.Value)
	if err != nil {
		return "", false
	}

	return userID, true
}

// HasCookie сообщает, присутствует ли кука CookieName в запросе.
// Валидность токена не проверяется.
func (s *Service) HasCookie(r *http.Request) bool {
	_, err := r.Cookie(CookieName)
	return err == nil
}

// SetUserIDCookie генерирует новый userID и устанавливает JWT-куку для клиента.
func (s *Service) SetUserIDCookie(w http.ResponseWriter) {
	userID := s.GenerateUserID()
	s.SetUserIDCookieWithUserID(w, userID)
}

// SetUserIDCookieWithUserID устанавливает HTTP-only куку с токеном указанного userID.
// При ошибке подписи кука не устанавливается.
func (s *Service) SetUserIDCookieWithUserID(w http.ResponseWriter, userID string) {
	tokenString, err := s.BuildJWTStringWithUserID(userID)
	if err != nil {
		return
	}

	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    tokenString,
		Path:     "/",
		MaxAge:   CookieMaxAge,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, cookie)
}

// GetUserIDFromContext читает идентификатор пользователя по ключу UserIDKey.
// Возвращает false, если в контексте нет значения этого типа.
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

// ContextWithUserID возвращает дочерний контекст с сохранённым идентификатором пользователя.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}
