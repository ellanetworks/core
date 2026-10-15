// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/lmf"
	"github.com/ellanetworks/core/internal/logger"
)

const (
	CreateDeferredLocationRequestAction = "create_deferred_location_request"
	DeleteDeferredLocationRequestAction = "delete_deferred_location_request"
)

const (
	ldrTypePeriodic = "PERIODIC"

	defaultLocationReportsLimit = 100
	maxLocationReportsLimit     = 1000
)

var knownLdrTypes = map[string]bool{
	"UE_AVAILABLE":       true,
	ldrTypePeriodic:      true,
	"ENTERING_INTO_AREA": true,
	"LEAVING_FROM_AREA":  true,
	"BEING_INSIDE_AREA":  true,
	"MOTION":             true,
}

type PeriodicEventInfo struct {
	ReportingAmount      int64  `json:"reporting_amount,omitempty"`
	ReportingInterval    int64  `json:"reporting_interval"`
	ReportingInfiniteInd bool   `json:"reporting_infinite_ind,omitempty"`
	ReportingIntervalMs  *int64 `json:"reporting_interval_ms,omitempty"`
}

type CreateDeferredLocationRequestParams struct {
	SUPI                  string                 `json:"supi"`
	LdrType               string                 `json:"ldr_type"`
	PeriodicEventInfo     PeriodicEventInfo      `json:"periodic_event_info"`
	ScheduledLocTime      *string                `json:"scheduled_loc_time,omitempty"`
	Method                string                 `json:"method,omitempty"`
	Mode                  string                 `json:"mode,omitempty"`
	MeasurementQuantities *MeasurementQuantities `json:"measurement_quantities,omitempty"`
}

type DeferredLocationRequest struct {
	ID                    string                 `json:"id"`
	SUPI                  string                 `json:"supi"`
	LdrType               string                 `json:"ldr_type"`
	PeriodicEventInfo     PeriodicEventInfo      `json:"periodic_event_info"`
	ScheduledLocTime      *string                `json:"scheduled_loc_time,omitempty"`
	Method                string                 `json:"method"`
	Mode                  string                 `json:"mode,omitempty"`
	MeasurementQuantities *MeasurementQuantities `json:"measurement_quantities,omitempty"`
	Status                string                 `json:"status"`
	TerminationCause      string                 `json:"termination_cause,omitempty"`
	TerminationReason     string                 `json:"termination_reason,omitempty"`
	EventReportCounter    int64                  `json:"event_report_counter"`
	CreatedAt             string                 `json:"created_at"`
	TerminatedAt          *string                `json:"terminated_at,omitempty"`
}

type ListDeferredLocationRequestsResponse struct {
	Items      []DeferredLocationRequest `json:"items"`
	Page       int                       `json:"page"`
	PerPage    int                       `json:"per_page"`
	TotalCount int                       `json:"total_count"`
}

type LocationReport struct {
	Seq          int64        `json:"seq"`
	ReportedAt   string       `json:"reported_at"`
	LocationData LocationData `json:"location_data"`
}

type ListLocationReportsResponse struct {
	Items []LocationReport `json:"items"`
	Next  int64            `json:"next"`
}

