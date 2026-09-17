package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileObserver_Notify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	obs, err := NewFileObserver(path)
	require.NoError(t, err)
	defer obs.Close()

	event1 := Event{TS: 1, Action: ActionShorten, UserID: "u1", URL: "http://example.com/a"}
	event2 := Event{TS: 2, Action: ActionFollow, UserID: "u1", URL: "http://example.com/a"}

	require.NoError(t, obs.Notify(context.Background(), event1))
	require.NoError(t, obs.Notify(context.Background(), event2))

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	require.NoError(t, scanner.Err())
	require.Len(t, lines, 2, "должно быть две строки")

	var got1, got2 Event
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &got1))
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &got2))

	assert.Equal(t, event1, got1)
	assert.Equal(t, event2, got2)
}

func TestFileObserver_Appends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	// Записываем через первый observer
	obs1, err := NewFileObserver(path)
	require.NoError(t, err)
	require.NoError(t, obs1.Notify(context.Background(), Event{TS: 1, Action: ActionShorten, URL: "a"}))
	require.NoError(t, obs1.Close())

	// Второй observer открывает тот же файл и дописывает
	obs2, err := NewFileObserver(path)
	require.NoError(t, err)
	require.NoError(t, obs2.Notify(context.Background(), Event{TS: 2, Action: ActionShorten, URL: "b"}))
	require.NoError(t, obs2.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	// Два события разделены переводом строки
	assert.Contains(t, string(data), "\n")
	assert.Equal(t, 2, countLines(string(data)))
}

func countLines(s string) int {
	n := 0
	for _, r := range s {
		if r == '\n' {
			n++
		}
	}
	return n
}

func TestFileObserver_Close(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	obs, err := NewFileObserver(path)
	require.NoError(t, err)

	// Записываем событие — файл открыт
	err = obs.Notify(context.Background(), Event{Action: "test"})
	require.NoError(t, err)

	// Закрываем
	require.NoError(t, obs.Close())

	// После Close запись должна возвращать ошибку
	err = obs.Notify(context.Background(), Event{Action: "after-close"})
	assert.Error(t, err, "Notify после Close должен вернуть ошибку")

	// Повторный Close безопасен
	require.NoError(t, obs.Close())
}

func TestFileObserver_CloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	obs, err := NewFileObserver(path)
	require.NoError(t, err)

	require.NoError(t, obs.Close())
	require.NoError(t, obs.Close())
	require.NoError(t, obs.Close())
}
