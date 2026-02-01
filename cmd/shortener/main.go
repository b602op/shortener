package main

import (
	"log"
	"net/http"

	"github.com/b602op/shortener/internal/config"
	"github.com/b602op/shortener/internal/handler"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatal("ошибка конфигурации: ", err)
	}

	log.Printf("Конфигурация загружена:")
	log.Printf("  Адрес сервера: %s", cfg.GetServerAddress())
	log.Printf("  Базовый URL: %s", cfg.GetBaseURL())

	r := handler.Handler(cfg)

	log.Printf("Запуск сервера на %s", cfg.GetServerAddress())

	err = http.ListenAndServe(cfg.GetServerAddress(), r)
	if err != nil {
		log.Println("ошибка запуска сервера: ", err)
		panic(err)
	}
}
