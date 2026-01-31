package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"

	"github.com/b602op/shortener/internal/repository"
)

func MethodPost(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(res, "Only POST requests are allowed!", http.StatusBadRequest)
		return
	}
	//читаем тело запроса
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(res, "Ошибка чтения тела", http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		http.Error(res, "Пустое тело запроса", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()
	//сокращаем url
	hash := sha256.Sum256([]byte(body))
	shortURL := "http://" + req.Host + "/" + hex.EncodeToString(hash[:4]) // 4 байта хеша = 8 символов в hex
	//формируем заголовок ответа
	res.Header().Set("content-type", "text/plain")
	res.Header().Set("Content-Length", strconv.Itoa(len(shortURL)))
	res.WriteHeader(http.StatusCreated)
	//записываем ответ
	res.Write([]byte(shortURL))
	repository.InsertData(string(body), hex.EncodeToString(hash[:4]))
}
