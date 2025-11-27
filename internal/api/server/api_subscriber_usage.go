package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

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

type SubscriberUsage struct {
	UplinkBytes   int64 `json:"uplink_bytes"`
	DownlinkBytes int64 `json:"downlink_bytes"`
	TotalBytes    int64 `json:"total_bytes"`
}

func stotimeDefault(s string, def time.Time) time.Time {
	if s == "" {
		return def
	}

	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return def
	}

	return t
}

func GetSubscriberUsage(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		startDate := stotimeDefault(q.Get("start"), time.Now().AddDate(0, 0, -7))
		endDate := stotimeDefault(q.Get("end"), time.Now())
		groupBy := q.Get("group_by")

		subscriber := q.Get("subscriber")

		ctx := r.Context()

		switch groupBy {
		case "day":
			dailyUsage, err := dbInstance.GetUsagePerDay(ctx, subscriber, startDate, endDate)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to retrieve subscriber usage", err, logger.APILog)
				return
			}

			response := make([]map[string]SubscriberUsage, len(dailyUsage))

			for i, usage := range dailyUsage {
				response[i] = map[string]SubscriberUsage{
					usage.GetDay().Format("2006-01-02"): {
						UplinkBytes:   usage.BytesUplink,
						DownlinkBytes: usage.BytesDownlink,
						TotalBytes:    usage.BytesUplink + usage.BytesDownlink,
					},
				}
			}

			writeResponse(w, response, http.StatusOK, logger.APILog)
			return
		case "subscriber":
			subscriberUsage, err := dbInstance.GetUsagePerSubscriber(ctx, subscriber, startDate, endDate)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to retrieve subscriber usage", err, logger.APILog)
				return
			}

			response := make([]map[string]SubscriberUsage, len(subscriberUsage))

			for i, usage := range subscriberUsage {
				response[i] = map[string]SubscriberUsage{
					usage.IMSI: {
						UplinkBytes:   usage.BytesUplink,
						DownlinkBytes: usage.BytesDownlink,
						TotalBytes:    usage.BytesUplink + usage.BytesDownlink,
					},
				}
			}

			writeResponse(w, response, http.StatusOK, logger.APILog)
			return
		default:
			writeError(w, http.StatusBadRequest, "Invalid group_by parameter", errors.New("group_by must be either 'day' or 'subscriber'"), logger.APILog)
			return
		}
	})
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
