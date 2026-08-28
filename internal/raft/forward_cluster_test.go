// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
//
// The test mini-dispatcher doesn't look up a typed-op registry (that
// lives in internal/db); instead it treats every forwarded envelope as
// a CmdChangeset carrying the raw payload. That's enough to exercise
// the transport + retry + convergence paths the raft package owns.
func miniProposeHandler(m *Manager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.IsLeader() {
			writeMiniError(w, http.StatusMisdirectedRequest, "not the leader")
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, MaxProposeForwardBodyBytes+1))
		if err != nil {
			writeMiniError(w, http.StatusBadRequest, err.Error())
			return
		}

		var envelope ProposeForwardRequest
		if err := json.Unmarshal(body, &envelope); err != nil {
			writeMiniError(w, http.StatusBadRequest, err.Error())
			return
		}

		cmd := &Command{Type: CmdChangeset, Payload: envelope.Payload}

		data, err := cmd.MarshalBinary()
		if err != nil {
			writeMiniError(w, http.StatusInternalServerError, err.Error())
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

// wireClusterProposeHandlers registers the mini propose handler on every
// test-cluster node's listener, and re-registers it after a restart so the
// handler always serves the node's current Manager.
func wireClusterProposeHandlers(t *testing.T, tc *TestCluster) {
	t.Helper()

	tc.WireHandlers([]string{listener.ALPNHTTP}, func(_ int, m *Manager, ln *listener.Listener) {
		sm := http.NewServeMux()
		sm.Handle("POST "+ProposeForwardPath, miniProposeHandler(m))

		startTestClusterHTTP(t, ln, sm)
	})
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

	payload, err := json.Marshal(map[string]string{"via": "follower"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result, err := follower.ForwardOperation(context.Background(), "TestOp", payload, 5*time.Second)
	if err != nil {
		t.Fatalf("follower.ForwardOperation: %v", err)
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
	payload, err := json.Marshal(map[string]string{"after": "leader-change"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result, err := follower.ForwardOperation(context.Background(), "TestOp", payload, 10*time.Second)
	if err != nil {
		t.Fatalf("follower.ForwardOperation after leader change: %v", err)
	}

	if result.Index == 0 {
		t.Fatal("committed index must be non-zero")
	}
}

// TestWriteProposeForwardResponse_EnvelopeAndHeader pins both consumers of
// the leader's success envelope. doForwardRequest decodes Index and Value
// from the JSON body — that is the value the follower returns to its
// caller — while the operator-API proxy reads X-Ella-Applied-Index from the
// header. Both must carry the committed index.
func TestWriteProposeForwardResponse_EnvelopeAndHeader(t *testing.T) {
	result := &ProposeResult{
		Index: 4242,
		Value: map[string]string{"direct": "leader"},
	}

	rec := newHeaderRecorder()
	if err := WriteProposeForwardResponse(rec, result); err != nil {
		t.Fatalf("write response: %v", err)
	}

	if rec.status != http.StatusOK {
		t.Errorf("status: got %d want %d", rec.status, http.StatusOK)
	}

	gotHeader := rec.header.Get(HeaderAppliedIndex)

	wantHeader := strconv.FormatUint(result.Index, 10)
	if gotHeader != wantHeader {
		t.Errorf("applied-index header: got %q want %q", gotHeader, wantHeader)
	}

	var env ProposeForwardResponse
	if err := json.Unmarshal(rec.body, &env); err != nil {
		t.Fatalf("decode envelope: %v (body=%q)", err, rec.body)
	}

	if env.Index != result.Index {
		t.Errorf("envelope index: got %d want %d", env.Index, result.Index)
	}

	var value map[string]string
	if err := json.Unmarshal(env.Value, &value); err != nil {
		t.Fatalf("decode envelope value: %v (value=%q)", err, env.Value)
	}

	if value["direct"] != "leader" {
		t.Errorf("envelope value: got %v want map[direct:leader]", value)
	}
}

// TestWriteProposeForwardResponse_NilValue covers the intent-op shape: no
// result value, but the index must still reach both consumers.
func TestWriteProposeForwardResponse_NilValue(t *testing.T) {
	rec := newHeaderRecorder()
	if err := WriteProposeForwardResponse(rec, &ProposeResult{Index: 7}); err != nil {
		t.Fatalf("write response: %v", err)
	}

	if got := rec.header.Get(HeaderAppliedIndex); got != "7" {
		t.Errorf("applied-index header: got %q want %q", got, "7")
	}

	var env ProposeForwardResponse
	if err := json.Unmarshal(rec.body, &env); err != nil {
		t.Fatalf("decode envelope: %v (body=%q)", err, rec.body)
	}

	if env.Index != 7 {
		t.Errorf("envelope index: got %d want 7", env.Index)
	}
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
