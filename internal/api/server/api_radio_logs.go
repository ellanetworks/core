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
	UpdateRadioLogRetentionPolicyAction = "update_radio_log_retention_policy"
)

type GetRadioLogsRetentionPolicyResponse struct {
	Days int `json:"days"`
}

type UpdateRadioLogsRetentionPolicyParams struct {
	Days int `json:"days"`
}

type RadioLog struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	RanID     string `json:"ran_id"`
	Event     string `json:"event"`
	Direction string `json:"direction"`
	Details   string `json:"details"`
}

type ListRadioLogsResponse struct {
	Items      []RadioLog `json:"items"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
	TotalCount int        `json:"total_count"`
}

func GetRadioLogRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		policyDays, err := dbInstance.GetLogRetentionPolicy(ctx, db.CategoryRadioLogs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve radio log retention policy", err, logger.APILog)
			return
		}

		response := GetRadioLogsRetentionPolicyResponse{Days: policyDays}
		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func UpdateRadioLogRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var params UpdateRadioLogsRetentionPolicyParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if params.Days < 1 {
			writeError(w, http.StatusBadRequest, "retention days must be greater than 0", nil, logger.APILog)
			return
		}

		updatedPolicy := &db.LogRetentionPolicy{
			Category: db.CategoryRadioLogs,
			Days:     params.Days,
		}

		if err := dbInstance.SetLogRetentionPolicy(r.Context(), updatedPolicy); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update radio log retention policy", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Radio log retention policy updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateRadioLogRetentionPolicyAction, email, getClientIP(r), fmt.Sprintf("User updated radio log retention policy to %d days", params.Days))
	})
}

func ListRadioLogs(dbInstance *db.Database) http.Handler {
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

		logs, total, err := dbInstance.ListRadioLogsPage(ctx, page, perPage)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve radio logs", err, logger.APILog)
			return
		}

		items := make([]RadioLog, len(logs))
		for i, log := range logs {
			items[i] = RadioLog{
				ID:        log.ID,
				Timestamp: log.Timestamp,
				Level:     log.Level,
				RanID:     log.RanID,
				Event:     log.Event,
				Direction: log.Direction,
				Details:   log.Details,
			}
		}

		response := ListRadioLogsResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func ClearRadioLogs(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		if err := dbInstance.ClearRadioLogs(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to clear radio logs", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "All radio logs cleared successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent("clear_radio_logs", email, getClientIP(r), "User cleared all radio logs")
	})
}
