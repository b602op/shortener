package main

import (
	"log"
	"net/http"

	"github.com/b602op/shortener/internal/handler"
)

func main() {
	log.Println("1 шаг запуск сервера на порте 8080, http://localhost")
	mux := http.NewServeMux()
	log.Println(mux)
	mux.HandleFunc(`/`, handler.MethodPost)
	mux.HandleFunc(`/{id}`, handler.MethodGet)
	log.Println("3 повесили 2 слушателя-обработчика HTTP запросов")
	err := http.ListenAndServe(`localhost:8080`, mux)
	if err != nil {
		log.Println("ошибка запуска сервера: ", err)
		panic(err)
	}
}
