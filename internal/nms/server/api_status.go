package server

import (
	"net/http"

	"github.com/ellanetworks/core/version"
	"github.com/gin-gonic/gin"
)

type StatusResponse struct {
	Version string `json:"version"`
}

func GetStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		statusResponse := StatusResponse{
			Version: version.GetVersion(),
		}
		err := writeResponse(c.Writer, statusResponse, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
