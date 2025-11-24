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
	UpdateSubscriberUsageRetentionPolicyAction = "update_subscriber_usage_retention_policy"
	ClearSubscriberUsageAction                 = "clear_subscriber_usage"
)

type GetSubscriberUsageRetentionPolicyResponse struct {
	Days int `json:"days"`
}

type UpdateSubscriberUsageRetentionPolicyParams struct {
	Days int `json:"days"`
}

func GetSubscriberUsageRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		policyDays, err := dbInstance.GetRetentionPolicy(ctx, db.CategorySubscriberUsage)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve subscriber usage retention policy", err, logger.APILog)
			return
		}

		response := GetSubscriberUsageRetentionPolicyResponse{Days: policyDays}
		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func UpdateSubscriberUsageRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var params UpdateSubscriberUsageRetentionPolicyParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if params.Days < 1 {
			writeError(w, http.StatusBadRequest, "retention days must be greater than 0", nil, logger.APILog)
			return
		}

		updatedPolicy := &db.RetentionPolicy{
			Category: db.CategorySubscriberUsage,
			Days:     params.Days,
		}

		if err := dbInstance.SetRetentionPolicy(r.Context(), updatedPolicy); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update subscriber usage retention policy", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Subscriber usage retention policy updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateSubscriberUsageRetentionPolicyAction, email, getClientIP(r), fmt.Sprintf("User updated subscriber usage retention policy to %d days", params.Days))
	})
}

func ClearSubscriberUsage(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		if err := dbInstance.ClearDailyUsage(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to clear subscriber usage", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "All subscriber usage cleared successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent("clear_subscriber_usage", email, getClientIP(r), "User cleared all subscriber usage")
	})
}
