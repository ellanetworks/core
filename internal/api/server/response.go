package server

import (
	"errors"

	"github.com/gin-gonic/gin"
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
	c.Error(errors.New(message)).SetType(gin.ErrorTypePublic)
	c.JSON(status, gin.H{"error": message})
}
