package server

import (
	"encoding/json"
	"net/http"

	"github.com/ellanetworks/core/internal/logger"
)

type SuccessResponse struct {
	Message string `json:"message"`
}

type CreateSuccessResponse struct {
	Message string `json:"message"`
	ID      int64  `json:"id"`
}

// writeResponse is a helper function that writes a JSON response to the http.ResponseWriter
func writeResponse(w http.ResponseWriter, v any, status int) error {
	type response struct {
		Result any `json:"result,omitempty"`
	}
	resp := response{Result: v}
	respBytes, err := json.Marshal(&resp)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(respBytes); err != nil {
		return err
	}
	return nil
}

// writeError is a helper function that logs any error and writes it back as an http response
func writeError(w http.ResponseWriter, status int, message string) {
	type errorResponse struct {
		Error string `json:"error"`
	}
	logger.APILog.Infof("API Error: %s", message)
	resp := errorResponse{Error: message}
	respBytes, err := json.Marshal(&resp)
	if err != nil {
		logger.APILog.Warnf("Error marshalling error response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(respBytes)
	if err != nil {
		logger.APILog.Warnf("Error writing error response: %v", err)
	}
}
