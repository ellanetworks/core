// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

type fakeLeadership struct {
	enabled    bool
	leader     bool
	leaderID   string
	member     *db.ClusterMember
	memberErr  error
	lookupCall int
}

func (f *fakeLeadership) ClusterEnabled() bool { return f.enabled }
func (f *fakeLeadership) IsLeader() bool       { return f.leader }

func (f *fakeLeadership) LeaderAddressAndID() (string, string) {
	return "", f.leaderID
}

func (f *fakeLeadership) GetClusterMember(_ context.Context, _ string) (*db.ClusterMember, error) {
	f.lookupCall++

	if f.memberErr != nil {
		return nil, f.memberErr
	}

	return f.member, nil
}

func leaderOnlyResult(t *testing.T, f *fakeLeadership) (int, string, bool) {
	t.Helper()

	served := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served = true

		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/cluster/members/abc/drain", nil)

	LeaderOnly(f, next).ServeHTTP(rec, req)

	message := ""

	if rec.Code != http.StatusOK {
		var body ErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode error body: %v", err)
		}

		message = body.Error
	}

	return rec.Code, message, served
}

func TestLeaderOnlyPassesThrough(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    *fakeLeadership
	}{
		{"cluster disabled", &fakeLeadership{enabled: false, leader: false}},
		{"is leader", &fakeLeadership{enabled: true, leader: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, served := leaderOnlyResult(t, tc.f)

			if !served {
				t.Fatal("expected the wrapped handler to run")
			}

			if code != http.StatusOK {
				t.Fatalf("got status %d, want %d", code, http.StatusOK)
			}

			if tc.f.lookupCall != 0 {
				t.Fatalf("leader lookup ran %d times on the pass-through path", tc.f.lookupCall)
			}
		})
	}
}

func TestLeaderOnlyRejectsFollower(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    *fakeLeadership
		want string
	}{
		{
			name: "no leader elected",
			f:    &fakeLeadership{enabled: true, leaderID: ""},
			want: "This node is not the cluster leader; no leader is currently elected, retry shortly",
		},
		{
			name: "leader row missing",
			f: &fakeLeadership{
				enabled:   true,
				leaderID:  "11111111-1111-1111-1111-111111111111",
				memberErr: db.ErrNotFound,
			},
			want: "This node is not the cluster leader; retry against node 11111111-1111-1111-1111-111111111111",
		},
		{
			name: "leader row without an api address",
			f: &fakeLeadership{
				enabled:  true,
				leaderID: "11111111-1111-1111-1111-111111111111",
				member:   &db.ClusterMember{NodeID: "11111111-1111-1111-1111-111111111111", DisplayName: "node-a"},
			},
			want: "This node is not the cluster leader; retry against node node-a (11111111-1111-1111-1111-111111111111)",
		},
		{
			name: "leader named and addressable",
			f: &fakeLeadership{
				enabled:  true,
				leaderID: "11111111-1111-1111-1111-111111111111",
				member: &db.ClusterMember{
					NodeID:      "11111111-1111-1111-1111-111111111111",
					DisplayName: "node-a",
					APIAddress:  "10.0.0.1:5002",
				},
			},
			want: "This node is not the cluster leader; retry against node node-a (11111111-1111-1111-1111-111111111111) at 10.0.0.1:5002",
		},
		{
			name: "leader without a display name",
			f: &fakeLeadership{
				enabled:  true,
				leaderID: "11111111-1111-1111-1111-111111111111",
				member: &db.ClusterMember{
					NodeID:     "11111111-1111-1111-1111-111111111111",
					APIAddress: "10.0.0.1:5002",
				},
			},
			want: "This node is not the cluster leader; retry against node 11111111-1111-1111-1111-111111111111 at 10.0.0.1:5002",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, message, served := leaderOnlyResult(t, tc.f)

			if served {
				t.Fatal("wrapped handler ran on a follower")
			}

			if code != http.StatusMisdirectedRequest {
				t.Fatalf("got status %d, want %d", code, http.StatusMisdirectedRequest)
			}

			if message != tc.want {
				t.Fatalf("got message %q, want %q", message, tc.want)
			}
		})
	}
}

func TestLeaderOnlyLookupFailureStillNamesTheLeader(t *testing.T) {
	f := &fakeLeadership{
		enabled:   true,
		leaderID:  "11111111-1111-1111-1111-111111111111",
		memberErr: errors.New("boom"),
	}

	code, message, _ := leaderOnlyResult(t, f)

	if code != http.StatusMisdirectedRequest {
		t.Fatalf("got status %d, want %d", code, http.StatusMisdirectedRequest)
	}

	want := "This node is not the cluster leader; retry against node 11111111-1111-1111-1111-111111111111"
	if message != want {
		t.Fatalf("got message %q, want %q", message, want)
	}
}

var _ clusterLeadership = (*db.Database)(nil)
