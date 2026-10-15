// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/lmf"
)

func TestCreateDeferredLocationRequestValidation(t *testing.T) {
	handler := CreateDeferredLocationRequest(lmf.New(nil, nil, nil))

	for _, tc := range []struct {
		body    string
		status  int
		message string
	}{
		{`{"ldr_type":"PERIODIC","periodic_event_info":{"reporting_amount":2,"reporting_interval":30}}`, http.StatusBadRequest, "supi is required"},
		{`{"supi":"imsi-001010000000001","periodic_event_info":{"reporting_amount":2,"reporting_interval":30}}`, http.StatusBadRequest, "ldr_type is required"},
		{`{"supi":"imsi-001010000000001","ldr_type":"SOMETIMES"}`, http.StatusBadRequest, "invalid ldr_type"},
		{`{"supi":"imsi-001010000000001","ldr_type":"MOTION"}`, http.StatusNotImplemented, "unsupported ldr_type: MOTION"},
		{`{"supi":"imsi-001010000000001","ldr_type":"PERIODIC"}`, http.StatusBadRequest, "periodic_event_info is required"},
		{`{"supi":"imsi-001010000000001","ldr_type":"PERIODIC","periodic_event_info":{"reporting_amount":2,"reporting_interval":30},"method":"otdoa"}`, http.StatusBadRequest, "unsupported method: otdoa"},
		{`{"supi":"imsi-001010000000001","ldr_type":"PERIODIC","periodic_event_info":{"reporting_amount":2,"reporting_interval":30},"method":"cell_id","measurement_quantities":{"nr":["sS-RSRP"]}}`, http.StatusBadRequest, "only apply to network-based E-CID"},
		{`{"supi":"imsi-001010000000001","ldr_type":"PERIODIC","periodic_event_info":{"reporting_amount":2,"reporting_interval":30},"scheduled_loc_time":"tomorrow"}`, http.StatusBadRequest, "RFC 3339"},
		{`{"supi":"imsi-001010000000001","ldr_type":"PERIODIC","periodic_event_info":{"reporting_amount":2,"reporting_interval":30},"request_type":"periodic"}`, http.StatusBadRequest, "Invalid request body"},
		{`{"supi":"imsi-001010000000001","ldr_type":"PERIODIC","periodic_event_info":{"reporting_amount":2,"reporting_interval":30}}`, http.StatusNotFound, "UE not found"},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/beta/location/deferred-requests", strings.NewReader(tc.body))

		handler.ServeHTTP(rec, req)

		if rec.Code != tc.status {
			t.Errorf("%s: status = %d, want %d", tc.body, rec.Code, tc.status)
		}

		if !strings.Contains(rec.Body.String(), tc.message) {
			t.Errorf("%s: body %q does not contain %q", tc.body, rec.Body.String(), tc.message)
		}
	}
}

func TestWriteDeferredLocationError(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{db.ErrNotFound, http.StatusNotFound},
		{fmt.Errorf("UE not registered: %w", lmf.ErrNotFound), http.StatusNotFound},
		{fmt.Errorf("%w: too long", lmf.ErrInvalidPeriodicEventInfo), http.StatusBadRequest},
		{lmf.ErrInvalidScheduledLocTime, http.StatusBadRequest},
		{lmf.ErrDeferredRequestExists, http.StatusConflict},
		{lmf.ErrDeferredNotRunning, http.StatusServiceUnavailable},
		{db.ErrMigrationPending, http.StatusServiceUnavailable},
		{errors.New("boom"), http.StatusInternalServerError},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)

		writeDeferredLocationError(req, rec, tc.err)

		if rec.Code != tc.status {
			t.Errorf("%v: status = %d, want %d", tc.err, rec.Code, tc.status)
		}
	}
}

func TestDeferredLocationRequestReadEndpoints(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	lmfInstance := lmf.New(nil, nil, database)

	row := &db.DeferredLocationRequest{SUPI: "imsi-001010000000001", Method: "cell_id", ReportingIntervalS: 30, ReportingAmount: 2}
	if err := database.CreateDeferredLocationRequest(ctx, row); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := database.AddLocationReport(ctx, &db.LocationReport{RequestID: row.ID, Seq: 1, ReportedAt: 1000, LocationData: `{"supi":"imsi-001010000000001"}`}, db.LocationReportsKept); err != nil {
		t.Fatalf("seed report: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /deferred-requests", ListDeferredLocationRequests(lmfInstance))
	mux.Handle("GET /deferred-requests/{id}", GetDeferredLocationRequest(lmfInstance))
	mux.Handle("GET /deferred-requests/{id}/reports", ListLocationReports(lmfInstance))
	mux.Handle("DELETE /deferred-requests/{id}", DeleteDeferredLocationRequest(lmfInstance))

	for _, tc := range []struct {
		method, path string
		status       int
		contains     string
	}{
		{http.MethodGet, "/deferred-requests", http.StatusOK, `"total_count":1`},
		{http.MethodGet, "/deferred-requests?status=ACTIVE&supi=imsi-001010000000001", http.StatusOK, `"total_count":1`},
		{http.MethodGet, "/deferred-requests?status=TERMINATED", http.StatusOK, `"total_count":0`},
		{http.MethodGet, "/deferred-requests?status=DONE", http.StatusBadRequest, "status must be ACTIVE or TERMINATED"},
		{http.MethodGet, "/deferred-requests?per_page=101", http.StatusBadRequest, "per_page"},
		{http.MethodGet, "/deferred-requests/" + row.ID, http.StatusOK, `"event_report_counter":1`},
		{http.MethodGet, "/deferred-requests/" + row.ID, http.StatusOK, `"ldr_type":"PERIODIC"`},
		{http.MethodGet, "/deferred-requests/unknown", http.StatusNotFound, "not found"},
		{http.MethodGet, "/deferred-requests/" + row.ID + "/reports", http.StatusOK, `"next":1`},
		{http.MethodGet, "/deferred-requests/" + row.ID + "/reports?after=1", http.StatusOK, `"items":[],"next":1`},
		{http.MethodGet, "/deferred-requests/" + row.ID + "/reports?after=-1", http.StatusBadRequest, "after"},
		{http.MethodGet, "/deferred-requests/" + row.ID + "/reports?limit=1001", http.StatusBadRequest, "limit"},
		{http.MethodGet, "/deferred-requests/unknown/reports", http.StatusNotFound, "not found"},
		{http.MethodDelete, "/deferred-requests/" + row.ID, http.StatusNoContent, ""},
		{http.MethodDelete, "/deferred-requests/" + row.ID, http.StatusNotFound, "not found"},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(ctx, tc.method, tc.path, nil)

		mux.ServeHTTP(rec, req)

		if rec.Code != tc.status {
			t.Errorf("%s %s: status = %d, want %d (body %s)", tc.method, tc.path, rec.Code, tc.status, rec.Body.String())
		}

		if !strings.Contains(rec.Body.String(), tc.contains) {
			t.Errorf("%s %s: body %q does not contain %q", tc.method, tc.path, rec.Body.String(), tc.contains)
		}
	}
}
