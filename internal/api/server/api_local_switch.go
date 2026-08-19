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

type GetLocalSwitchInfoResponse struct {
	Enabled bool `json:"enabled"`
}

type UpdateLocalSwitchInfoParams struct {
	Enabled bool `json:"enabled"`
}

const (
	UpdateLocalSwitchSettingsAction = "update_local_switch_settings"
)

func GetLocalSwitchInfo(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isEnabled, err := dbInstance.IsLocalSwitchEnabled(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Local switch info not found", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, GetLocalSwitchInfoResponse{Enabled: isEnabled}, http.StatusOK, logger.APILog)
	})
}

func UpdateLocalSwitchInfo(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)

		email, ok := emailAny.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateLocalSwitchInfoParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := dbInstance.UpdateLocalSwitchSettings(r.Context(), params.Enabled); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update local switch settings", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Local switch settings updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			UpdateLocalSwitchSettingsAction,
			email,
			getClientIP(r),
			fmt.Sprintf("Local switch settings updated: enabled=%t", params.Enabled),
		)
	})
}
