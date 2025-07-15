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

func writeResponse(w http.ResponseWriter, v any, status int, logger *zap.Logger) {
	type response struct {
		Result any `json:"result,omitempty"`
	}
	resp := response{Result: v}
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
func writeErrorHTTP(w http.ResponseWriter, status int, message string, err error, logger *zap.Logger) {
	logger.Info(message, zap.Error(err))
	type errorResponse struct {
		Error string `json:"error"`
	}
	resp := errorResponse{Error: message}
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
