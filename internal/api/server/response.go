package server

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

type SuccessResponse struct {
	Message string `json:"message"`
}

type CreateSuccessResponse struct {
	Message string `json:"message"`
	ID      int64  `json:"id"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type Response struct {
	Result any `json:"result,omitempty"`
}

func writeResponse(w http.ResponseWriter, v any, status int, logger *zap.Logger) {
	resp := Response{Result: v}

	respBytes, err := json.Marshal(&resp)
	if err != nil {
		logger.Error("Error marshalling response", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(status)

	if _, err := w.Write(respBytes); err != nil {
		logger.Error("Error writing response", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)

		return
	}
}

// writeError is a helper function that logs errors and writes http response for errors
func writeError(w http.ResponseWriter, status int, message string, err error, logger *zap.Logger) {
	logger.Debug(message, zap.Error(err))

	resp := ErrorResponse{Error: message}

	respBytes, err := json.Marshal(&resp)
	if err != nil {
		logger.Error("Error marshalling error response", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(status)

	_, err = w.Write(respBytes)
	if err != nil {
		logger.Error("Error writing error response", zap.Error(err))
	}
}
