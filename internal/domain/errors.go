// Package domain содержит общие ошибки бизнес-логики, независимые от транспорта
// (HTTP, gRPC) и хранилища.
package domain

import "errors"

var (
	// ErrInvalidURL — невалидный исходный URL (пустой, слишком длинный,
	// не http/https или не парсится).
	ErrInvalidURL = errors.New("invalid URL")

	// ErrNotFound — короткая ссылка не найдена в хранилище.
	ErrNotFound = errors.New("not found")

	// ErrDeleted — короткая ссылка найдена, но помечена удалённой.
	ErrDeleted = errors.New("deleted")

	// ErrDuplicateURL — исходный URL уже был сокращён ранее.
	ErrDuplicateURL = errors.New("duplicate URL")
)
