package server

import (
	"encoding/json"
	"log"
	"net/http"
)

// writeJSON is a helper function that writes a JSON response to the http.ResponseWriter
func writeJSON(w http.ResponseWriter, v any) error {
	type response struct {
		Result any `json:"result,omitempty"`
	}
	resp := response{Result: v}
	respBytes, err := json.Marshal(&resp)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		return err
	}
	return nil
}

// writeError is a helper function that logs any error and writes it back as an http response
func writeError(w http.ResponseWriter, status int, format string) {
	type errorResponse struct {
		Error string `json:"error"`
	}

	log.Println(format)

	resp := errorResponse{Error: format}
	respBytes, err := json.Marshal(&resp)
	if err != nil {
		log.Printf("Error marshalling error response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	_, err = w.Write(respBytes)
	if err != nil {
		log.Printf("Error writing error response: %v", err)
	}
}
