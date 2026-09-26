// Package service содержит общий слой бизнес-логики, используемый
// одновременно HTTP-хендлерами и gRPC-сервером.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/b602op/shortener/internal/domain"
	"github.com/b602op/shortener/internal/repository"
)

// maxURLLength — максимальная длина исходного URL.
const maxURLLength = 2048

// ShortenerService — фасад бизнес-логики сокращения URL.
// Используется и HTTP-хендлерами, и gRPC-сервером, поэтому валидация URL
// и правила работы с хранилищем описаны ровно один раз.
// Потокобезопасен: всё состояние иммутабельно, безопасность обеспечивает Store.
type ShortenerService struct {
	store   repository.Store
	baseURL string
}

// NewShortenerService создаёт сервис поверх переданного хранилища.
// baseURL — префикс коротких ссылок (например, "http://localhost:8080").
func NewShortenerService(store repository.Store, baseURL string) *ShortenerService {
	return &ShortenerService{
		store:   store,
		baseURL: baseURL,
	}
}

// ShortenURL валидирует исходный URL, сохраняет его и возвращает полную
// короткую ссылку (baseURL + "/" + id).
//
// Возвращает:
//   - domain.ErrInvalidURL — при невалидном URL;
//   - domain.ErrDuplicateURL — если URL уже был сокращён; при этом короткая
//     ссылка всё равно возвращается (первый результат), так как идентификатор
//     детерминирован (хеш от URL).
func (s *ShortenerService) ShortenURL(_ context.Context, userID, originalURL string) (string, error) {
	if err := validateURL(originalURL); err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrInvalidURL, err)
	}

	shortID := generateShortID(originalURL)

	if err := s.store.Insert(userID, originalURL, shortID); err != nil {
		if errors.Is(err, repository.ErrDuplicateURL) {
			// Идентификатор детерминирован, поэтому существующая короткая
			// ссылка совпадает с только что сгенерированной.
			return s.FullShortURL(shortID), domain.ErrDuplicateURL
		}
		return "", fmt.Errorf("ошибка сохранения: %w", err)
	}

	return s.FullShortURL(shortID), nil
}

// ExpandURL возвращает оригинальный URL по короткому идентификатору.
//
// Возвращает domain.ErrNotFound, если ссылка не найдена, и domain.ErrDeleted,
// если она помечена удалённой.
func (s *ShortenerService) ExpandURL(_ context.Context, shortID string) (string, error) {
	record, ok := s.store.Select(shortID)
	if !ok {
		return "", domain.ErrNotFound
	}

	if record.DeletedFlag {
		return "", domain.ErrDeleted
	}

	return record.OriginalURL, nil
}

// ListUserURLs возвращает все неудалённые ссылки, сокращённые пользователем.
func (s *ShortenerService) ListUserURLs(_ context.Context, userID string) ([]repository.URLRecord, error) {
	return s.store.SelectByUser(userID), nil
}

// FullShortURL возвращает полную короткую ссылку по идентификатору.
func (s *ShortenerService) FullShortURL(shortID string) string {
	return s.baseURL + "/" + shortID
}

// BaseURL возвращает префикс коротких ссылок.
func (s *ShortenerService) BaseURL() string {
	return s.baseURL
}

// generateShortID возвращает детерминированный короткий идентификатор:
// первые 4 байта SHA-256 от исходного URL в hex-кодировке.
func generateShortID(originalURL string) string {
	hash := sha256.Sum256([]byte(originalURL))
	return hex.EncodeToString(hash[:4])
}

// validateURL проверяет исходный URL: непустоту, длину, схему http/https
// и успешный парсинг. Общая для HTTP и gRPC.
func validateURL(raw string) error {
	if raw == "" {
		return errors.New("URL пустой")
	}

	if len(raw) > maxURLLength {
		return fmt.Errorf("URL длиннее %d символов", maxURLLength)
	}

	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return errors.New("URL должен начинаться с http:// или https://")
	}

	if _, err := url.ParseRequestURI(raw); err != nil {
		return fmt.Errorf("невалидный URL: %w", err)
	}

	return nil
}
