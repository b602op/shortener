package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/handler"
	"github.com/b602op/shortener/internal/repository"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Ошибка конфигурации: %v", err)
	}

	store := cfg.GetStorage()

	// Сохраняем данные при завершении сервера
	if fileStore, ok := store.(*repository.FileStorage); ok {
		defer fileStore.Close()
	}

	// Если используется DBStorage, закрываем подключение при завершении
	if dbStore, ok := store.(*repository.DBStorage); ok {
		defer dbStore.Close()
	}

	addr := cfg.GetServerAddress()
	baseURL := cfg.GetBaseURL()

	log.Printf("Сервер запускается на %s", addr)
	log.Printf("Базовый URL: %s", baseURL)

	httpHandler := handler.Handler(cfg, store, nil)

	server := &http.Server{
		Addr:    addr,
		Handler: httpHandler,
	}

	// Запуск в горутине
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	// Ожидание сигнала завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Завершение работы сервера...")

	ctx, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Ошибка завершения сервера: %v", err)
	}

	log.Println("Сервер завершил работу")
}
