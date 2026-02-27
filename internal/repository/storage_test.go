package repository

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitStorage_LoadExistingFile(t *testing.T) {
	testFile := "test_storage_load.json"
	defer os.Remove(testFile)

	records := []URLRecord{
		{UUID: "1", ShortURL: "abc123", OriginalURL: "http://yandex.ru"},
		{UUID: "2", ShortURL: "def456", OriginalURL: "http://ya.ru"},
	}

	data, err := json.Marshal(records)
	require.NoError(t, err)
	err = os.WriteFile(testFile, data, 0644)
	require.NoError(t, err)

	Storage = make(map[string]string)

	err = InitStorage(testFile)
	require.NoError(t, err)

	assert.Equal(t, "http://yandex.ru", Storage["abc123"])
	assert.Equal(t, "http://ya.ru", Storage["def456"])
}

func TestInitStorage_FileNotExists(t *testing.T) {
	testFile := "nonexistent_file.json"
	defer os.Remove(testFile)

	Storage = make(map[string]string)

	err := InitStorage(testFile)
	require.NoError(t, err)
}

func TestSaveStorage(t *testing.T) {
	testFile := "test_storage_save.json"
	defer os.Remove(testFile)

	Storage = make(map[string]string)
	filePath = testFile

	Storage["test123"] = "http://example.com"
	Storage["test456"] = "http://test.ru"

	err := SaveStorage()
	require.NoError(t, err)

	_, err = os.Stat(testFile)
	require.NoError(t, err)

	data, err := os.ReadFile(testFile)
	require.NoError(t, err)

	var records []URLRecord
	err = json.Unmarshal(data, &records)
	require.NoError(t, err)

	assert.Len(t, records, 2)

	urlMap := make(map[string]string)
	for _, r := range records {
		urlMap[r.ShortURL] = r.OriginalURL
	}

	assert.Equal(t, "http://example.com", urlMap["test123"])
	assert.Equal(t, "http://test.ru", urlMap["test456"])
}

func TestInsertData_SavesToFile(t *testing.T) {
	testFile := "test_storage_insert.json"
	defer os.Remove(testFile)

	Storage = make(map[string]string)
	filePath = testFile

	InsertData("http://newurl.ru", "new123")

	assert.Equal(t, "http://newurl.ru", Storage["new123"])
}

func TestSelectData(t *testing.T) {
	Storage = make(map[string]string)

	Storage["short1"] = "http://url1.ru"
	Storage["short2"] = "http://url2.ru"

	assert.Equal(t, "http://url1.ru", SelectData("short1"))
	assert.Equal(t, "http://url2.ru", SelectData("short2"))
	assert.Equal(t, "", SelectData("nonexistent"))
}
