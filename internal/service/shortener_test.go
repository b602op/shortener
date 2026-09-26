package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/b602op/shortener/internal/domain"
	"github.com/b602op/shortener/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBaseURL = "http://localhost:8080"

func TestShortenerService_ShortenURL(t *testing.T) {
	store := repository.NewFileStorage()
	svc := NewShortenerService(store, testBaseURL)

	t.Run("валидный URL", func(t *testing.T) {
		result, err := svc.ShortenURL(context.Background(), "user1", "https://example.com")
		require.NoError(t, err)
		assert.Contains(t, result, testBaseURL+"/")

		// Идентификатор детерминирован: хеш от URL
		id := strings.TrimPrefix(result, testBaseURL+"/")
		record, ok := store.Select(id)
		require.True(t, ok)
		assert.Equal(t, "https://example.com", record.OriginalURL)
	})

	t.Run("дубликат возвращает существующую ссылку", func(t *testing.T) {
		first, err := svc.ShortenURL(context.Background(), "user1", "https://example.com/dup")
		require.NoError(t, err)

		second, err := svc.ShortenURL(context.Background(), "user2", "https://example.com/dup")
		require.ErrorIs(t, err, domain.ErrDuplicateURL)
		assert.Equal(t, first, second)
	})

	t.Run("невалидные URL", func(t *testing.T) {
		for _, raw := range []string{"", "ftp://example.com", "not-a-url", "https://" + strings.Repeat("a", 2100)} {
			_, err := svc.ShortenURL(context.Background(), "user1", raw)
			require.ErrorIs(t, err, domain.ErrInvalidURL, "URL %q должен быть невалидным", raw)
		}
	})
}

func TestShortenerService_ExpandURL(t *testing.T) {
	store := repository.NewFileStorage()
	require.NoError(t, store.Insert("user1", "https://example.com/orig", "abc1"))
	require.NoError(t, store.Insert("user1", "https://example.com/gone", "gone1"))
	require.NoError(t, store.DeleteByUser("user1", []string{"gone1"}))

	svc := NewShortenerService(store, testBaseURL)

	t.Run("существующий ID", func(t *testing.T) {
		result, err := svc.ExpandURL(context.Background(), "abc1")
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/orig", result)
	})

	t.Run("несуществующий ID", func(t *testing.T) {
		_, err := svc.ExpandURL(context.Background(), "missing")
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("удалённый ID", func(t *testing.T) {
		_, err := svc.ExpandURL(context.Background(), "gone1")
		require.ErrorIs(t, err, domain.ErrDeleted)
	})
}

func TestShortenerService_ListUserURLs(t *testing.T) {
	store := repository.NewFileStorage()
	require.NoError(t, store.Insert("user1", "https://example.com/1", "id1"))
	require.NoError(t, store.Insert("user1", "https://example.com/2", "id2"))
	require.NoError(t, store.Insert("user2", "https://example.com/other", "id3"))

	svc := NewShortenerService(store, testBaseURL)

	records, err := svc.ListUserURLs(context.Background(), "user1")
	require.NoError(t, err)
	require.Len(t, records, 2)

	records, err = svc.ListUserURLs(context.Background(), "nobody")
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestShortenerService_FullShortURL(t *testing.T) {
	svc := NewShortenerService(repository.NewFileStorage(), testBaseURL)
	assert.Equal(t, testBaseURL+"/abc", svc.FullShortURL("abc"))
	assert.Equal(t, testBaseURL, svc.BaseURL())

	// Ошибки домена — самостоятельные значения
	assert.False(t, errors.Is(domain.ErrNotFound, domain.ErrDeleted))
}
