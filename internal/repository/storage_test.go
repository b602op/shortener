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
	defer os.Remove(testFile)

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

	data, err := json.Marshal(records)
	require.NoError(t, err)
	err = os.WriteFile(testFile, data, 0644)
	require.NoError(t, err)

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
	defer os.Remove(testFile)

	storage := NewFileStorage()
	err := storage.Init(testFile)

	require.NoError(t, err)
	assert.Empty(t, storage.data)
}

// TestStorage_Save проверяет вставку данных (Save удалён из FileStorage)
func TestStorage_Save(t *testing.T) {
	testFile := "test_storage_save.json"
	defer os.Remove(testFile)

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
	defer os.Remove(testFile)

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
	defer os.Remove(testFile)

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
