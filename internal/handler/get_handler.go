package handler

import (
	"net/http"

	"github.com/b602op/shortener/internal/repository"
)

func MethodGet(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(res, "Only Get requests are allowed!", http.StatusBadRequest)
		return
	}

	shortURL := string(req.RequestURI)
	shortURL = shortURL[1:]
	res.Header().Set("Location", repository.SelectData(shortURL))
	res.Header().Set("Content-type", "text/plain")
	res.WriteHeader(http.StatusTemporaryRedirect)
}
