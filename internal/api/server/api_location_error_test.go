// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/lmf"
	"github.com/ellanetworks/core/internal/lmf/lpp"
)

func TestWriteLocationError(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"UE not found", lmf.ErrNotFound, http.StatusNotFound, "UE not found"},
		{"no cell coordinate", lmf.ErrNoLocationEstimate, http.StatusNotFound, "no coordinate for serving cell"},
		{"UE provided no estimate", fmt.Errorf("LPP session failed: %w", lpp.ErrUENoLocationEstimate), http.StatusNotFound, "UE provided no location estimate"},
		{"unsupported shape", fmt.Errorf("LPP session failed: %w", lpp.ErrUnsupportedLocationShape), http.StatusNotFound, "unsupported shape"},
		{"timeout", fmt.Errorf("GNSS positioning timed out: %w", context.DeadlineExceeded), http.StatusGatewayTimeout, "timed out"},
		{"other", errors.New("boom"), http.StatusInternalServerError, "Failed to determine location"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			writeLocationError(context.Background(), rec, tc.err)

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}

			if !strings.Contains(rec.Body.String(), tc.message) {
				t.Errorf("body = %s, want it to contain %q", rec.Body.String(), tc.message)
			}
		})
	}
}

func TestGetSubscriberLocationRejectsUnsupportedMode(t *testing.T) {
	handler := GetSubscriberLocation(lmf.New(nil, nil, nil))

	for _, tc := range []struct {
		body    string
		message string
	}{
		{`{"supi":"imsi-001010000000001","request_type":"immediate","method":"ecid","mode":"ue_based"}`, "supported: ue_assisted, network_based"},
		{`{"supi":"imsi-001010000000001","request_type":"immediate","method":"gnss","mode":"ue_assisted"}`, "supported: standalone"},
		{`{"supi":"imsi-001010000000001","request_type":"immediate","method":"cell_id","mode":"network_based"}`, "supported: none"},
		{`{"supi":"imsi-001010000000001","request_type":"immediate","mode":"ue_assisted"}`, "for method cell_id"},
		{`{"supi":"imsi-001010000000001","request_type":"immediate","method":"otdoa"}`, "unsupported method: otdoa"},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/beta/location", strings.NewReader(tc.body))

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", tc.body, rec.Code)
		}

		if !strings.Contains(rec.Body.String(), tc.message) {
			t.Errorf("%s: body %q does not contain %q", tc.body, rec.Body.String(), tc.message)
		}
	}
}
