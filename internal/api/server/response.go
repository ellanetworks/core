package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SuccessResponse struct {
	Message string `json:"message"`
}

type CreateSuccessResponse struct {
	Message string `json:"message"`
	ID      int64  `json:"id"`
}

func writeResponse(c *gin.Context, v any, status int) {
	c.JSON(status, gin.H{"result": v})
}

func writeError(c *gin.Context, status int, message string) {
	_ = c.Error(errors.New(message)).SetType(gin.ErrorTypePublic)
	c.JSON(status, gin.H{"error": message})
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
