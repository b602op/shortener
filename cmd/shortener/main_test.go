package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/b602op/shortener/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.T) {
	t.Skip("Integration tests not implemented")
}

// fakeCloser — тестовый ресурс, записывающий факт закрытия в общий лог.
type fakeCloser struct {
	name  string
	log   *[]string
	fails bool
}

func (f *fakeCloser) Close() error {
	*f.log = append(*f.log, f.name)
	if f.fails {
		return errCloseFailed(f.name)
	}
	return nil
}

// errCloseFailed возвращает ошибку закрытия ресурса.
func errCloseFailed(name string) error {
	return &closeError{name: name}
}

// closeError — ошибка закрытия тестового ресурса.
type closeError struct{ name string }

func (e *closeError) Error() string { return "ошибка закрытия " + e.name }

// TestCloseAll_LIFO проверяет, что ресурсы закрываются в обратном порядке (LIFO):
// при добавлении store → audit → delete закрытие идёт delete → audit → store.
func TestCloseAll_LIFO(t *testing.T) {
	var order []string

	closers := []io.Closer{
		&fakeCloser{name: "store", log: &order},
		&fakeCloser{name: "audit", log: &order},
		&fakeCloser{name: "delete", log: &order},
	}

	closeAll(closers)

	assert.Equal(t, []string{"delete", "audit", "store"}, order,
		"ресурсы должны закрываться в обратном порядке (LIFO)")
}

// TestCloseAll_WithError проверяет, что ошибка одного ресурса
// не прерывает закрытие остальных.
func TestCloseAll_WithError(t *testing.T) {
	var order []string

	closers := []io.Closer{
		&fakeCloser{name: "first", log: &order, fails: true},
		&fakeCloser{name: "second", log: &order},
	}

	closeAll(closers)

	assert.Equal(t, []string{"second", "first"}, order,
		"все ресурсы должны быть закрыты, даже если один вернул ошибку")
}

// TestCloseAll_PersistsFileStorage проверяет, что closeAll сохраняет
// данные файлового хранилища на диск — как при graceful shutdown.
func TestCloseAll_PersistsFileStorage(t *testing.T) {
	dir := t.TempDir()
	storagePath := filepath.Join(dir, "storage.json")

	store := repository.NewFileStorage()
	require.NoError(t, store.Init(storagePath))

	err := store.Insert("user-1", "https://example.com", "abc123")
	require.NoError(t, err)

	closeAll([]io.Closer{store})

	data, err := os.ReadFile(storagePath)
	require.NoError(t, err, "файл хранилища должен существовать после закрытия")
	assert.Contains(t, string(data), "https://example.com",
		"данные должны сохраниться в файл при закрытии")
}

// TestGracefulShutdown_Sigterm — интеграционный тест graceful shutdown:
// запускает собранный бинарник с файловым хранилищем, создаёт URL,
// отправляет SIGTERM и проверяет, что данные сохранились.
//
// На Windows пропускается: os.Process.Signal не поддерживает SIGTERM/SIGQUIT,
// доставить эти сигналы дочернему процессу невозможно.
func TestGracefulShutdown_Sigterm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("на Windows нельзя отправить SIGTERM дочернему процессу: " +
			"os.Process.Signal поддерживает только os.Kill; " +
			"graceful shutdown по сигналам проверяется вручную на Unix-системах")
	}

	dir := t.TempDir()
	binPath := filepath.Join(dir, "shortener-test")
	storagePath := filepath.Join(dir, "storage.json")

	// Сборка бинарника из корня проекта
	root, err := os.Getwd()
	require.NoError(t, err)
	root = filepath.Dir(root)

	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = root
	out, err := build.CombinedOutput()
	require.NoError(t, err, "сборка бинарника: %s", string(out))

	// Запуск с файловым хранилищем
	cmd := exec.Command(binPath, "-a", "localhost:18080", "-f", storagePath)
	require.NoError(t, cmd.Start())

	// Даём серверу подняться и создаём URL
	baseURL := "http://localhost:18080"
	shortURL := waitServerAndCreateURL(t, baseURL, "https://example.com")
	require.NotEmpty(t, shortURL, "сервер должен вернуть короткий URL")

	// Отправляем SIGTERM — сервер должен завершиться gracefully
	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("процесс не завершился после SIGTERM за 10 секунд")
	}

	// Проверяем, что данные сохранились
	data, err := os.ReadFile(storagePath)
	require.NoError(t, err, "файл хранилища должен существовать после graceful shutdown")
	assert.Contains(t, string(data), "https://example.com",
		"данные должны сохраниться после SIGTERM")
}

// waitServerAndCreateURL ждёт готовности сервера и создаёт короткий URL.
// Возвращает короткий URL из ответа.
func waitServerAndCreateURL(t *testing.T, baseURL, longURL string) string {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Post(baseURL+"/", "text/plain", strings.NewReader(longURL))
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr == nil && resp.StatusCode == http.StatusCreated {
				return string(body)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("сервер %s не поднялся или не создал URL за 10 секунд", baseURL)
	return ""
}

// TestGetSecretKey_FromEnv проверяет приоритет переменной окружения.
func TestGetSecretKey_FromEnv(t *testing.T) {
	t.Setenv("AUTH_SECRET_KEY", "env-secret")

	key, err := getSecretKey()
	require.NoError(t, err)
	assert.Equal(t, "env-secret", key)
}

// TestGetSecretKey_Generated проверяет генерацию случайного ключа.
func TestGetSecretKey_Generated(t *testing.T) {
	t.Setenv("AUTH_SECRET_KEY", "")

	key, err := getSecretKey()
	require.NoError(t, err)
	assert.Len(t, key, 64, "32 байта в hex-кодировке — 64 символа")

	// Повторная генерация даёт другой ключ
	key2, err := getSecretKey()
	require.NoError(t, err)
	assert.NotEqual(t, key, key2, "ключи должны быть случайными")
}

// TestFormatBuildInfo_AllSet проверяет вывод при полностью заданной сборке.
func TestFormatBuildInfo_AllSet(t *testing.T) {
	got := formatBuildInfo("1.0.0", "2025-01-15T10:30:00Z", "abc1234")
	want := "Build version: 1.0.0\nBuild date: 2025-01-15T10:30:00Z\nBuild commit: abc1234"
	assert.Equal(t, want, got)
}

// TestFormatBuildInfo_AllEmpty проверяет подстановку N/A для пустых значений.
func TestFormatBuildInfo_AllEmpty(t *testing.T) {
	got := formatBuildInfo("", "", "")
	want := "Build version: N/A\nBuild date: N/A\nBuild commit: N/A"
	assert.Equal(t, want, got)
}

// TestFormatBuildInfo_PartialSet проверяет частично заданную сборку.
func TestFormatBuildInfo_PartialSet(t *testing.T) {
	got := formatBuildInfo("1.0.0", "", "")
	want := "Build version: 1.0.0\nBuild date: N/A\nBuild commit: N/A"
	assert.Equal(t, want, got)
}

// TestOrNA проверяет подстановку N/A для пустой строки.
func TestOrNA(t *testing.T) {
	assert.Equal(t, "N/A", orNA(""))
	assert.Equal(t, "value", orNA("value"))
}
