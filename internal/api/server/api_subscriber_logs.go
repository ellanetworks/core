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
	UpdateSubscriberLogRetentionPolicyAction = "update_subscriber_log_retention_policy"
)

type GetSubscriberLogsRetentionPolicyResponse struct {
	Days int `json:"days"`
}

type UpdateSubscriberLogsRetentionPolicyParams struct {
	Days int `json:"days"`
}

type SubscriberLog struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	IMSI      string `json:"imsi"`
	Event     string `json:"event"`
	Direction string `json:"direction"`
	Details   string `json:"details"`
}

type ListSubscriberLogsResponse struct {
	Items      []SubscriberLog `json:"items"`
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
	TotalCount int             `json:"total_count"`
}

func GetSubscriberLogRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		policyDays, err := dbInstance.GetLogRetentionPolicy(ctx, db.CategorySubscriberLogs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve subscriber log retention policy", err, logger.APILog)
			return
		}

		response := GetSubscriberLogsRetentionPolicyResponse{Days: policyDays}
		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func UpdateSubscriberLogRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var params UpdateSubscriberLogsRetentionPolicyParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if params.Days < 1 {
			writeError(w, http.StatusBadRequest, "retention days must be greater than 0", nil, logger.APILog)
			return
		}

		updatedPolicy := &db.LogRetentionPolicy{
			Category: db.CategorySubscriberLogs,
			Days:     params.Days,
		}

		if err := dbInstance.SetLogRetentionPolicy(r.Context(), updatedPolicy); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update subscriber log retention policy", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Subscriber log retention policy updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateSubscriberLogRetentionPolicyAction, email, getClientIP(r), fmt.Sprintf("User updated subscriber log retention policy to %d days", params.Days))
	})
}

func ListSubscriberLogs(dbInstance *db.Database) http.Handler {
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

		logs, total, err := dbInstance.ListSubscriberLogsPage(ctx, page, perPage)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve subscriber logs", err, logger.APILog)
			return
		}

		items := make([]SubscriberLog, len(logs))
		for i, log := range logs {
			items[i] = SubscriberLog{
				ID:        log.ID,
				Timestamp: log.Timestamp,
				Level:     log.Level,
				IMSI:      log.IMSI,
				Event:     log.Event,
				Direction: log.Direction,
				Details:   log.Details,
			}
		}

		response := ListSubscriberLogsResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func ClearSubscriberLogs(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		if err := dbInstance.ClearSubscriberLogs(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to clear subscriber logs", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "All subscriber logs cleared successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent("clear_subscriber_logs", email, getClientIP(r), "User cleared all subscriber logs")
	})
}
