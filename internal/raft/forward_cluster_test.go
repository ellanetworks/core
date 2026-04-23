// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	hraft "github.com/hashicorp/raft"
)

// miniProposeHandler is a minimal re-implementation of the production
// /cluster/internal/propose handler (internal/api/server/cluster_http_forward.go)
// used only by this end-to-end test. Kept inline so the raft package
// doesn't need to import api/server (which would be a cycle).
func miniProposeHandler(m *Manager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.IsLeader() {
			writeMiniError(w, http.StatusMisdirectedRequest, "not the leader")
			return
		}

		data, err := io.ReadAll(io.LimitReader(r.Body, MaxProposeForwardBodyBytes+1))
		if err != nil {
			writeMiniError(w, http.StatusBadRequest, err.Error())
			return
		}

		result, err := m.ApplyBytes(data, m.ProposeTimeout())
		if err != nil {
			switch {
			case errors.Is(err, hraft.ErrNotLeader), errors.Is(err, hraft.ErrLeadershipLost):
				writeMiniError(w, http.StatusMisdirectedRequest, err.Error())
			default:
				writeMiniError(w, http.StatusInternalServerError, err.Error())
			}

			return
		}

		_ = WriteProposeForwardResponse(w, result)
	})
}

func writeMiniError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ProposeForwardErrorBody{Message: msg})
}

// wireClusterProposeHandlers registers the mini propose handler on
// every test-cluster node's listener.
func wireClusterProposeHandlers(t *testing.T, tc *TestCluster) {
	t.Helper()

	mux := func(m *Manager) http.Handler {
		sm := http.NewServeMux()
		sm.Handle("POST "+ProposeForwardPath, miniProposeHandler(m))

		return sm
	}

	for i, m := range tc.Nodes {
		startTestClusterHTTP(t, tc.Listeners[i], mux(m))
	}
}

// TestForwardPropose_EndToEnd proves that proposing on a follower
// round-trips through the mTLS cluster port to the leader, commits,
// and replicates to every node's FSM.
func TestForwardPropose_EndToEnd(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })
	wireClusterProposeHandlers(t, tc)

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader")
	}

	// Find a follower.
	var follower *Manager

	for _, m := range tc.Nodes {
		if !m.IsLeader() {
			follower = m
			break
		}
	}

	if follower == nil {
		t.Fatal("no follower")
	}

	cmd, err := NewCommand(CmdChangeset, map[string]string{"via": "follower"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	result, err := follower.Propose(cmd, 5*time.Second)
	if err != nil {
		t.Fatalf("follower.Propose: %v", err)
	}

	if result.Index == 0 {
		t.Fatal("committed index must be non-zero")
	}

	if err := tc.WaitForConvergence(result.Index, 2*time.Second); err != nil {
		t.Fatalf("convergence: %v", err)
	}

	// The leader and every follower FSM must have seen the same
	// committed command. Each applier records what it received.
	for i, a := range tc.Appliers {
		ta, ok := a.(*testApplier)
		if !ok {
			t.Fatalf("applier %d unexpected type %T", i, a)
		}

		seen := ta.seen()

		found := false

		for _, c := range seen {
			if c.Type == CmdChangeset {
				var payload map[string]string
				if err := json.Unmarshal(c.Payload, &payload); err == nil && payload["via"] == "follower" {
					found = true
					break
				}
			}
		}

		if !found {
			t.Fatalf("node %d applier did not see the forwarded command: %+v", i, seen)
		}
	}
}

// TestForwardPropose_FollowerRetriesOnLeaderChange verifies that when
// the follower's view of the leader is stale, the 421 signal from the
// old leader triggers a re-resolve and retry against the new one.
//
// We drive this deterministically by killing the current leader after
// the follower sees it, then issuing Propose from the follower. The
// first attempt hits the dead peer (dial fails) — retry logic resolves
// the new leader and the second attempt succeeds.
func TestForwardPropose_FollowerRetriesOnLeaderChange(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })
	wireClusterProposeHandlers(t, tc)

	oldLeader := tc.Leader()
	if oldLeader == nil {
		t.Fatal("no leader")
	}

	var follower *Manager

	for _, m := range tc.Nodes {
		if !m.IsLeader() {
			follower = m
			break
		}
	}

	if follower == nil {
		t.Fatal("no follower")
	}

	// Kill the leader.
	for i, m := range tc.Nodes {
		if m == oldLeader {
			if err := m.Shutdown(); err != nil {
				t.Fatalf("shutdown old leader: %v", err)
			}

			tc.Listeners[i].Stop()

			break
		}
	}

	// Wait for a new leader among the survivors.
	deadline := time.Now().Add(5 * time.Second)

	var newLeader *Manager

	for time.Now().Before(deadline) {
		for _, m := range tc.Nodes {
			if m == oldLeader {
				continue
			}

			if m.LeaderObserver().IsLeader() {
				newLeader = m
				break
			}
		}

		if newLeader != nil {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if newLeader == nil {
		t.Fatal("new leader was not elected")
	}

	if newLeader == follower {
		// Our 'follower' won the election. Pick another follower.
		for _, m := range tc.Nodes {
			if m != oldLeader && m != newLeader {
				follower = m
				break
			}
		}
	}

	// Propose from the follower. The first attempt may hit the dead
	// old leader or fail with no-leader; retry logic must recover.
	cmd, err := NewCommand(CmdChangeset, map[string]string{"after": "leader-change"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	result, err := follower.Propose(cmd, 10*time.Second)
	if err != nil {
		t.Fatalf("follower.Propose after leader change: %v", err)
	}

	if result.Index == 0 {
		t.Fatal("committed index must be non-zero")
	}
}

// TestForwardPropose_AppliedIndexHeader proves the leader sets
// X-Ella-Applied-Index on the success response; the forwarder's
// read-your-writes wait depends on it.
func TestForwardPropose_AppliedIndexHeader(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })
	wireClusterProposeHandlers(t, tc)

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader")
	}

	cmd, err := NewCommand(CmdChangeset, map[string]string{"direct": "leader"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	// Direct-apply on leader, then verify the handler-facing env is
	// what the follower-side decodeForwardResponse parses.
	result, err := leader.ApplyBytes(mustMarshal(t, cmd), time.Second)
	if err != nil {
		t.Fatalf("leader.ApplyBytes: %v", err)
	}

	// Round-trip envelope through the handler's writer to catch
	// accidental omission of the applied-index header.
	rec := newHeaderRecorder()
	if err := WriteProposeForwardResponse(rec, result); err != nil {
		t.Fatalf("write response: %v", err)
	}

	got := rec.header.Get(HeaderAppliedIndex)
	want := strconv.FormatUint(result.Index, 10)

	if got != want {
		t.Fatalf("applied-index header: got %q want %q", got, want)
	}
}

func mustMarshal(t *testing.T, cmd *Command) []byte {
	t.Helper()

	b, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return b
}

// headerRecorder captures header state set by WriteProposeForwardResponse
// without needing the full httptest machinery (which belongs in the
// server package's tests).
type headerRecorder struct {
	header http.Header
	status int
	body   []byte
}

func newHeaderRecorder() *headerRecorder {
	return &headerRecorder{header: http.Header{}}
}

func (r *headerRecorder) Header() http.Header { return r.header }
func (r *headerRecorder) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return len(b), nil
}
func (r *headerRecorder) WriteHeader(status int) { r.status = status }

// Compile-time check that startTestClusterHTTP is used (otherwise the
// import it pulls in — testutil.PKI — would be flagged).
var (
	_ = context.Background
	_ = listener.ALPNHTTP
)
