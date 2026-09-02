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
	CookieName   = "user_id"
	CookieMaxAge = 3600 * 24 * 30 // 30 дней
)

// contextKey — тип для ключей контекста
type contextKey string

const UserIDKey contextKey = "user_id"

// Service — сервис аутентификации с JWT
type Service struct {
	secretKey []byte
}

// NewService создаёт новый сервис аутентификации
func NewService(secretKey string) *Service {
	return &Service{secretKey: []byte(secretKey)}
}

// Claims — структура JWT-claims
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// generateUserID создаёт новый UUID (только для новых пользователей)
func (s *Service) GenerateUserID() string {
	return uuid.New().String()
}

// BuildJWTString создаёт токен с НОВЫМ userID
func (s *Service) BuildJWTString() (string, error) {
	userID := s.GenerateUserID()
	return s.BuildJWTStringWithUserID(userID)
}

// BuildJWTStringWithUserID создаёт токен с указанным userID
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

// Verify проверяет JWT-токен и возвращает userID
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

// GetUserIDFromCookie извлекает и проверяет user_id из куки
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

// SetUserIDCookie устанавливает JWT-куку для нового пользователя
func (s *Service) SetUserIDCookie(w http.ResponseWriter) {
	userID := s.GenerateUserID()
	s.SetUserIDCookieWithUserID(w, userID)
}

// SetUserIDCookieWithUserID устанавливает JWT-куку с указанным userID
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

// GetUserIDFromContext извлекает user_id из контекста
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

// ContextWithUserID сохраняет user_id в контексте
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}
