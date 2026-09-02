package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/b602op/shortener/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware_IssuesCookie(t *testing.T) {
	authService := auth.NewService("test-secret")
	middleware := AuthMiddleware(authService)

	var ctxUserID string
	var ctxOK bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxUserID, ctxOK = auth.GetUserIDFromContext(r.Context())
		t.Logf("ctxUserID=%q, ctxOK=%v", ctxUserID, ctxOK)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	middleware(next).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	// Закрываем тело ответа
	resp := rec.Result()
	defer resp.Body.Close()

	// Кука должна быть выдана
	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, auth.CookieName, cookies[0].Name)

	// Идентификатор должен попасть в контекст
	require.True(t, ctxOK)
	require.NotEmpty(t, ctxUserID, "userID должен быть установлен в контексте")

	// Проверяем, что кука валидная и содержит правильный userID
	issuedUserID, err := authService.Verify(cookies[0].Value)
	require.NoError(t, err)
	assert.Equal(t, ctxUserID, issuedUserID)
}

func TestAuthMiddleware_KeepsValidCookie(t *testing.T) {
	authService := auth.NewService("test-secret")
	middleware := AuthMiddleware(authService)

	// Сначала получаем подписанную куку
	firstRec := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodGet, "/", nil)
	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(firstRec, firstReq)

	// Закрываем тело первого ответа
	firstResp := firstRec.Result()
	defer firstResp.Body.Close()
	cookie := firstResp.Cookies()[0]

	// Повторный запрос с той же кукой — новая кука не выдаётся
	var ctxUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxUserID, _ = auth.GetUserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	secondReq := httptest.NewRequest(http.MethodGet, "/", nil)
	secondReq.AddCookie(cookie)
	secondRec := httptest.NewRecorder()

	middleware(next).ServeHTTP(secondRec, secondReq)

	// Закрываем тело второго ответа
	secondResp := secondRec.Result()
	defer secondResp.Body.Close()

	assert.Empty(t, secondResp.Cookies(), "кука не должна перевыдаваться")

	// Проверяем, что userID в контексте совпадает с userID из куки
	expectedUserID, err := authService.Verify(cookie.Value)
	require.NoError(t, err)
	assert.Equal(t, ctxUserID, expectedUserID)
}

func TestAuthMiddleware_ReissuesInvalidCookie(t *testing.T) {
	authService := auth.NewService("test-secret")
	middleware := AuthMiddleware(authService)

	var ctxUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxUserID, _ = auth.GetUserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "tampered.signature"})
	rec := httptest.NewRecorder()

	middleware(next).ServeHTTP(rec, req)

	// Закрываем тело ответа
	resp := rec.Result()
	defer resp.Body.Close()

	// Кука с неверной подписью должна быть заменена на новую
	cookies := resp.Cookies()
	require.Len(t, cookies, 1)

	require.NotEmpty(t, ctxUserID, "userID должен быть установлен в контексте")

	newUserID, err := authService.Verify(cookies[0].Value)
	require.NoError(t, err)
	assert.NotEqual(t, "tampered.signature", cookies[0].Value)
	assert.Equal(t, ctxUserID, newUserID)
}

func TestAuthMiddleware_RejectsForeignSignedCookie(t *testing.T) {
	foreign := auth.NewService("other-secret")
	authService := auth.NewService("test-secret")
	middleware := AuthMiddleware(authService)

	// Создаём токен с другим секретом
	userID := "some-user"
	foreignToken, err := foreign.BuildJWTStringWithUserID(userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: foreignToken})
	rec := httptest.NewRecorder()

	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec, req)

	// Закрываем тело ответа
	resp := rec.Result()
	defer resp.Body.Close()

	// Должна быть выдана новая кука (с нашим секретом)
	cookies := resp.Cookies()
	require.Len(t, cookies, 1)

	// Проверяем, что новая кука валидна
	_, err = authService.Verify(cookies[0].Value)
	assert.NoError(t, err, "должна быть выдана новая валидная кука")
}

func TestUserIDFromContext_Missing(t *testing.T) {
	// Используем auth.GetUserIDFromContext напрямую
	_, ok := auth.GetUserIDFromContext(context.Background())
	assert.False(t, ok)
}

func mustParse(t *testing.T, svc *auth.Service, token string) string {
	t.Helper()
	userID, err := svc.Verify(token)
	require.NoError(t, err)
	return userID
}
