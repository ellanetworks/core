// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestListRoamers_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"imsi": "208930000000001", "partner": "partner", "home_plmn": {"mcc": "208", "mnc": "93"}, "msisdn": "+33612345678", "since": "2026-01-01T00:00:00Z", "status": {"registered": true, "connection_state": "idle", "systems": ["5G"], "num_sessions": 1}}], "page": 1, "per_page": 10, "total_count": 1}`),
		},
	}
	clientObj := &client.Client{Requester: fake}

	resp, err := clientObj.ListRoamers(context.Background(), &client.ListRoamersParams{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/roamers" {
		t.Fatalf("unexpected request %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	if resp.TotalCount != 1 || len(resp.Items) != 1 {
		t.Fatalf("unexpected response %+v", resp)
	}

	r := resp.Items[0]
	if r.Imsi != "208930000000001" || r.Partner != "partner" || r.HomePLMN == nil || r.HomePLMN.Mnc != "93" || r.Msisdn != "+33612345678" || r.Since != "2026-01-01T00:00:00Z" || r.Status.NumSessions != 1 {
		t.Fatalf("unexpected roamer %+v", r)
	}
}

func TestListRoamers_Query(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params client.ListRoamersParams
		want   string
	}{
		{"page only", client.ListRoamersParams{Page: 1, PerPage: 10}, "page=1&per_page=10"},
		{"partner", client.ListRoamersParams{Page: 1, PerPage: 10, Partner: "partner a"}, "page=1&partner=partner+a&per_page=10"},
		{"search", client.ListRoamersParams{Page: 2, PerPage: 25, Search: "+336"}, "page=2&per_page=25&search=%2B336"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeRequester{
				response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"items": [], "page": 1, "per_page": 10, "total_count": 0}`)},
			}
			clientObj := &client.Client{Requester: fake}

			if _, err := clientObj.ListRoamers(context.Background(), &tc.params); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if got := fake.lastOpts.Query.Encode(); got != tc.want {
				t.Fatalf("query = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetRoamer_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"imsi": "208930000000001", "partner": "partner", "home_plmn": {"mcc": "208", "mnc": "93"}, "since": "2026-01-01T00:00:00Z", "registrations": [{"system": "5G", "registered": true, "connection_state": "idle", "connection": null}], "sessions": [{"system": "5G", "id": 1, "status": "active"}]}`),
		},
	}
	clientObj := &client.Client{Requester: fake}

	roamer, err := clientObj.GetRoamer(context.Background(), "208930000000001")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/roamers/208930000000001" {
		t.Fatalf("unexpected request %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	if roamer.Partner != "partner" || roamer.HomePLMN == nil || len(roamer.Registrations) != 1 || len(roamer.Sessions) != 1 || roamer.Sessions[0].ID != 1 {
		t.Fatalf("unexpected roamer %+v", roamer)
	}
}

func TestDeregisterRoamer_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Roamer deregistered successfully"}`),
		},
	}
	clientObj := &client.Client{Requester: fake}

	if err := clientObj.DeregisterRoamer(context.Background(), "208930000000001"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" || fake.lastOpts.Path != "api/v1/roamers/208930000000001" {
		t.Fatalf("unexpected request %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}
