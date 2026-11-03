// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/lmf"
	"github.com/ellanetworks/core/internal/roaming"
)

func TestLocationPrivacyRefusalIsForbidden(t *testing.T) {
	err := fmt.Errorf("%w: %w", lmf.ErrPrivacyRefused, fmt.Errorf("fetch: %w", context.DeadlineExceeded))

	rec := httptest.NewRecorder()
	writeLocationError(context.Background(), rec, err)

	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "TS 23.271 clause 9.5.3.4") {
		t.Fatalf("immediate: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	writeDeferredLocationError(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil), rec, err)

	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "TS 23.271 clause 9.5.3.4") {
		t.Fatalf("deferred: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLocatingARoamerTheHomeDoesNotAllowIsForbidden(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	ue := amfInstance.HoldVisitorForTest(supi, roaming.Visitor5G{PartnerID: "p", Registered: true})
	ue.ForceStateForTest(amf.Registered)

	l := lmf.New(amfInstance, nil, nil)

	for _, tc := range []struct {
		handler http.Handler
		path    string
		body    string
	}{
		{GetSubscriberLocation(l), "/api/beta/location", `{"supi":"imsi-001010000000001","method":"cell_id"}`},
		{CreateDeferredLocationRequest(l), "/api/beta/location/deferred-requests", `{"supi":"imsi-001010000000001","ldr_type":"PERIODIC","periodic_event_info":{"reporting_amount":2,"reporting_interval":30}}`},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, tc.path, strings.NewReader(tc.body))

		tc.handler.ServeHTTP(rec, req)

		var body struct {
			Error string `json:"error"`
		}

		_ = json.Unmarshal(rec.Body.Bytes(), &body)

		if rec.Code != http.StatusForbidden || body.Error != locationPrivacyRefusedMessage {
			t.Errorf("%s: %d %s, want 403 %q", tc.path, rec.Code, rec.Body.String(), locationPrivacyRefusedMessage)
		}
	}
}
