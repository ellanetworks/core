package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

const (
	UpdateAuditLogRetentionPolicyAction = "update_audit_log_retention_policy"
)

type GetAuditLogResponse struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	IP        string `json:"ip"`
	Details   string `json:"details"`
}

type GetAuditLogsRetentionPolicyResponse struct {
	Days int `json:"days"`
}

type UpdateAuditLogsRetentionPolicyParams struct {
	Days int `json:"days"`
}

func GetAuditLogRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		policyDays, err := dbInstance.GetLogRetentionPolicy(ctx, db.CategoryAuditLogs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve audit log retention policy", err, logger.APILog)
			return
		}

		response := GetAuditLogsRetentionPolicyResponse{Days: policyDays}
		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func UpdateAuditLogRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var params UpdateAuditLogsRetentionPolicyParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if params.Days < 0 {
			writeError(w, http.StatusBadRequest, "Retention days must be non-negative", nil, logger.APILog)
			return
		}

		updatedPolicy := &db.LogRetentionPolicy{
			Category: db.CategoryAuditLogs,
			Days:     params.Days,
		}

		if err := dbInstance.SetLogRetentionPolicy(r.Context(), updatedPolicy); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update audit log retention policy", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Audit log retention policy updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateAuditLogRetentionPolicyAction, email, getClientIP(r), fmt.Sprintf("Retrieved audit log retention policy: %d days", params.Days))
	})
}

func ListAuditLogs(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logs, err := dbInstance.ListAuditLogs(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve audit logs", err, logger.APILog)
			return
		}

		response := make([]GetAuditLogResponse, len(logs))
		for i, log := range logs {
			response[i] = GetAuditLogResponse{
				ID:        log.ID,
				Timestamp: log.Timestamp,
				Level:     log.Level,
				Actor:     log.Actor,
				Action:    log.Action,
				IP:        log.IP,
				Details:   log.Details,
			}
		}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}
