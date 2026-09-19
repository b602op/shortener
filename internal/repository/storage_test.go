package repository

import (
	"encoding/json"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewFileStorage проверяет создание нового хранилища
func TestNewFileStorage(t *testing.T) {
	storage := NewFileStorage()

	assert.NotNil(t, storage)
	assert.NotNil(t, storage.data)
	assert.Empty(t, storage.data)
	assert.Empty(t, storage.filePath)
}

// TestStorage_Init_LoadExistingFile проверяет загрузку из существующего файла
func TestStorage_Init_LoadExistingFile(t *testing.T) {
	testFile := "test_storage_load.json"
	defer func() { _ = os.Remove(testFile) }()

	records := []URLRecord{
		{
			UUID:        "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			ShortURL:    "abc123",
			OriginalURL: "http://yandex.ru",
		},
		{
			UUID:        "123e4567-e89b-12d3-a456-426614174000",
			ShortURL:    "def456",
			OriginalURL: "http://ya.ru",
		},
	}

	// Записываем в формате JSON-lines (по одному объекту на строку)
	f, err := os.Create(testFile)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	for _, r := range records {
		require.NoError(t, enc.Encode(r))
	}

	storage := NewFileStorage()
	err = storage.Init(testFile)
	require.NoError(t, err)

	record1, ok := storage.Select("abc123")
	assert.True(t, ok)
	assert.Equal(t, "http://yandex.ru", record1.OriginalURL)
	assert.Equal(t, "f47ac10b-58cc-4372-a567-0e02b2c3d479", record1.UUID)

	record2, ok := storage.Select("def456")
	assert.True(t, ok)
	assert.Equal(t, "http://ya.ru", record2.OriginalURL)
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", record2.UUID)
}

// TestStorage_Init_FileNotExists проверяет, что если файла нет, ошибки не будет
func TestStorage_Init_FileNotExists(t *testing.T) {
	testFile := "nonexistent_file.json"
	defer func() { _ = os.Remove(testFile) }()

	storage := NewFileStorage()
	err := storage.Init(testFile)

	require.NoError(t, err)
	assert.Empty(t, storage.data)
}

// TestStorage_Save проверяет вставку данных (Save удалён из FileStorage)
func TestStorage_Save(t *testing.T) {
	testFile := "test_storage_save.json"
	defer func() { _ = os.Remove(testFile) }()

	storage := NewFileStorage()

	err := storage.Insert("", "http://example.com", "test123")
	require.NoError(t, err)

	err = storage.Insert("", "http://test.ru", "test456")
	require.NoError(t, err)

	_, err = os.Stat(testFile)
	require.Error(t, err)
}

// TestStorage_Insert проверяет вставку данных
func TestStorage_Insert(t *testing.T) {
	storage := NewFileStorage()

	err := storage.Insert("", "http://newurl.ru", "new123")
	require.NoError(t, err)

	found, ok := storage.Select("new123")
	assert.True(t, ok)
	assert.Equal(t, "http://newurl.ru", found.OriginalURL)
	assert.NotEmpty(t, found.UUID)
}

// TestStorage_Select проверяет поиск данных
func TestStorage_Select(t *testing.T) {
	storage := NewFileStorage()

	err := storage.Insert("", "http://url1.ru", "short1")
	require.NoError(t, err)

	err = storage.Insert("", "http://url2.ru", "short2")
	require.NoError(t, err)

	record1, ok := storage.Select("short1")
	assert.True(t, ok)
	assert.Equal(t, "http://url1.ru", record1.OriginalURL)
	assert.NotEmpty(t, record1.UUID)

	record2, ok := storage.Select("short2")
	assert.True(t, ok)
	assert.Equal(t, "http://url2.ru", record2.OriginalURL)
	assert.NotEmpty(t, record2.UUID)

	record3, ok := storage.Select("nonexistent")
	assert.False(t, ok)
	assert.Empty(t, record3.UUID)
}

// TestStorage_Concurrent проверяет конкурентный доступ
func TestStorage_Concurrent(t *testing.T) {
	storage := NewFileStorage()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			url := "http://url-" + string(rune(i+65)) + ".ru"
			short := "short-" + string(rune(i+65))
			_ = storage.Insert("", url, short)
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			short := "short-" + string(rune(i+65))
			storage.Select(short)
		}(i)
	}

	wg.Wait()
}

// TestStorage_Init_EmptyFile проверяет загрузку пустого файла
func TestStorage_Init_EmptyFile(t *testing.T) {
	testFile := "empty_file.json"
	defer func() { _ = os.Remove(testFile) }()

	err := os.WriteFile(testFile, []byte{}, 0644)
	require.NoError(t, err)

	storage := NewFileStorage()
	err = storage.Init(testFile)
	require.NoError(t, err)

	assert.Empty(t, storage.data)
}

