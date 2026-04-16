// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newProbePeerServer(t *testing.T, statusCode int, body statusResponse) *httptest.Server {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/status" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}))

	t.Cleanup(ts.Close)

	return ts
}

func TestProbePeer_LeaderReturns200(t *testing.T) {
	ts := newProbePeerServer(t, http.StatusOK, statusResponse{
		Result: statusResult{
			Cluster: &statusClusterBlock{
				Role:          "Leader",
				NodeID:        1,
				ClusterID:     "cluster-1",
				SchemaVersion: 9,
			},
		},
	})

	m := &Manager{}
	state, nodeID, clusterID, schema := m.probePeer(context.Background(), ts.Client(), ts.URL)

	if state != peerFormed {
		t.Fatalf("expected peerFormed, got %d", state)
	}

	if nodeID != 1 {
		t.Fatalf("expected nodeID=1, got %d", nodeID)
	}

	if clusterID != "cluster-1" {
		t.Fatalf("expected clusterID=cluster-1, got %s", clusterID)
	}

	if schema != 9 {
		t.Fatalf("expected schema=9, got %d", schema)
	}
}

func TestProbePeer_FollowerReturns200(t *testing.T) {
	ts := newProbePeerServer(t, http.StatusOK, statusResponse{
		Result: statusResult{
			Cluster: &statusClusterBlock{
				Role:          "Follower",
				NodeID:        2,
				ClusterID:     "cluster-1",
				SchemaVersion: 9,
			},
		},
	})

	m := &Manager{}
	state, nodeID, clusterID, schema := m.probePeer(context.Background(), ts.Client(), ts.URL)

	if state != peerFormed {
		t.Fatalf("expected peerFormed, got %d", state)
	}

	if nodeID != 2 {
		t.Fatalf("expected nodeID=2, got %d", nodeID)
	}

	if clusterID != "cluster-1" {
		t.Fatalf("expected clusterID=cluster-1, got %s", clusterID)
	}

	if schema != 9 {
		t.Fatalf("expected schema=9, got %d", schema)
	}
}

func TestProbePeer_FormingNode(t *testing.T) {
	ts := newProbePeerServer(t, http.StatusOK, statusResponse{
		Result: statusResult{
			Cluster: &statusClusterBlock{
				Role:   "Follower",
				NodeID: 3,
			},
		},
	})

	m := &Manager{}
	state, nodeID, _, _ := m.probePeer(context.Background(), ts.Client(), ts.URL)

	if state != peerForming {
		t.Fatalf("expected peerForming, got %d", state)
	}

	if nodeID != 3 {
		t.Fatalf("expected nodeID=3, got %d", nodeID)
	}
}

func TestProbePeer_503IsUnreachable(t *testing.T) {
	ts := newProbePeerServer(t, http.StatusServiceUnavailable, statusResponse{})

	m := &Manager{}
	state, nodeID, clusterID, schema := m.probePeer(context.Background(), ts.Client(), ts.URL)

	if state != peerUnreachable {
		t.Fatalf("expected peerUnreachable, got %d", state)
	}

	if nodeID != 0 {
		t.Fatalf("expected nodeID=0, got %d", nodeID)
	}

	if clusterID != "" {
		t.Fatalf("expected empty clusterID, got %s", clusterID)
	}

	if schema != 0 {
		t.Fatalf("expected schema=0, got %d", schema)
	}
}
