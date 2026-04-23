// Copyright 2026 Ella Networks

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

// newTestDB creates a real SQLite-backed *db.Database for in-package tests.
func newTestDB(t *testing.T) *db.Database {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")

	testDB, err := db.NewDatabase(context.Background(), dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	t.Cleanup(func() { _ = testDB.Close() })

	return testDB
}

// ctxWithPeerNodeID injects a peer node-id into the request context using
// the same key that peerNodeIDConnContext uses in production. Exists so
// in-package tests do not need to stand up a TLS connection.
func ctxWithPeerNodeID(ctx context.Context, nodeID int) context.Context {
	return context.WithValue(ctx, peerNodeIDCtxKey{}, nodeID)
}

func TestRemovedNodeFence_RejectsUnknownPeer(t *testing.T) {
	testDB := newTestDB(t)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	handler := removedNodeFence(testDB, next)

	req := httptest.NewRequestWithContext(
		ctxWithPeerNodeID(context.Background(), 42),
		http.MethodPost, "/api/v1/subscribers", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", w.Code)
	}

	if nextCalled {
		t.Fatal("next handler must not be called when peer is fenced")
	}
}

func TestRemovedNodeFence_AllowsCurrentMember(t *testing.T) {
	testDB := newTestDB(t)

	if err := testDB.UpsertClusterMember(context.Background(), &db.ClusterMember{
		NodeID:      7,
		RaftAddress: "127.0.0.1:9000",
		APIAddress:  "127.0.0.1:9001",
		Suffrage:    "voter",
	}); err != nil {
		t.Fatalf("seed cluster member: %v", err)
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	handler := removedNodeFence(testDB, next)

	req := httptest.NewRequestWithContext(
		ctxWithPeerNodeID(context.Background(), 7),
		http.MethodPost, "/api/v1/subscribers", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if !nextCalled {
		t.Fatal("expected next handler to be called for current member")
	}
}

// newLeaderTestDB creates a real SQLite-backed DB and waits up to 3s for
// the embedded single-node Raft cluster to elect itself leader, so the
// returned DB is guaranteed to accept replicated write calls.
func newLeaderTestDB(t *testing.T) *db.Database {
	t.Helper()

	testDB := newTestDB(t)

	deadline := time.Now().Add(3 * time.Second)
	for !testDB.IsLeader() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if !testDB.IsLeader() {
		t.Fatal("single-node DB did not become leader")
	}

	return testDB
}

func TestDrainSelfOnLeader_HappyPath(t *testing.T) {
	testDB := newLeaderTestDB(t)

	const peerID = 42
	if err := testDB.UpsertClusterMember(context.Background(), &db.ClusterMember{
		NodeID:      peerID,
		RaftAddress: "10.0.0.42:7000",
		APIAddress:  "http://10.0.0.42:5000",
		Suffrage:    "voter",
	}); err != nil {
		t.Fatalf("upsert peer: %v", err)
	}

	handler := DrainSelfOnLeader(testDB)

	req := httptest.NewRequestWithContext(
		ctxWithPeerNodeID(context.Background(), peerID),
		http.MethodPost, "/cluster/internal/drain-self", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	member, err := testDB.GetClusterMember(context.Background(), peerID)
	if err != nil {
		t.Fatalf("get peer: %v", err)
	}

	if member.DrainState != db.DrainStateDrained {
		t.Errorf("expected drainState=drained, got %s", member.DrainState)
	}
}

func TestDrainSelfOnLeader_MissingPeerIdentity(t *testing.T) {
	testDB := newLeaderTestDB(t)

	handler := DrainSelfOnLeader(testDB)

	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, "/cluster/internal/drain-self", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when peer identity missing, got %d", w.Code)
	}
}

func TestDrainSelfOnLeader_UnknownPeer(t *testing.T) {
	testDB := newLeaderTestDB(t)

	handler := DrainSelfOnLeader(testDB)

	req := httptest.NewRequestWithContext(
		ctxWithPeerNodeID(context.Background(), 999),
		http.MethodPost, "/cluster/internal/drain-self", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown peer, got %d", w.Code)
	}
}

func TestDrainLocalSideEffects_NoDeps(t *testing.T) {
	// Clear any previously-set deps so this test sees the pre-Upgrade state.
	clusterSideEffectDeps.Store(nil)

	handler := DrainLocalSideEffects()

	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, "/cluster/internal/drain-side-effects", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when deps not installed, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "dependencies not yet installed") {
		t.Errorf("expected deps-missing message, got %s", w.Body.String())
	}
}

func TestResumeLocalSideEffects_NoDeps(t *testing.T) {
	clusterSideEffectDeps.Store(nil)

	handler := ResumeLocalSideEffects()

	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, "/cluster/internal/resume-side-effects", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when deps not installed, got %d", w.Code)
	}
}

func TestDrainLocalSideEffects_HappyPath(t *testing.T) {
	// AMF nil is acceptable (notifyRANsUnavailable treats nil as a no-op);
	// BGP nil is also acceptable. With both nil, the handler still succeeds
	// and reports zero notifications / bgpStopped=false.
	SetClusterSideEffectDeps(ClusterSideEffectDeps{AMF: nil, BGP: nil})

	t.Cleanup(func() { clusterSideEffectDeps.Store(nil) })

	handler := DrainLocalSideEffects()

	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, "/cluster/internal/drain-side-effects", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var env struct {
		Result DrainSideEffectsResponse `json:"result"`
	}

	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if env.Result.RANsNotified != 0 {
		t.Errorf("expected 0 RANs notified with nil AMF, got %d", env.Result.RANsNotified)
	}

	if env.Result.BGPStopped {
		t.Errorf("expected bgpStopped=false with nil BGP, got true")
	}
}

func TestRemovedNodeFence_MissingPeerIdentity(t *testing.T) {
	testDB := newTestDB(t)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true

		w.WriteHeader(http.StatusOK)
	})

	handler := removedNodeFence(testDB, next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/subscribers", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when peer identity missing, got %d", w.Code)
	}

	if nextCalled {
		t.Fatal("next handler must not be called without peer identity")
	}
}
