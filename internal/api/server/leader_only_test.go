// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

type fakeLeadership struct {
	enabled  bool
	leader   bool
	leaderAt string
	members  []db.ClusterMember
}

func (f *fakeLeadership) ClusterEnabled() bool  { return f.enabled }
func (f *fakeLeadership) IsLeader() bool        { return f.leader }
func (f *fakeLeadership) LeaderAddress() string { return f.leaderAt }

func (f *fakeLeadership) ListClusterMembers(context.Context) ([]db.ClusterMember, error) {
	return f.members, nil
}

func serveLeaderOnly(t *testing.T, l clusterLeadership) *httptest.ResponseRecorder {
	t.Helper()

	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/cluster/members/2", nil)
	w := httptest.NewRecorder()

	LeaderOnly(l, next).ServeHTTP(w, req)

	if reached != (w.Code == http.StatusOK) {
		t.Fatalf("handler reached = %v with status %d", reached, w.Code)
	}

	return w
}

func TestLeaderOnlyServesStandaloneDeployments(t *testing.T) {
	w := serveLeaderOnly(t, &fakeLeadership{enabled: false, leader: false})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: a standalone node has no leader to defer to", w.Code)
	}
}

func TestLeaderOnlyServesTheLeader(t *testing.T) {
	w := serveLeaderOnly(t, &fakeLeadership{enabled: true, leader: true})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestLeaderOnlyNamesTheLeaderToAFollower(t *testing.T) {
	w := serveLeaderOnly(t, &fakeLeadership{
		enabled:  true,
		leader:   false,
		leaderAt: "10.0.0.1:7000",
		members: []db.ClusterMember{
			{NodeID: 1, RaftAddress: "10.0.0.1:7000", APIAddress: "https://10.0.0.1:5002"},
			{NodeID: 2, RaftAddress: "10.0.0.2:7000", APIAddress: "https://10.0.0.2:5002"},
		},
	})

	if w.Code != http.StatusMisdirectedRequest {
		t.Fatalf("status = %d, want 421", w.Code)
	}

	var body struct {
		Error string `json:"error"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %s", err)
	}

	if !strings.Contains(body.Error, "node 1") || !strings.Contains(body.Error, "https://10.0.0.1:5002") {
		t.Fatalf("error = %q, want the leader's node id and API address", body.Error)
	}
}

func TestLeaderOnlyFallsBackWhenTheLeaderIsUnknown(t *testing.T) {
	w := serveLeaderOnly(t, &fakeLeadership{enabled: true, leader: false})

	if w.Code != http.StatusMisdirectedRequest {
		t.Fatalf("status = %d, want 421", w.Code)
	}

	if !strings.Contains(w.Body.String(), "not the leader") {
		t.Fatalf("body = %q, want a not-the-leader message", w.Body.String())
	}
}
