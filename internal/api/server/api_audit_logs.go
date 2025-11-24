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

type GetAuditLogsRetentionPolicyResponse struct {
	Days int `json:"days"`
}

type UpdateAuditLogsRetentionPolicyParams struct {
	Days int `json:"days"`
}

type AuditLog struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	IP        string `json:"ip"`
	Details   string `json:"details"`
}

type ListAuditLogsResponse struct {
	Items      []AuditLog `json:"items"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
	TotalCount int        `json:"total_count"`
}

func GetAuditLogRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		policyDays, err := dbInstance.GetRetentionPolicy(ctx, db.CategoryAuditLogs)
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

		if params.Days < 1 {
			writeError(w, http.StatusBadRequest, "retention days must be greater than 0", nil, logger.APILog)
			return
		}

		updatedPolicy := &db.RetentionPolicy{
			Category: db.CategoryAuditLogs,
			Days:     params.Days,
		}

		if err := dbInstance.SetRetentionPolicy(r.Context(), updatedPolicy); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update audit log retention policy", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Audit log retention policy updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateAuditLogRetentionPolicyAction, email, getClientIP(r), fmt.Sprintf("User updated audit log retention policy to %d days", params.Days))
	})
}

func ListAuditLogs(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		ctx := r.Context()

		logs, total, err := dbInstance.ListAuditLogsPage(ctx, page, perPage)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve audit logs", err, logger.APILog)
			return
		}

		items := make([]AuditLog, len(logs))
		for i, log := range logs {
			items[i] = AuditLog{
				ID:        log.ID,
				Timestamp: log.Timestamp,
				Level:     log.Level,
				Actor:     log.Actor,
				Action:    log.Action,
				IP:        log.IP,
				Details:   log.Details,
			}
		}

		response := ListAuditLogsResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}
