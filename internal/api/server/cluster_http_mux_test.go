// Copyright 2026 Ella Networks

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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
