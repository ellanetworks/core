package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/ellanetworks/core/internal/logger"
)

const (
	UpdateRadioEventRetentionPolicyAction = "update_network_log_retention_policy"
)

type GetRadioEventsRetentionPolicyResponse struct {
	Days int `json:"days"`
}

type UpdateRadioEventsRetentionPolicyParams struct {
	Days int `json:"days"`
}

type RadioEvent struct {
	ID            int    `json:"id"`
	Timestamp     string `json:"timestamp"`
	Protocol      string `json:"protocol"`
	MessageType   string `json:"message_type"`
	Direction     string `json:"direction"`
	LocalAddress  string `json:"local_address"`
	RemoteAddress string `json:"remote_address"`
	Raw           []byte `json:"raw"`
	Details       string `json:"details"`
}

type ListRadioEventsResponse struct {
	Items      []RadioEvent `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

type GetRadioEventResponse struct {
	Raw     []byte           `json:"raw"`
	Decoded ngap.NGAPMessage `json:"decoded"`
}

func isRFC3339(s string) bool {
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		return false
	}

	return true
}

func parseRadioEventFilters(r *http.Request) (*db.RadioEventFilters, error) {
	q := r.URL.Query()
	f := &db.RadioEventFilters{}

	if v := strings.TrimSpace(q.Get("protocol")); v != "" {
		f.Protocol = &v
	}

	if v := strings.TrimSpace(q.Get("direction")); v != "" {
		v = strings.ToLower(v)
		if v != "inbound" && v != "outbound" {
			return f, fmt.Errorf("invalid direction")
		}
		f.Direction = &v
	}

	if v := strings.TrimSpace(q.Get("local_address")); v != "" {
		f.LocalAddress = &v
	}

	if v := strings.TrimSpace(q.Get("remote_address")); v != "" {
		f.RemoteAddress = &v
	}

	if v := strings.TrimSpace(q.Get("message_type")); v != "" {
		f.MessageType = &v
	}

	if v := strings.TrimSpace(q.Get("timestamp_from")); v != "" {
		if !isRFC3339(v) {
			return f, fmt.Errorf("invalid from timestamp")
		}
		f.TimestampFrom = &v
	}

	if v := strings.TrimSpace(q.Get("timestamp_to")); v != "" {
		if !isRFC3339(v) {
			return f, fmt.Errorf("invalid to timestamp")
		}
		f.TimestampTo = &v
	}

	return f, nil
}

func GetRadioEventRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		policyDays, err := dbInstance.GetRetentionPolicy(ctx, db.CategoryRadioLogs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve radio event retention policy", err, logger.APILog)
			return
		}

		response := GetRadioEventsRetentionPolicyResponse{Days: policyDays}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func UpdateRadioEventRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var params UpdateRadioEventsRetentionPolicyParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if params.Days < 1 {
			writeError(w, http.StatusBadRequest, "retention days must be greater than 0", nil, logger.APILog)
			return
		}

		updatedPolicy := &db.RetentionPolicy{
			Category: db.CategoryRadioLogs,
			Days:     params.Days,
		}

		if err := dbInstance.SetRetentionPolicy(r.Context(), updatedPolicy); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update radio event retention policy", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Radio event retention policy updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(r.Context(), UpdateRadioEventRetentionPolicyAction, email, getClientIP(r), fmt.Sprintf("User updated radio event retention policy to %d days", params.Days))
	})
}

func ListRadioEvents(dbInstance *db.Database) http.Handler {
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

		filters, err := parseRadioEventFilters(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		logs, total, err := dbInstance.ListRadioEvents(ctx, page, perPage, filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve radio events", err, logger.APILog)
			return
		}

		items := make([]RadioEvent, len(logs))
		for i, log := range logs {
			items[i] = RadioEvent{
				ID:            log.ID,
				Timestamp:     log.Timestamp,
				Protocol:      log.Protocol,
				MessageType:   log.MessageType,
				Direction:     log.Direction,
				LocalAddress:  log.LocalAddress,
				RemoteAddress: log.RemoteAddress,
				Raw:           log.Raw,
				Details:       log.Details,
			}
		}

		response := ListRadioEventsResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func GetRadioEvent(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		networkIDStr := r.PathValue("id")
		networkID, err := strconv.Atoi(networkIDStr)
		if err != nil || networkID < 1 {
			writeError(w, http.StatusBadRequest, "Invalid radio event ID", err, logger.APILog)
			return
		}

		networkLog, err := dbInstance.GetRadioEventByID(ctx, networkID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve radio event", err, logger.APILog)
			return
		}

		response := GetRadioEventResponse{
			Raw:     networkLog.Raw,
			Decoded: ngap.DecodeNGAPMessage(networkLog.Raw),
		}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func ClearRadioEvents(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		if err := dbInstance.ClearRadioEvents(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to clear radio events", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "All radio events cleared successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(r.Context(), "clear_network_logs", email, getClientIP(r), "User cleared all radio events")
	})
}
