package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignParse_Valid(t *testing.T) {
	svc := NewService("test-secret")

	userID := svc.GenerateUserID()
	token, err := svc.BuildJWTStringWithUserID(userID)
	require.NoError(t, err)

	got, err := svc.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestParse_InvalidSignature(t *testing.T) {
	svc := NewService("test-secret")
	other := NewService("other-secret")

	userID := svc.GenerateUserID()
	token, err := svc.BuildJWTStringWithUserID(userID)
	require.NoError(t, err)

	_, err = other.Verify(token)
	assert.Error(t, err, "подпись другим ключом не должна проходить проверку")
}

func TestParse_TamperedToken(t *testing.T) {
	svc := NewService("test-secret")

	userID := svc.GenerateUserID()
	token, err := svc.BuildJWTStringWithUserID(userID)
	require.NoError(t, err)

	token += "tampered"

	_, err = svc.Verify(token)
	assert.Error(t, err)
}

func TestParse_MalformedToken(t *testing.T) {
	svc := NewService("test-secret")

	for _, token := range []string{"", "no-dot-inside", "...", "a.b.c"} {
		_, err := svc.Verify(token)
		assert.Error(t, err, "токен %q не должен проходить проверку", token)
	}
}

func TestNewUserID_Unique(t *testing.T) {
	svc := NewService("test-secret")

	seen := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		id := svc.GenerateUserID()
		_, dup := seen[id]
		assert.False(t, dup, "идентификаторы должны быть уникальными")
		seen[id] = struct{}{}
	}
}

func TestCookieRoundTrip(t *testing.T) {
	svc := NewService("test-secret")

	userID := svc.GenerateUserID()

	// Выдаём куку
	rec := httptest.NewRecorder()
	svc.SetUserIDCookieWithUserID(rec, userID)

	// Закрываем тело ответа
	resp := rec.Result()
	defer resp.Body.Close()

	// Получаем куку из ответа
	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, CookieName, cookies[0].Name)

	// Читаем куку из запроса
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])

	got, ok := svc.GetUserIDFromCookie(req)
	require.True(t, ok)
	assert.Equal(t, userID, got)
}

func TestGetUserID_NoCookie(t *testing.T) {
	svc := NewService("test-secret")

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, ok := svc.GetUserIDFromCookie(req)
	assert.False(t, ok)
}

func TestGetUserID_InvalidCookie(t *testing.T) {
	svc := NewService("test-secret")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "invalid.value"})

	_, ok := svc.GetUserIDFromCookie(req)
	assert.False(t, ok)
}
