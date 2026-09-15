package worker

import (
	"testing"

	"github.com/b602op/shortener/internal/repository"
	"github.com/stretchr/testify/assert"
)

func TestDeleteService_CloseIsIdempotent(t *testing.T) {
	store := repository.NewFileStorage()
	svc := NewDeleteService(store, DefaultConfig())

	svc.Close()
	svc.Close() // не должно паниковать
}

func TestDeleteService_DeleteAfterClose(t *testing.T) {
	store := repository.NewFileStorage()
	svc := NewDeleteService(store, DefaultConfig())

	svc.Close()

	err := svc.Delete("user", "short")
	assert.ErrorIs(t, err, ErrQueueFull)
}
