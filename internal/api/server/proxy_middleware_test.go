package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func TestIsWriteMethod(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"GET", false},
		{"HEAD", false},
		{"OPTIONS", false},
		{"POST", true},
		{"PUT", true},
		{"PATCH", true},
		{"DELETE", true},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := isWriteMethod(tt.method); got != tt.want {
				t.Errorf("isWriteMethod(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

func TestWaitForIndex_CatchesUpBeforeDeadline(t *testing.T) {
	var local atomic.Uint64

	local.Store(5)

	go func() {
		time.Sleep(20 * time.Millisecond)
		local.Store(10)
	}()

	got, caughtUp := waitForIndex(10, local.Load, 500*time.Millisecond, 2*time.Millisecond)
	if !caughtUp {
		t.Fatalf("expected catch-up before deadline, got timeout (localIdx=%d)", got)
	}

	if got < 10 {
		t.Fatalf("expected localIdx >= 10, got %d", got)
	}
}

func TestWaitForIndex_TimesOutReportsLastIndex(t *testing.T) {
	var local atomic.Uint64

	local.Store(7)

	got, caughtUp := waitForIndex(10, local.Load, 20*time.Millisecond, 2*time.Millisecond)
	if caughtUp {
		t.Fatalf("expected timeout, got catch-up (localIdx=%d)", got)
	}

	if got != 7 {
		t.Fatalf("expected last-observed localIdx=7, got %d", got)
	}
}

func TestWaitForIndex_AlreadyCaughtUpReturnsImmediately(t *testing.T) {
	var local atomic.Uint64

	local.Store(42)

	start := time.Now()

	got, caughtUp := waitForIndex(10, local.Load, 500*time.Millisecond, 50*time.Millisecond)
	if !caughtUp {
		t.Fatalf("expected immediate catch-up, got timeout")
	}

	if got != 42 {
		t.Fatalf("expected localIdx=42, got %d", got)
	}

	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("expected near-instant return, took %v", elapsed)
	}
}

// stubRoundTripper returns a canned response regardless of the request URL.
// Lets the proxy-client tests exercise branch logic without dialing a real
// listener.
type stubRoundTripper struct {
	statusCode int
	body       string
}

func (s *stubRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: s.statusCode,
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     make(http.Header),
	}, nil
}

func TestProxyToLeaderCluster_410TranslatesTo502(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	testDB, err := db.NewDatabase(context.Background(), dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	t.Cleanup(func() { _ = testDB.Close() })

	// Single-server raft bootstraps self as leader; LeaderAddress is the
	// local transport addr — non-empty, which is all proxyToLeaderCluster
	// needs since the stub RoundTripper never dials.
	deadline := time.Now().Add(3 * time.Second)
	for testDB.LeaderAddress() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if testDB.LeaderAddress() == "" {
		t.Fatal("single-server DB did not elect a leader")
	}

	client := &http.Client{Transport: &stubRoundTripper{
		statusCode: http.StatusGone,
		body:       `{"error":"node-id 5 is not a current cluster member"}`,
	}}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/subscribers", bytes.NewReader(nil))
	w := httptest.NewRecorder()

	doProxyToLeader(w, req, client, testDB.LeaderAddress(), testDB)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 410 from leader to translate to 502, got %d", w.Code)
	}

	var body struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !strings.Contains(body.Error, "no longer a cluster member") {
		t.Fatalf("expected evicted-node error message, got %q", body.Error)
	}
}

func TestProxyToLeaderCluster_PassesNon410Through(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	testDB, err := db.NewDatabase(context.Background(), dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	t.Cleanup(func() { _ = testDB.Close() })

	deadline := time.Now().Add(3 * time.Second)
	for testDB.LeaderAddress() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	client := &http.Client{Transport: &stubRoundTripper{
		statusCode: http.StatusCreated,
		body:       `{"result":{"message":"ok"}}`,
	}}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/api/v1/subscribers", bytes.NewReader(nil))
	w := httptest.NewRecorder()

	doProxyToLeader(w, req, client, testDB.LeaderAddress(), testDB)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected non-410 response to pass through, got %d", w.Code)
	}
}

func TestIsSelfRemoval(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		localNodeID int
		want        bool
	}{
		{"self DELETE", http.MethodDelete, "/api/v1/cluster/members/3", 3, true},
		{"other DELETE", http.MethodDelete, "/api/v1/cluster/members/3", 1, false},
		{"self GET is not a write", http.MethodGet, "/api/v1/cluster/members/3", 3, false},
		{"self POST promote is not a remove", http.MethodPost, "/api/v1/cluster/members/3/promote", 3, false},
		{"self DELETE with trailing slash is not matched", http.MethodDelete, "/api/v1/cluster/members/3/", 3, false},
		{"unrelated DELETE", http.MethodDelete, "/api/v1/subscribers/001010", 3, false},
		{"members list DELETE (no id)", http.MethodDelete, "/api/v1/cluster/members/", 3, false},
		{"non-numeric id", http.MethodDelete, "/api/v1/cluster/members/abc", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), tt.method, tt.path, nil)
			if got := isSelfRemoval(req, tt.localNodeID); got != tt.want {
				t.Errorf("isSelfRemoval(%s %s, local=%d) = %v, want %v", tt.method, tt.path, tt.localNodeID, got, tt.want)
			}
		})
	}
}

func TestLeaderProxyMiddleware_NilDB(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
	})

	handler := LeaderProxyMiddleware(nil, nil, next)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/subscribers", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected next handler to be called when dbInstance is nil (standalone)")
	}
}
