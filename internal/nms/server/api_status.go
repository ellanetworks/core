package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/version"
)

type StatusResponse struct {
	Version string `json:"version"`
}

func GetStatus(c *gin.Context) {
	statusResponse := StatusResponse{
		Version: version.GetVersion(),
	}
	c.JSON(http.StatusOK, statusResponse)
}
