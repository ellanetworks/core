package server

import (
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/version"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type StatusResponse struct {
	Version     string `json:"version"`
	Initialized bool   `json:"initialized"`
}

const GetStatusAction = "get_status"

func GetStatus(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		numUsers, err := dbInstance.NumUsers(c.Request.Context())
		if err != nil {
			logger.APILog.Warn("Failed to query number of users", zap.Error(err))
			writeError(c, http.StatusInternalServerError, "Unable to retrieve number of users")
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
		writeResponse(c, statusResponse, http.StatusOK)
		logger.LogAuditEvent(
			GetStatusAction,
			"",
			c.ClientIP(),
			"Successfully retrieved status",
		)
	}
}
