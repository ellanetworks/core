package server

import (
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/version"
	"go.uber.org/zap"
)

type StatusResponse struct {
	Version     string `json:"version"`
	Initialized bool   `json:"initialized"`
}

func GetStatus(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		numUsers, err := dbInstance.NumUsers(ctx)
		if err != nil {
			logger.APILog.Warn("Failed to query number of users", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Unable to retrieve number of users", err, logger.APILog)
			return
		}

		initialized := numUsers > 0
		statusResponse := StatusResponse{
			Version:     version.GetVersion(),
			Initialized: initialized,
		}

		writeResponse(w, statusResponse, http.StatusOK, logger.APILog)
	})
}