// TestStorage_Init_InvalidJSON проверяет обработку битого JSON
func TestStorage_Init_InvalidJSON(t *testing.T) {
	testFile := "invalid_json.json"
	defer func() { _ = os.Remove(testFile) }()

	err := os.WriteFile(testFile, []byte("{это не json"), 0644)
	require.NoError(t, err)

	storage := NewFileStorage()
	err = storage.Init(testFile)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ошибка парсинга JSON")
}

// TestStorage_InsertDuplicate проверяет вставку дубликата
func TestStorage_InsertDuplicate(t *testing.T) {
	storage := NewFileStorage()

	err := storage.Insert("", "http://first.com", "test")
	require.NoError(t, err)

	err = storage.Insert("", "http://second.com", "test")
	require.NoError(t, err)

	found, ok := storage.Select("test")
	assert.True(t, ok)
	assert.Equal(t, "http://second.com", found.OriginalURL)
}

// TestStorage_BatchInsert проверяет пакетную вставку с дубликатами
func TestStorage_BatchInsert(t *testing.T) {
	storage := NewFileStorage()

	records := []URLRecord{
		{OriginalURL: "http://one.com", ShortURL: "short1"},
		{OriginalURL: "http://two.com", ShortURL: "short2"},
	}

	results, err := storage.BatchInsert("user-1", records)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "http://one.com", results[0].OriginalURL)
	assert.Equal(t, "user-1", results[0].UserID)

	// Повторная вставка с дубликатом: возвращается существующая запись
	dup := []URLRecord{
		{OriginalURL: "http://one.com", ShortURL: "other-short"},
		{OriginalURL: "http://three.com", ShortURL: "short3"},
	}
	results, err = storage.BatchInsert("user-2", dup)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "short1", results[0].ShortURL, "дубликат должен вернуть существующий short_url")
	assert.Equal(t, "user-1", results[0].UserID, "дубликат должен вернуть исходного владельца")
	assert.Equal(t, "user-2", results[1].UserID)
}

// TestStorage_SaveAndClose проверяет сохранение в файл через Save и Close
func TestStorage_SaveAndClose(t *testing.T) {
	testFile := "test_storage_save_close.json"
	defer func() { _ = os.Remove(testFile) }()

	storage := NewFileStorage()
	err := storage.Init(testFile)
	require.NoError(t, err)

	err = storage.Insert("user-1", "http://example.com", "save1")
	require.NoError(t, err)

	err = storage.Save()
	require.NoError(t, err)

	// Файл должен существовать и содержать запись
	_, err = os.Stat(testFile)
	require.NoError(t, err)

	// Close сохраняет данные и не возвращает ошибку
	err = storage.Close()
	require.NoError(t, err)

	// Проверяем, что данные читаются из файла
	reloaded := NewFileStorage()
	err = reloaded.Init(testFile)
	require.NoError(t, err)
	record, ok := reloaded.Select("save1")
	require.True(t, ok)
	assert.Equal(t, "http://example.com", record.OriginalURL)
}

// TestStorage_SelectByUser проверяет выборку URL пользователя
func TestStorage_SelectByUser(t *testing.T) {
	storage := NewFileStorage()

	_, err := storage.BatchInsert("user-1", []URLRecord{
		{OriginalURL: "http://one.com", ShortURL: "short1"},
		{OriginalURL: "http://two.com", ShortURL: "short2"},
	})
	require.NoError(t, err)
	_, err = storage.BatchInsert("user-2", []URLRecord{
		{OriginalURL: "http://three.com", ShortURL: "short3"},
	})
	require.NoError(t, err)

	// Только свои URL
	got := storage.SelectByUser("user-1")
	assert.Len(t, got, 2)

	// Незнакомый пользователь — пустой список
	got = storage.SelectByUser("nobody")
	assert.Empty(t, got)

	// Удалённые URL не возвращаются
	err = storage.DeleteByUser("user-1", []string{"short1"})
	require.NoError(t, err)
	got = storage.SelectByUser("user-1")
	assert.Len(t, got, 1)
	assert.Equal(t, "short2", got[0].ShortURL)
}

// TestStorage_DeleteByUser проверяет удаление только своих URL
func TestStorage_DeleteByUser(t *testing.T) {
	storage := NewFileStorage()

	_, err := storage.BatchInsert("user-1", []URLRecord{
		{OriginalURL: "http://one.com", ShortURL: "short1"},
	})
	require.NoError(t, err)

	// Чужой пользователь не может удалить
	err = storage.DeleteByUser("user-2", []string{"short1"})
	require.NoError(t, err)
	record, ok := storage.Select("short1")
	require.True(t, ok)
	assert.False(t, record.DeletedFlag, "чужой URL не должен быть удалён")

	// Владелец удаляет
	err = storage.DeleteByUser("user-1", []string{"short1", "unknown"})
	require.NoError(t, err)
	record, ok = storage.Select("short1")
	require.True(t, ok)
	assert.True(t, record.DeletedFlag, "свой URL должен быть помечен удалённым")
}