func CreateDeferredLocationRequest(lmfInstance *lmf.LMF) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var params CreateDeferredLocationRequestParams
		if err := decodeStrict(r, &params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		supi, msg := parseSUPI(params.SUPI)
		if msg != "" {
			writeError(r.Context(), w, http.StatusBadRequest, msg, nil, logger.APILog)
			return
		}

		switch {
		case params.LdrType == "":
			writeError(r.Context(), w, http.StatusBadRequest, "ldr_type is required", nil, logger.APILog)
			return
		case !knownLdrTypes[params.LdrType]:
			writeError(r.Context(), w, http.StatusBadRequest, fmt.Sprintf("invalid ldr_type: %s", params.LdrType), nil, logger.APILog)
			return
		case params.LdrType != ldrTypePeriodic:
			writeError(r.Context(), w, http.StatusNotImplemented, fmt.Sprintf("unsupported ldr_type: %s", params.LdrType), nil, logger.APILog)
			return
		case params.PeriodicEventInfo == (PeriodicEventInfo{}):
			writeError(r.Context(), w, http.StatusBadRequest, "periodic_event_info is required when ldr_type is PERIODIC", nil, logger.APILog)
			return
		}

		locationRequest, msg := parseLocationRequest(params.Method, params.Mode, params.MeasurementQuantities)
		if msg != "" {
			writeError(r.Context(), w, http.StatusBadRequest, msg, nil, logger.APILog)
			return
		}

		var scheduled *time.Time

		if params.ScheduledLocTime != nil {
			t, err := time.Parse(time.RFC3339, *params.ScheduledLocTime)
			if err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "scheduled_loc_time must be an RFC 3339 date-time", err, logger.APILog)
				return
			}

			scheduled = &t
		}

		info := params.PeriodicEventInfo

		created, err := lmfInstance.CreateDeferredRequest(r.Context(), lmf.DeferredRequestParams{
			SUPI:     supi,
			Location: locationRequest,
			PeriodicEventInfo: lmf.PeriodicEventInfo{
				ReportingAmount:     info.ReportingAmount,
				ReportingInterval:   info.ReportingInterval,
				ReportingInfinite:   info.ReportingInfiniteInd,
				ReportingIntervalMs: info.ReportingIntervalMs,
			},
			ScheduledLocTime: scheduled,
		})
		if err != nil {
			writeDeferredLocationError(r, w, err)
			return
		}

		email, _ := r.Context().Value(contextKeyEmail).(string)

		logger.LogAuditEvent(
			r.Context(),
			CreateDeferredLocationRequestAction,
			email,
			getClientIP(r),
			fmt.Sprintf("User created periodic location request %s for SUPI %s (method=%s, interval=%ds, amount=%d)",
				created.ID, created.SUPI, created.Location.Method, created.PeriodicEventInfo.ReportingInterval, created.PeriodicEventInfo.ReportingAmount),
		)

		writeResponse(r.Context(), w, toDeferredLocationRequest(created), http.StatusCreated, logger.APILog)
	})
}

func ListDeferredLocationRequests(lmfInstance *lmf.LMF) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(r.Context(), w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(r.Context(), w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		var supi *string

		if v := q.Get("supi"); v != "" {
			supi = &v
		}

		var status *lmf.DeferredStatus

		if v := q.Get("status"); v != "" {
			s := lmf.DeferredStatus(v)
			if s != lmf.DeferredStatusActive && s != lmf.DeferredStatusTerminated {
				writeError(r.Context(), w, http.StatusBadRequest, "status must be ACTIVE or TERMINATED", nil, logger.APILog)
				return
			}

			status = &s
		}

		requests, total, err := lmfInstance.ListDeferredRequests(r.Context(), page, perPage, supi, status)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list deferred location requests", err, logger.APILog)
			return
		}

		items := make([]DeferredLocationRequest, 0, len(requests))
		for i := range requests {
			items = append(items, toDeferredLocationRequest(&requests[i]))
		}

		writeResponse(r.Context(), w, ListDeferredLocationRequestsResponse{Items: items, Page: page, PerPage: perPage, TotalCount: total}, http.StatusOK, logger.APILog)
	})
}

func GetDeferredLocationRequest(lmfInstance *lmf.LMF) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := lmfInstance.GetDeferredRequest(r.Context(), r.PathValue("id"))
		if err != nil {
			writeDeferredLocationError(r, w, err)
			return
		}

		writeResponse(r.Context(), w, toDeferredLocationRequest(request), http.StatusOK, logger.APILog)
	})
}

func DeleteDeferredLocationRequest(lmfInstance *lmf.LMF) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		if err := lmfInstance.DeleteDeferredRequest(r.Context(), id); err != nil {
			writeDeferredLocationError(r, w, err)
			return
		}

		email, _ := r.Context().Value(contextKeyEmail).(string)

		logger.LogAuditEvent(
			r.Context(),
			DeleteDeferredLocationRequestAction,
			email,
			getClientIP(r),
			"User deleted deferred location request "+id,
		)

		w.WriteHeader(http.StatusNoContent)
	})
}

