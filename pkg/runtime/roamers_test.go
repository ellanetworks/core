// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/prometheus/client_golang/prometheus"
)

type fakeRoamerSource struct {
	load    map[string]roaming.PartnerLoad
	roamers []roaming.LiveRoamer
}

func (f fakeRoamerSource) RoamerLoad() map[string]roaming.PartnerLoad { return f.load }

func (f fakeRoamerSource) Roamers() []roaming.LiveRoamer { return f.roamers }

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("unreachable")
}

type fakeN32 struct{}

func (fakeN32) Client(string) *http.Client { return &http.Client{Transport: failingTransport{}} }

func (fakeN32) NRFAPIRoot(string) (string, error) { return "", errors.New("no nrf") }

func (fakeN32) PartnerName(string) string { return "partner-a" }

func TestRoamingMetricsExposedOnMetricsEndpoint(t *testing.T) {
	registerRoamers(prometheus.DefaultRegisterer,
		fakeRoamerSource{load: map[string]roaming.PartnerLoad{"partner-a": {Roamers: 2, Sessions: 3}}},
		fakeRoamerSource{load: map[string]roaming.PartnerLoad{"partner-b": {Roamers: 1, Sessions: 1}}},
	)

	roaming.RecordReject5G("partner-a", "PLMN not allowed")
	roaming.RecordS6aAnswer("partner-b", s6a.CommandUpdateLocation, nil, errors.New("no peer"))

	_, _ = roaming.NewHomeSBI(fakeN32{}, nil).Do(t.Context(), "id-a", roaming.HomeRequest{
		Method: http.MethodGet,
		URL:    "https://home.example/nudm-sdm/v2/imsi-001010000000001/am-data",
	})

	rr := httptest.NewRecorder()
	server.GetMetrics().ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/metrics", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	for _, want := range []string{
		`app_roaming_visitors{partner="partner-a",rat="5g"} 2`,
		`app_roaming_visitors{partner="partner-b",rat="4g"} 1`,
		`app_roaming_sessions{partner="partner-a",rat="5g"} 3`,
		`app_roaming_sessions{partner="partner-b",rat="4g"} 1`,
		`app_roaming_rejects_total{cause="PLMN not allowed",partner="partner-a",rat="5g"} 1`,
		`app_roaming_s6a_requests_total{command="ULR",partner="partner-b",result="error"} 1`,
		`app_roaming_home_sbi_requests_total{partner="partner-a",result="error",service="nudm-sdm"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics is missing %s", want)
		}
	}
}
