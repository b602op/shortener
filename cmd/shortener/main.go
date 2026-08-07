package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/handler"
	"github.com/b602op/shortener/internal/repository"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	cfg, err := config.New()

	if err != nil {
		slog.Error("Ошибка конфигурации", "error", err)
		os.Exit(1)
	}

	storage := repository.NewStorage()

	var db *sql.DB
	if dsn := cfg.GetDatabaseDSN(); dsn != "" {
		db, err = sql.Open("pgx", dsn)
		if err != nil {
			slog.Error("Ошибка открытия соединения с БД", "error", err)
			os.Exit(1)
		}
		defer db.Close()

		// Проверяем, что БД доступна
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			slog.Error("БД не отвечает", "error", err)
			os.Exit(1)
		}

		slog.Info("Подключение к PostgreSQL установлено")
	}

	if err := storage.Init(cfg.GetFileStoragePath()); err != nil {
		slog.Warn("Не удалось загрузить данные из файла", "error", err)
	}

	slog.Info("Запуск сервера", "address", cfg.GetServerAddress())
	slog.Info("Базовый URL", "baseURL", cfg.GetBaseURL())
	slog.Info("Путь к файлу хранилища", "file", cfg.GetFileStoragePath())

	r := handler.Handler(cfg, storage, db)

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
