package server

import (
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/version"
	"github.com/gin-gonic/gin"
)

type StatusResponse struct {
	Version     string `json:"version"`
	Initialized bool   `json:"initialized"`
}

func GetStatus(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		numUsers, err := dbInstance.NumUsers()
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Unable to retrieve number of users")
			return
		}
		var initialized bool
		if numUsers > 0 {
			initialized = true
		} else {
			initialized = false
		}
		statusResponse := StatusResponse{
			Version:     version.GetVersion(),
			Initialized: initialized,
		}
		err = writeResponse(c.Writer, statusResponse, http.StatusOK)
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "internal error")
			return
		}
	}
}
