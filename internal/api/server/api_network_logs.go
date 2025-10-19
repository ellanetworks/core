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
	UpdateNetworkLogRetentionPolicyAction = "update_network_log_retention_policy"
)

type GetNetworkLogsRetentionPolicyResponse struct {
	Days int `json:"days"`
}

type UpdateNetworkLogsRetentionPolicyParams struct {
	Days int `json:"days"`
}

type NetworkLog struct {
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

type ListNetworkLogsResponse struct {
	Items      []NetworkLog `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

type GetNetworkLogResponse struct {
	Raw     []byte            `json:"raw"`
	Decoded *ngap.NGAPMessage `json:"decoded"`
}

func isRFC3339(s string) bool {
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		return false
	}

	return true
}

func parseNetworkLogFilters(r *http.Request) (*db.NetworkLogFilters, error) {
	q := r.URL.Query()
	f := &db.NetworkLogFilters{}

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

func GetNetworkLogRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		policyDays, err := dbInstance.GetLogRetentionPolicy(ctx, db.CategoryNetworkLogs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve network log retention policy", err, logger.APILog)
			return
		}

		response := GetNetworkLogsRetentionPolicyResponse{Days: policyDays}
		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func UpdateNetworkLogRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var params UpdateNetworkLogsRetentionPolicyParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if params.Days < 1 {
			writeError(w, http.StatusBadRequest, "retention days must be greater than 0", nil, logger.APILog)
			return
		}

		updatedPolicy := &db.LogRetentionPolicy{
			Category: db.CategoryNetworkLogs,
			Days:     params.Days,
		}

		if err := dbInstance.SetLogRetentionPolicy(r.Context(), updatedPolicy); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update network log retention policy", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Network log retention policy updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateNetworkLogRetentionPolicyAction, email, getClientIP(r), fmt.Sprintf("User updated network log retention policy to %d days", params.Days))
	})
}

func ListNetworkLogs(dbInstance *db.Database) http.Handler {
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

		filters, err := parseNetworkLogFilters(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		logs, total, err := dbInstance.ListNetworkLogs(ctx, page, perPage, filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve network logs", err, logger.APILog)
			return
		}

		items := make([]NetworkLog, len(logs))
		for i, log := range logs {
			items[i] = NetworkLog{
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

		response := ListNetworkLogsResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func GetNetworkLog(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		networkIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/logs/network/")
		networkID, err := strconv.Atoi(networkIDStr)
		if err != nil || networkID < 1 {
			writeError(w, http.StatusBadRequest, "Invalid network log ID", err, logger.APILog)
			return
		}

		networkLog, err := dbInstance.GetNetworkLogByID(ctx, networkID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve network log", err, logger.APILog)
			return
		}

		decodedContent, err := ngap.DecodeNGAPMessage(networkLog.Raw)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to decode network log", err, logger.APILog)
			return
		}

		response := GetNetworkLogResponse{
			Raw:     networkLog.Raw,
			Decoded: decodedContent,
		}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func ClearNetworkLogs(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		if err := dbInstance.ClearNetworkLogs(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to clear network logs", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "All network logs cleared successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent("clear_network_logs", email, getClientIP(r), "User cleared all network logs")
	})
}
