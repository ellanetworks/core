// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type GetFlowAccountingInfoResponse struct {
	Enabled bool `json:"enabled"`
}

type UpdateFlowAccountingInfoParams struct {
	Enabled bool `json:"enabled"`
}

const (
	UpdateFlowAccountingSettingsAction = "update_flow_accounting_settings"
)

func GetFlowAccountingInfo(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isEnabled, err := dbInstance.IsFlowAccountingEnabled(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Flow accounting info not found", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, GetFlowAccountingInfoResponse{Enabled: isEnabled}, http.StatusOK, logger.APILog)
	})
}

func UpdateFlowAccountingInfo(dbInstance *db.Database, upf UPFUpdater) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)

		email, ok := emailAny.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateFlowAccountingInfoParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := dbInstance.UpdateFlowAccountingSettings(r.Context(), params.Enabled); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update flow accounting settings", err, logger.APILog)
			return
		}

		if err := upf.ReloadFlowAccounting(params.Enabled); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to reload UPF with new flow accounting settings", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Flow accounting settings updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			UpdateFlowAccountingSettingsAction,
			email,
			getClientIP(r),
			fmt.Sprintf("Flow accounting settings updated: enabled=%t", params.Enabled),
		)
	})
}
