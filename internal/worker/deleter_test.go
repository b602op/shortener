package worker

import (
	"testing"

	"github.com/b602op/shortener/internal/repository"
	"github.com/stretchr/testify/assert"
)

func TestDeleteService_CloseIsIdempotent(t *testing.T) {
	store := repository.NewFileStorage()
	svc := NewDeleteService(store, DefaultConfig())

	_ = svc.Close()
	_ = svc.Close() // не должно паниковать
}

func TestDeleteService_DeleteAfterClose(t *testing.T) {
	store := repository.NewFileStorage()
	svc := NewDeleteService(store, DefaultConfig())

	_ = svc.Close()

	err := svc.Delete("user", "short")
	assert.ErrorIs(t, err, ErrQueueFull)
}

func TestDeleteService_DeleteBatch(t *testing.T) {
	store := repository.NewFileStorage()
	svc := NewDeleteService(store, DefaultConfig())
	defer func() { _ = svc.Close() }()

	// Пакетное удаление принимает все ссылки без ошибки
	err := svc.DeleteBatch("user", []string{"short1", "short2", "short3"})
	assert.NoError(t, err)

	// Пустой пакет — тоже без ошибки
	err = svc.DeleteBatch("user", nil)
	assert.NoError(t, err)

	// После закрытия очереди пакетное удаление возвращает ошибку
	_ = svc.Close()
	err = svc.DeleteBatch("user", []string{"short4"})
	assert.ErrorIs(t, err, ErrQueueFull)
}

// TestDeleteTask_Reset проверяет сгенерированный метод Reset для DeleteTask.
func TestDeleteTask_Reset(t *testing.T) {
	task := DeleteTask{UserID: "user-1", ShortURL: "abc123"}

	task.Reset()
	assert.Equal(t, DeleteTask{}, task, "Reset должен обнулить все поля задачи")

	// nil-ресивер не паникует
	var nilTask *DeleteTask
	nilTask.Reset()
}
