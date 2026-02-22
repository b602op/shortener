package handler

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse представляет структуру JSON-ответа с ошибкой
type ErrorResponse struct {
	Error string `json:"error"`
}

// respondWithError отправляет ошибку в формате JSON
func respondWithError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}