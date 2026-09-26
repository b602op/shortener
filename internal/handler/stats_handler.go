package handler

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"

	"github.com/b602op/shortener/internal/repository"
)

// StatsResponse — ответ эндпоинта /api/internal/stats.
type StatsResponse struct {
	URLs  int `json:"urls"`
	Users int `json:"users"`
}

// StatsHandler обрабатывает GET /api/internal/stats.
//
// Доступ разрешён только клиентам из доверенной подсети (trusted_subnet).
// IP клиента берётся из заголовка X-Real-IP.
type StatsHandler struct {
	store         repository.Store
	trustedSubnet *net.IPNet // nil — доступ запрещён всем
}

// NewStatsHandler создаёт обработчик статистики.
// trustedSubnet == nil означает пустой конфиг: доступ запрещён всем.
func NewStatsHandler(store repository.Store, trustedSubnet *net.IPNet) *StatsHandler {
	return &StatsHandler{
		store:         store,
		trustedSubnet: trustedSubnet,
	}
}

// GetStats возвращает статистику сервиса: количество сокращённых URL
// и уникальных пользователей.
//
// Статусы:
//   - 200 — IP из X-Real-IP входит в доверенную подсеть;
//   - 400 — невалидный IP в X-Real-IP;
//   - 403 — нет заголовка X-Real-IP, IP вне подсети или подсеть не задана;
//   - 500 — ошибка хранилища.
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	switch code := h.checkAccess(r); code {
	case http.StatusOK:
		// доступ разрешён — продолжаем
	case http.StatusBadRequest:
		http.Error(w, "Invalid X-Real-IP", http.StatusBadRequest)
		return
	default:
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Сбор статистики из хранилища.
	urls, err := h.store.CountURLs()
	if err != nil {
		slog.Error("ошибка подсчёта URL", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	users, err := h.store.CountUsers()
	if err != nil {
		slog.Error("ошибка подсчёта пользователей", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Ответ — только счётчики.
	resp := StatsResponse{URLs: urls, Users: users}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("ошибка сериализации ответа", "error", err)
	}
}

// checkAccess проверяет IP из заголовка X-Real-IP по доверенной подсети.
//
// Возвращает:
//   - 200 — доступ разрешён;
//   - 400 — невалидный IP в X-Real-IP;
//   - 403 — доступ запрещён (нет заголовка, пустой конфиг, IP не в подсети).
func (h *StatsHandler) checkAccess(r *http.Request) int {
	// Пустой конфиг — запрет всем.
	if h.trustedSubnet == nil {
		return http.StatusForbidden
	}

	ipStr := r.Header.Get("X-Real-IP")
	// Нет заголовка — запрет.
	if ipStr == "" {
		return http.StatusForbidden
	}

	ip := net.ParseIP(ipStr)
	// Невалидный IP — ошибка запроса.
	if ip == nil {
		return http.StatusBadRequest
	}

	// IP вне подсети — запрет.
	if !h.trustedSubnet.Contains(ip) {
		return http.StatusForbidden
	}

	return http.StatusOK
}
