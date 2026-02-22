package main

import (
	"log"
	"net/http"
	"time"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/handler"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Ошибка конфигурации: %v", err)
	}

	log.Printf("Запуск сервера на %s", cfg.GetServerAddress())
	log.Printf("Базовый URL: %s", cfg.GetBaseURL())

	r := handler.Handler()

	server := &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Printf("Ошибка запуска сервера: %v", err)
		panic(err)
	}
}
