// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

const (
	UpdateFlowReportsRetentionPolicyAction = "update_flow_reports_retention_policy"
	ClearFlowReportsAction                 = "clear_flow_reports"
)

type GetFlowReportsRetentionPolicyResponse struct {
	Days int `json:"days"`
}

type UpdateFlowReportsRetentionPolicyParams struct {
	Days int `json:"days"`
}

type FlowReport struct {
	ID              int    `json:"id"`
	SubscriberID    string `json:"subscriber_id"`
	Timestamp       string `json:"timestamp"`
	SourceIP        string `json:"source_ip"`
	DestinationIP   string `json:"destination_ip"`
	SourcePort      uint16 `json:"source_port"`
	DestinationPort uint16 `json:"destination_port"`
	Protocol        uint8  `json:"protocol"`
	Packets         uint64 `json:"packets"`
	Bytes           uint64 `json:"bytes"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
}

type ListFlowReportsResponse struct {
	Items      []FlowReport `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

// parseFlowReportFilters extracts query parameters and converts them to db.FlowReportFilters
func parseFlowReportFilters(r *http.Request) (*db.FlowReportFilters, error) {
	q := r.URL.Query()
	f := &db.FlowReportFilters{}

	if v := strings.TrimSpace(q.Get("subscriber_id")); v != "" {
		f.SubscriberID = &v
	}

	if v := strings.TrimSpace(q.Get("protocol")); v != "" {
		protoNum, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			return f, fmt.Errorf("invalid protocol number")
		}

		proto := uint8(protoNum)
		f.Protocol = &proto
	}

	if v := strings.TrimSpace(q.Get("source_ip")); v != "" {
		f.SourceIP = &v
	}

	if v := strings.TrimSpace(q.Get("destination_ip")); v != "" {
		f.DestinationIP = &v
	}

	return f, nil
}

// GetFlowReportsRetentionPolicy returns the current retention policy for flow reports
func GetFlowReportsRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		policyDays, err := dbInstance.GetRetentionPolicy(ctx, db.CategoryFlowReports)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to retrieve flow reports retention policy", err, logger.APILog)
			return
		}

		response := GetFlowReportsRetentionPolicyResponse{Days: policyDays}

		writeResponse(ctx, w, response, http.StatusOK, logger.APILog)
	})
}

// UpdateFlowReportsRetentionPolicy updates the retention policy for flow reports
func UpdateFlowReportsRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		email, ok := ctx.Value(contextKeyEmail).(string)
		if !ok {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var params UpdateFlowReportsRetentionPolicyParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(ctx, w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if params.Days < 1 {
			writeError(ctx, w, http.StatusBadRequest, "retention days must be greater than 0", nil, logger.APILog)
			return
		}

		updatedPolicy := &db.RetentionPolicy{
			Category: db.CategoryFlowReports,
			Days:     params.Days,
		}

		if err := dbInstance.SetRetentionPolicy(r.Context(), updatedPolicy); err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to update flow reports retention policy", err, logger.APILog)
			return
		}

		writeResponse(ctx, w, SuccessResponse{Message: "Flow reports retention policy updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(r.Context(), UpdateFlowReportsRetentionPolicyAction, email, getClientIP(r), fmt.Sprintf("User updated flow reports retention policy to %d days", params.Days))
	})
}

// ListFlowReports returns a paginated list of flow reports with optional filtering
func ListFlowReports(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(ctx, w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(ctx, w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		filters, err := parseFlowReportFilters(r)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		reports, total, err := dbInstance.ListFlowReports(ctx, page, perPage, filters)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to retrieve flow reports", err, logger.APILog)
			return
		}

		items := make([]FlowReport, len(reports))
		for i, report := range reports {
			items[i] = FlowReport{
				ID:              report.ID,
				SubscriberID:    report.SubscriberID,
				Timestamp:       report.Timestamp,
				SourceIP:        report.SourceIP,
				DestinationIP:   report.DestinationIP,
				SourcePort:      report.SourcePort,
				DestinationPort: report.DestinationPort,
				Protocol:        report.Protocol,
				Packets:         report.Packets,
				Bytes:           report.Bytes,
				StartTime:       report.StartTime,
				EndTime:         report.EndTime,
			}
		}

		response := ListFlowReportsResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(ctx, w, response, http.StatusOK, logger.APILog)
	})
}

// ClearFlowReports deletes all flow reports from the database
func ClearFlowReports(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		email, ok := ctx.Value(contextKeyEmail).(string)
		if !ok {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		if err := dbInstance.ClearFlowReports(r.Context()); err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to clear flow reports", err, logger.APILog)
			return
		}

		writeResponse(ctx, w, SuccessResponse{Message: "All flow reports cleared successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(ctx, ClearFlowReportsAction, email, getClientIP(r), "User cleared all flow reports")
	})
}
