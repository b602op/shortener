package main

import (
	"net/http"

	"github.com/b602op/shortener/internal/handler"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc(`/`, handler.MethodPost)
	mux.HandleFunc(`/{id}`, handler.MethodGet)
	err := http.ListenAndServe(`localhost:8080`, mux)
	if err != nil {
		panic(err)
	}
}
