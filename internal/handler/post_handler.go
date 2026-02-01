package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/b602op/shortener/internal/repository"
)

func MethodPost(res http.ResponseWriter, req *http.Request) {
	log.Println("Оппа-нифига, иди сюда POST запрос: ", req.RequestURI)

	if req.Method != http.MethodPost {
		http.Error(res, "Only POST requests are allowed!", http.StatusBadRequest)
		return
	}

	defer req.Body.Close()

	body, err := io.ReadAll(req.Body)
	log.Println("Тело запроса: ", string(body))

	if err != nil {
		http.Error(res, "Ошибка чтения тела", http.StatusBadRequest)
		return
	}
	log.Println("Длина тела: ", len(body))
	if len(body) == 0 {
		http.Error(res, "Пустое тело запроса", http.StatusBadRequest)
		return
	}

	//сокращаем url

	hash := sha256.Sum256([]byte(body))
	log.Println("Хеш: ", hex.EncodeToString(hash[:4]))

	shortURL := "http://" + req.Host + "/" + hex.EncodeToString(hash[:4])

	log.Println("Ответ: ", shortURL)

	res.Header().Set("content-type", "text/plain")
	res.Header().Set("Content-Length", strconv.Itoa(len(shortURL)))
	res.WriteHeader(http.StatusCreated)
	res.Write([]byte(shortURL))

	repository.InsertData(string(body), hex.EncodeToString(hash[:4]))
}
