package server

import (
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/version"
)

type FleetStatus struct {
	Managed    bool   `json:"managed"`
	LastSyncAt string `json:"lastSyncAt,omitempty"`
}

type StatusResponse struct {
	Version     string      `json:"version"`
	Revision    string      `json:"revision"`
	Initialized bool        `json:"initialized"`
	Fleet       FleetStatus `json:"fleet"`
}

func GetStatus(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		numUsers, err := dbInstance.CountUsers(ctx)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Unable to retrieve number of users", err, logger.APILog)
			return
		}

		initialized := numUsers > 0

		var fleetStatus FleetStatus

		fleetData, err := dbInstance.GetFleet(ctx)
		if err != nil {
			logger.APILog.Warn("couldn't check fleet status for status endpoint")
		} else {
			fleetStatus.Managed = len(fleetData.Certificate) > 0 && len(fleetData.CACertificate) > 0
			fleetStatus.LastSyncAt = fleetData.LastSyncAt
		}

		ver := version.GetVersion()

		statusResponse := StatusResponse{
			Version:     ver.Version,
			Revision:    ver.Revision,
			Initialized: initialized,
			Fleet:       fleetStatus,
		}

		writeResponse(r.Context(), w, statusResponse, http.StatusOK, logger.APILog)
	})
}
