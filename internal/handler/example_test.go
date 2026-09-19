package handler_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"

	"github.com/b602op/shortener/internal/audit"
	"github.com/b602op/shortener/internal/auth"
	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/handler"
	"github.com/b602op/shortener/internal/repository"
	"github.com/b602op/shortener/internal/worker"
)

// ExampleHandler_shortenPOST сокращает URL через POST / с телом text/plain.
func ExampleHandler_shortenPOST() {
	cfg := config.NewTest()
	store := repository.NewFileStorage()
	authService := auth.NewService("test-secret")
	auditService := audit.NewService()

	deps := handler.Dependencies{
		Config:       cfg,
		Store:        store,
		AuthService:  authService,
		AuditService: auditService,
	}

	srv := httptest.NewServer(handler.Handler(deps))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/", "text/plain", strings.NewReader("https://example.com"))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	fmt.Println(resp.StatusCode)
	// Output: 201
}

// ExampleHandler_shortenAPI сокращает URL через POST /api/shorten с JSON-телом.
func ExampleHandler_shortenAPI() {
	cfg := config.NewTest()
	store := repository.NewFileStorage()
	authService := auth.NewService("test-secret")
	auditService := audit.NewService()

	deps := handler.Dependencies{
		Config:       cfg,
		Store:        store,
		AuthService:  authService,
		AuditService: auditService,
	}

	srv := httptest.NewServer(handler.Handler(deps))
	defer srv.Close()

	body := strings.NewReader(`{"url":"https://example.com"}`)
	resp, err := http.Post(srv.URL+"/api/shorten", "application/json", body)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	var shortenResp handler.ShortenResponse
	if err := json.NewDecoder(resp.Body).Decode(&shortenResp); err != nil {
		fmt.Println("error:", err)
		return
	}

	// Короткий адрес содержит нестабильный хеш — проверяем только статус и префикс.
	fmt.Println(resp.StatusCode)
	fmt.Println(strings.HasPrefix(shortenResp.Result, cfg.GetBaseURL()+"/"))
	// Output:
	// 201
	// true
}

// ExampleHandler_batchShorten пакетно сокращает несколько URL через POST /api/shorten/batch.
func ExampleHandler_batchShorten() {
	cfg := config.NewTest()
	store := repository.NewFileStorage()
	authService := auth.NewService("test-secret")
	auditService := audit.NewService()

	deps := handler.Dependencies{
		Config:       cfg,
		Store:        store,
		AuthService:  authService,
		AuditService: auditService,
	}

	srv := httptest.NewServer(handler.Handler(deps))
	defer srv.Close()

	batchReq := []handler.BatchShortenRequest{
		{CorrelationID: "1", OriginalURL: "https://example.com"},
		{CorrelationID: "2", OriginalURL: "https://example.org"},
	}
	body, err := json.Marshal(batchReq)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	resp, err := http.Post(srv.URL+"/api/shorten/batch", "application/json", strings.NewReader(string(body)))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	var responses []handler.BatchShortenResponse
	if err := json.NewDecoder(resp.Body).Decode(&responses); err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(resp.StatusCode)
	fmt.Println(len(responses))
	// Output:
	// 201
	// 2
}

// ExampleHandler_redirect редиректит по короткой ссылке GET /{id}.
func ExampleHandler_redirect() {
	cfg := config.NewTest()
	store := repository.NewFileStorage()
	authService := auth.NewService("test-secret")
	auditService := audit.NewService()

	deps := handler.Dependencies{
		Config:       cfg,
		Store:        store,
		AuthService:  authService,
		AuditService: auditService,
	}

	srv := httptest.NewServer(handler.Handler(deps))
	defer srv.Close()

	// Создаём короткую ссылку, чтобы было куда редиректить.
	resp, err := http.Post(srv.URL+"/", "text/plain", strings.NewReader("https://example.com"))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Идентификатор — нестабильный хеш, берём его из ответа, но не печатаем.
	id := strings.TrimPrefix(string(body), cfg.GetBaseURL()+"/")

	// Редирект не должны отслеживаться, иначе не увидеть заголовок Location.
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	redirectResp, err := client.Get(srv.URL + "/" + id)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer func() { _ = redirectResp.Body.Close() }()

	fmt.Println(redirectResp.StatusCode)
	fmt.Println(redirectResp.Header.Get("Location"))
	// Output:
	// 307
	// https://example.com
}

// ExampleHandler_userURLs запрашивает список URL текущего пользователя.
func ExampleHandler_userURLs() {
	cfg := config.NewTest()
	store := repository.NewFileStorage()
	authService := auth.NewService("test-secret")
	auditService := audit.NewService()

	deps := handler.Dependencies{
		Config:       cfg,
		Store:        store,
		AuthService:  authService,
		AuditService: auditService,
	}

	srv := httptest.NewServer(handler.Handler(deps))
	defer srv.Close()

	// Кука с идентификатором должна дожить до второго запроса.
	jar, err := cookiejar.New(nil)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	client := &http.Client{Jar: jar}

	resp, err := client.Post(srv.URL+"/", "text/plain", strings.NewReader("https://example.com"))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	_ = resp.Body.Close()

	listResp, err := client.Get(srv.URL + "/api/user/urls")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer func() { _ = listResp.Body.Close() }()

	// Ответ приходит как массив объектов, тип хендлера не экспортируется.
	var urls []struct {
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&urls); err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(listResp.StatusCode)
	fmt.Println(len(urls))
	// Output:
	// 200
	// 1
}

// ExampleHandler_deleteUserURLs помечает URL пользователя удалёнными асинхронно.
func ExampleHandler_deleteUserURLs() {
	cfg := config.NewTest()
	store := repository.NewFileStorage()
	authService := auth.NewService("test-secret")
	auditService := audit.NewService()
	deleteService := worker.NewDeleteService(store, worker.DefaultConfig())
	defer func() { _ = deleteService.Close() }()

	deps := handler.Dependencies{
		Config:        cfg,
		Store:         store,
		AuthService:   authService,
		DeleteService: deleteService,
		AuditService:  auditService,
	}

	srv := httptest.NewServer(handler.Handler(deps))
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	client := &http.Client{Jar: jar}

	// Сначала создаём URL, чтобы было что удалять.
	createResp, err := client.Post(srv.URL+"/", "text/plain", strings.NewReader("https://example.com"))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	shortBody, err := io.ReadAll(createResp.Body)
	_ = createResp.Body.Close()
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	id := strings.TrimPrefix(string(shortBody), cfg.GetBaseURL()+"/")

	body, err := json.Marshal([]string{id})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/user/urls", strings.NewReader(string(body)))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	deleteResp, err := client.Do(req)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer func() { _ = deleteResp.Body.Close() }()

	// 202 — запрос принят, фактическое удаление произойдёт позже.
	fmt.Println(deleteResp.StatusCode)
	// Output: 202
}
