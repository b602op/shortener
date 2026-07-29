package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/handler"
	"github.com/b602op/shortener/internal/repository"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	cfg, err := config.New()

	if err != nil {
		slog.Error("Ошибка конфигурации", "error", err)
		os.Exit(1)
	}

	storage := repository.NewStorage()

	if err := storage.Init(cfg.GetFileStoragePath()); err != nil {
		slog.Warn("Не удалось загрузить данные из файла", "error", err)
	}

	slog.Info("Запуск сервера", "address", cfg.GetServerAddress())
	slog.Info("Базовый URL", "baseURL", cfg.GetBaseURL())
	slog.Info("Путь к файлу хранилища", "file", cfg.GetFileStoragePath())

	r := handler.Handler(cfg)

	server := &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Ошибка запуска сервера", "error", err)
		os.Exit(1)
	}
}
