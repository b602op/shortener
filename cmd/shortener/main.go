package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/handler"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	cfg, err := config.New()
	if err != nil {
		slog.Error("Ошибка конфигурации", "error", err)
		os.Exit(1)
	}

	slog.Info("Запуск сервера", "address", cfg.GetServerAddress())
	slog.Info("Базовый URL", "baseURL", cfg.GetBaseURL())

	r := handler.Handler()

	server := &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		panic(fmt.Sprintf("Ошибка запуска сервера: %v", err))
	}
}
