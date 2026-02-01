package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func Handler() http.Handler {
	r := chi.NewRouter()

	r.Post("/", MethodPost)

	r.Get("/{id}", MethodGet)

	return r
}