func ListLocationReports(lmfInstance *lmf.LMF) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		after := int64(0)

		if v := q.Get("after"); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n < 0 {
				writeError(r.Context(), w, http.StatusBadRequest, "after must be a non-negative integer", nil, logger.APILog)
				return
			}

			after = n
		}

		limit := atoiDefault(q.Get("limit"), defaultLocationReportsLimit)
		if limit < 1 || limit > maxLocationReportsLimit {
			writeError(r.Context(), w, http.StatusBadRequest, fmt.Sprintf("limit must be between 1 and %d", maxLocationReportsLimit), nil, logger.APILog)
			return
		}

		reports, err := lmfInstance.ListLocationReports(r.Context(), r.PathValue("id"), after, limit)
		if err != nil {
			writeDeferredLocationError(r, w, err)
			return
		}

		response := ListLocationReportsResponse{Items: make([]LocationReport, 0, len(reports)), Next: after}

		for _, report := range reports {
			response.Items = append(response.Items, LocationReport{
				Seq:          report.Seq,
				ReportedAt:   report.ReportedAt.UTC().Format(time.RFC3339Nano),
				LocationData: *toLocationData(report.Result),
			})
			response.Next = report.Seq
		}

		writeResponse(r.Context(), w, response, http.StatusOK, logger.APILog)
	})
}

func toDeferredLocationRequest(r *lmf.DeferredRequest) DeferredLocationRequest {
	out := DeferredLocationRequest{
		ID:      r.ID,
		SUPI:    r.SUPI,
		LdrType: ldrTypePeriodic,
		PeriodicEventInfo: PeriodicEventInfo{
			ReportingAmount:      r.PeriodicEventInfo.ReportingAmount,
			ReportingInterval:    r.PeriodicEventInfo.ReportingInterval,
			ReportingInfiniteInd: r.PeriodicEventInfo.ReportingInfinite,
		},
		Method:                string(r.Location.Method),
		Mode:                  string(r.Location.Mode),
		MeasurementQuantities: fromQuantitiesModel(r.Location.MeasurementQuantities),
		Status:                string(r.Status),
		TerminationCause:      string(r.TerminationCause),
		TerminationReason:     string(r.TerminationReason),
		EventReportCounter:    r.EventReportCounter,
		CreatedAt:             r.CreatedAt.UTC().Format(time.RFC3339Nano),
	}

	if r.ScheduledLocTime != nil {
		s := r.ScheduledLocTime.UTC().Format(time.RFC3339Nano)
		out.ScheduledLocTime = &s
	}

	if r.TerminatedAt != nil {
		s := r.TerminatedAt.UTC().Format(time.RFC3339Nano)
		out.TerminatedAt = &s
	}

	return out
}

func writeDeferredLocationError(r *http.Request, w http.ResponseWriter, err error) {
	ctx := r.Context()

	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(ctx, w, http.StatusNotFound, "Deferred location request not found", err, logger.APILog)
	case errors.Is(err, lmf.ErrNotFound):
		writeError(ctx, w, http.StatusNotFound, "UE not found or not registered", err, logger.APILog)
	case errors.Is(err, lmf.ErrInvalidPeriodicEventInfo), errors.Is(err, lmf.ErrInvalidScheduledLocTime):
		writeError(ctx, w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
	case errors.Is(err, lmf.ErrDeferredRequestExists):
		writeError(ctx, w, http.StatusConflict, err.Error(), nil, logger.APILog)
	case errors.Is(err, lmf.ErrDeferredNotRunning), errors.Is(err, db.ErrMigrationPending):
		writeError(ctx, w, http.StatusServiceUnavailable, "Deferred location requests are unavailable", err, logger.APILog)
	default:
		writeError(ctx, w, http.StatusInternalServerError, "Deferred location request failed", err, logger.APILog)
	}
}
