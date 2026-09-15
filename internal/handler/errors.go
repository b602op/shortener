package handler

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse — единый формат JSON-ответа с описанием ошибки.
type ErrorResponse struct {
	Error string `json:"error"`
}

// respondWithError записывает сообщение об ошибке в JSON-виде и устанавливает
// переданный HTTP-статус.
func respondWithError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}
