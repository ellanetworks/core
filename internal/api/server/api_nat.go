// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type GetNATInfoResponse struct {
	Enabled bool `json:"enabled"`
}

type UpdateNATInfoParams struct {
	Enabled bool `json:"enabled"`
}

const (
	UpdateNATSettingsAction = "update_nat_settings"
)

func GetNATInfo(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isNATEnabled, err := dbInstance.IsNATEnabled(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "NAT info not found", err, logger.APILog)
			return
		}

		routeResponse := GetNATInfoResponse{
			Enabled: isNATEnabled,
		}

		writeResponse(r.Context(), w, routeResponse, http.StatusOK, logger.APILog)
	})
}

func UpdateNATInfo(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)

		email, ok := emailAny.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateNATInfoParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		// Framed routes have no function under NAT, so the exclusion holds both
		// ways: enabling NAT is rejected while any framed route exists.
		if params.Enabled {
			framed, err := dbInstance.ListAllFramedRoutes(r.Context())
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check framed routes", err, logger.APILog)
				return
			}

			if len(framed) > 0 {
				writeError(r.Context(), w, http.StatusConflict, "NAT cannot be enabled while framed routes exist", nil, logger.APILog)
				return
			}
		}

		if err := dbInstance.UpdateNATSettings(r.Context(), params.Enabled); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update NAT settings", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "NAT settings updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			UpdateNATSettingsAction,
			email,
			getClientIP(r),
			fmt.Sprintf("NAT settings updated: enabled=%t", params.Enabled),
		)
	})
}
