// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/cluster/joinreq"
)

func postJoin(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/cluster/join", strings.NewReader(body))
	rec := httptest.NewRecorder()

	ClusterJoin().ServeHTTP(rec, req)

	return rec
}

func TestClusterJoinRefusedWhenNotWaiting(t *testing.T) {
	SetJoinCoordinator(nil)

	rec := postJoin(t, `{"token":"`+strings.Repeat("a", 100)+`","seedAddresses":["10.0.0.1:7000"]}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("a node that is not waiting must answer 409, got %d", rec.Code)
	}

	c := joinreq.New()
	SetJoinCoordinator(c)

	t.Cleanup(func() { SetJoinCoordinator(nil) })

	rec = postJoin(t, `{"token":"`+strings.Repeat("a", 100)+`","seedAddresses":["10.0.0.1:7000"]}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("a closed coordinator must answer 409, got %d", rec.Code)
	}
}

func TestClusterJoinRejectsBadRequests(t *testing.T) {
	c := joinreq.New()
	c.Open()

	SetJoinCoordinator(c)

	t.Cleanup(func() { SetJoinCoordinator(nil) })

	good := strings.Repeat("a", 100)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"missing token", `{"seedAddresses":["10.0.0.1:7000"]}`, "token is required"},
		{"short token", `{"token":"abc","seedAddresses":["10.0.0.1:7000"]}`, "too short"},
		{"no seeds", `{"token":"` + good + `"}`, "seedAddresses"},
		{"seed is a url", `{"token":"` + good + `","seedAddresses":["https://10.0.0.1:7000"]}`, "looks like a URL"},
		{"seed has no port", `{"token":"` + good + `","seedAddresses":["10.0.0.1"]}`, "host:port"},
		{"bad suffrage", `{"token":"` + good + `","seedAddresses":["10.0.0.1:7000"],"suffrage":"maybe"}`, "suffrage"},
		{"not json", `{`, "Invalid request data"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postJoin(t, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d (%s)", rec.Code, rec.Body.String())
			}

			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("error should mention %q, got %s", tc.want, rec.Body.String())
			}
		})
	}
}

func TestClusterJoinAcceptsAndReportsProgress(t *testing.T) {
	c := joinreq.New()
	c.Open()

	SetJoinCoordinator(c)

	t.Cleanup(func() { SetJoinCoordinator(nil) })

	rec := postJoin(t, `{"token":"`+strings.Repeat("a", 100)+`","seedAddresses":["10.0.0.1:7000"]}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("a submitted join must answer 202, got %d (%s)", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"state":"joining"`) {
		t.Errorf("response should report the joining state, got %s", rec.Body.String())
	}

	rec = postJoin(t, `{"token":"`+strings.Repeat("a", 100)+`","seedAddresses":["10.0.0.1:7000"]}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("a second join while one runs must answer 409, got %d", rec.Code)
	}
}

func TestClusterJoinStatusIsPollable(t *testing.T) {
	SetJoinCoordinator(nil)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/cluster/join", nil)
	rec := httptest.NewRecorder()

	GetClusterJoinStatus().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"state":"unavailable"`) {
		t.Fatalf("a node with no coordinator must report unavailable, got %d (%s)", rec.Code, rec.Body.String())
	}

	c := joinreq.New()
	c.Open()

	SetJoinCoordinator(c)

	t.Cleanup(func() { SetJoinCoordinator(nil) })

	if err := c.Submit(joinreq.Request{Mode: joinreq.ModeJoin, Token: "t"}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if _, err := c.Await(t.Context()); err != nil {
		t.Fatalf("await: %v", err)
	}

	c.Report(errors.New("seed unreachable"))

	rec = httptest.NewRecorder()
	GetClusterJoinStatus().ServeHTTP(httptest.NewRecorder(), req)
	GetClusterJoinStatus().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `"state":"waiting"`) || !strings.Contains(body, "seed unreachable") {
		t.Fatalf("status must carry the failure back to the operator, got %s", body)
	}
}

func TestValidateClusterJoinRequest_RejectsUnroutableSeeds(t *testing.T) {
	token := strings.Repeat("A", 96)

	for _, addr := range []string{
		"169.254.169.254:7000",
		"0.0.0.0:7000",
		"224.0.0.1:7000",
		"[fe80::1]:7000",
	} {
		req := &ClusterJoinRequest{Token: token, SeedAddresses: []string{addr}}
		if err := validateClusterJoinRequest(req); err == nil {
			t.Errorf("seed %q must be rejected: a cluster peer never lives there", addr)
		}
	}
}

func TestValidateClusterJoinRequest_AcceptsRoutableSeeds(t *testing.T) {
	token := strings.Repeat("A", 96)

	for _, addr := range []string{
		"10.100.0.11:7000",
		"192.168.1.5:7000",
		"127.0.0.1:7000",
		"ella-core-2:7000",
		"[2001:db8::1]:7000",
	} {
		req := &ClusterJoinRequest{Token: token, SeedAddresses: []string{addr}}
		if err := validateClusterJoinRequest(req); err != nil {
			t.Errorf("seed %q must be accepted, got %v", addr, err)
		}
	}
}
