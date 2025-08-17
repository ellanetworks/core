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

type GetSubscriberLogResponse struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	IMSI      string `json:"imsi"`
	Event     string `json:"event"`
	Details   string `json:"details"`
}

type GetSubscriberLogsRetentionPolicyResponse struct {
	Days int `json:"days"`
}

type UpdateSubscriberLogsRetentionPolicyParams struct {
	Days int `json:"days"`
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
		ctx := r.Context()
		logs, err := dbInstance.ListSubscriberLogs(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve subscriber logs", err, logger.APILog)
			return
		}

		response := make([]GetSubscriberLogResponse, len(logs))
		for i, log := range logs {
			response[i] = GetSubscriberLogResponse{
				ID:        log.ID,
				Timestamp: log.Timestamp,
				Level:     log.Level,
				IMSI:      log.IMSI,
				Event:     log.Event,
				Details:   log.Details,
			}
		}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}
