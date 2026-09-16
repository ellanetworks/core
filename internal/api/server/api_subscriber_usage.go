// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ellanetworks/core/etsi"
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

const MaxUsageDays = 11000

type DailySubscriberUsage struct {
	Date          string `json:"date"`
	UplinkBytes   int64  `json:"uplink_bytes"`
	DownlinkBytes int64  `json:"downlink_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
}

type PerSubscriberUsage struct {
	IMSI          string `json:"imsi"`
	UplinkBytes   int64  `json:"uplink_bytes"`
	DownlinkBytes int64  `json:"downlink_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
}

func usagePerDayResponse(usage []db.UsagePerDay, days db.DayRange) []DailySubscriberUsage {
	byDay := make(map[int64]db.UsagePerDay, len(usage))
	for _, u := range usage {
		byDay[u.EpochDay] = u
	}

	response := make([]DailySubscriberUsage, 0, days.Len())

	for offset := int64(0); offset < days.Len(); offset++ {
		u := byDay[days.First+offset]
		response = append(response, DailySubscriberUsage{
			Date:          days.Day(offset).Format("2006-01-02"),
			UplinkBytes:   u.BytesUplink,
			DownlinkBytes: u.BytesDownlink,
			TotalBytes:    u.BytesUplink + u.BytesDownlink,
		})
	}

	return response
}

func GetSubscriberUsage(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		now := time.Now()

		startDate, endDate, err := parseTimeRange(q, now.AddDate(0, 0, -7), now)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		days := db.NewDayRange(startDate, endDate)
		if today := db.DaysSinceEpoch(now); days.Last > today {
			days.Last = today
		}

		groupBy := q.Get("group_by")

		subscriber := q.Get("subscriber")
		if subscriber != "" {
			if _, err := etsi.NewSUPIFromIMSI(subscriber); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "invalid subscriber: must be a valid IMSI of 6 to 15 digits", nil, logger.APILog)
				return
			}
		}

		limit := db.NoUsageLimit

		if raw := q.Get("limit"); raw != "" {
			if groupBy != "subscriber" {
				writeError(r.Context(), w, http.StatusBadRequest, "limit is only supported with group_by=subscriber", nil, logger.APILog)
				return
			}

			parsed, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || parsed < 1 || parsed > MaxNumSubscribers {
				writeError(r.Context(), w, http.StatusBadRequest, fmt.Sprintf("limit must be between 1 and %d", MaxNumSubscribers), nil, logger.APILog)
				return
			}

			limit = parsed
		}

		ctx := r.Context()

		switch groupBy {
		case "day":
			if days.Len() > MaxUsageDays {
				writeError(r.Context(), w, http.StatusBadRequest, fmt.Sprintf("exceeded maximum of %d days per query: narrow the time range", MaxUsageDays), nil, logger.APILog)
				return
			}

			dailyUsage, err := dbInstance.GetUsagePerDay(ctx, subscriber, days)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve subscriber usage", err, logger.APILog)
				return
			}

			response := usagePerDayResponse(dailyUsage, days)

			writeResponse(r.Context(), w, response, http.StatusOK, logger.APILog)

			return
		case "subscriber":
			subscriberUsage, err := dbInstance.GetUsagePerSubscriber(ctx, subscriber, days, limit)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve subscriber usage", err, logger.APILog)
				return
			}

			response := make([]PerSubscriberUsage, len(subscriberUsage))

			for i, usage := range subscriberUsage {
				response[i] = PerSubscriberUsage{
					IMSI:          usage.IMSI,
					UplinkBytes:   usage.BytesUplink,
					DownlinkBytes: usage.BytesDownlink,
					TotalBytes:    usage.BytesUplink + usage.BytesDownlink,
				}
			}

			writeResponse(r.Context(), w, response, http.StatusOK, logger.APILog)

			return
		default:
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid group_by parameter", errors.New("group_by must be either 'day' or 'subscriber'"), logger.APILog)
			return
		}
	})
}

func GetSubscriberUsageRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		policyDays, err := dbInstance.GetRetentionPolicy(ctx, db.CategorySubscriberUsage)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve subscriber usage retention policy", err, logger.APILog)
			return
		}

		response := GetSubscriberUsageRetentionPolicyResponse{Days: policyDays}
		writeResponse(r.Context(), w, response, http.StatusOK, logger.APILog)
	})
}

func UpdateSubscriberUsageRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var params UpdateSubscriberUsageRetentionPolicyParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if params.Days < 1 {
			writeError(r.Context(), w, http.StatusBadRequest, "retention days must be greater than 0", nil, logger.APILog)
			return
		}

		updatedPolicy := &db.RetentionPolicy{
			Category: db.CategorySubscriberUsage,
			Days:     params.Days,
		}

		if err := dbInstance.SetRetentionPolicy(r.Context(), updatedPolicy); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update subscriber usage retention policy", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Subscriber usage retention policy updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(r.Context(), UpdateSubscriberUsageRetentionPolicyAction, email, getClientIP(r), fmt.Sprintf("User updated subscriber usage retention policy to %d days", params.Days))
	})
}

func ClearSubscriberUsage(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		if err := dbInstance.ClearDailyUsage(r.Context()); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to clear subscriber usage", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "All subscriber usage cleared successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(r.Context(), "clear_subscriber_usage", email, getClientIP(r), "User cleared all subscriber usage")
	})
}
