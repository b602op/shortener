package main

import (
	"log"
	"net/http"

	"github.com/b602op/shortener/internal/handler"
)

func main() {
	log.Println("1 шаг запуск сервера на порте 8080, http://localhost")

	r := handler.SetupRouter()

	log.Println("Запуск сервера на localhost:8080")
	err := http.ListenAndServe(`localhost:8080`, r)
	if err != nil {
		log.Println("ошибка запуска сервера: ", err)
		panic(err)
	}
}
