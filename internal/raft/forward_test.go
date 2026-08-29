// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	hraft "github.com/hashicorp/raft"
)

// scriptedAttempter returns a stable sequence of forwardAttemptFn
// responses to exercise the retry loop without HTTP or mTLS.
type scriptedAttempter struct {
	responses []attemptResponse
	calls     atomic.Int32
}

type attemptResponse struct {
	result *ProposeResult
	status int
	err    error
}

func (s *scriptedAttempter) fn() forwardAttemptFn {
	return func(context.Context) (*ProposeResult, int, error) {
		idx := int(s.calls.Add(1)) - 1
		if idx >= len(s.responses) {
			idx = len(s.responses) - 1
		}

		r := s.responses[idx]

		return r.result, r.status, r.err
	}
}

// newRetryLoopTestManager returns a Manager with just enough state to
// exercise runForwardRetryLoop. waitForLocalApply reads m.fsm which
// NewTestManager provides; other fields go unused. The applied-index
// wait is shortened so tests don't burn the full production budget on
// the unavoidable "we return an index the fake fsm never applies" case.
func newRetryLoopTestManager(t *testing.T) *Manager {
	t.Helper()

	origMax := appliedIndexWaitMax
	origPoll := appliedIndexPollInterval
	appliedIndexWaitMax = 20 * time.Millisecond
	appliedIndexPollInterval = time.Millisecond

	t.Cleanup(func() {
		appliedIndexWaitMax = origMax
		appliedIndexPollInterval = origPoll
	})

	m, _ := NewTestManager(t, newTestApplier(t))

	return m
}

func TestRunForwardRetryLoop_HappyPath(t *testing.T) {
	m := newRetryLoopTestManager(t)

	s := &scriptedAttempter{
		responses: []attemptResponse{
			{result: &ProposeResult{Index: 1, Value: "ok"}, status: http.StatusOK},
		},
	}

	result, err := m.runForwardRetryLoop(context.Background(), time.Second, s.fn())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil || result.Index != 1 || result.Value != "ok" {
		t.Fatalf("unexpected result: %+v", result)
	}

	if got := s.calls.Load(); got != 1 {
		t.Fatalf("expected 1 attempt, got %d", got)
	}
}

func TestRunForwardRetryLoop_421ThenOK(t *testing.T) {
	m := newRetryLoopTestManager(t)

	s := &scriptedAttempter{
		responses: []attemptResponse{
			{status: http.StatusMisdirectedRequest, err: errors.New("not leader")},
			{result: &ProposeResult{Index: 2}, status: http.StatusOK},
		},
	}

	result, err := m.runForwardRetryLoop(context.Background(), time.Second, s.fn())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Index != 2 {
		t.Fatalf("unexpected index: %d", result.Index)
	}

	if got := s.calls.Load(); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

func TestRunForwardRetryLoop_503ThenOK(t *testing.T) {
	m := newRetryLoopTestManager(t)

	s := &scriptedAttempter{
		responses: []attemptResponse{
			{status: http.StatusServiceUnavailable, err: errors.New("no leader")},
			{result: &ProposeResult{Index: 3}, status: http.StatusOK},
		},
	}

	start := time.Now()

	result, err := m.runForwardRetryLoop(context.Background(), time.Second, s.fn())

	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Index != 3 {
		t.Fatalf("unexpected index: %d", result.Index)
	}

	// 503 must back off (noLeaderBackoff) before retry — the test
	// guards against accidentally flipping to immediate-retry, which
	// would hot-loop under a persistent no-leader condition.
	if elapsed < noLeaderBackoff {
		t.Fatalf("503 did not back off: elapsed=%v want>=%v", elapsed, noLeaderBackoff)
	}
}

func TestRunForwardRetryLoop_500IsTerminal(t *testing.T) {
	m := newRetryLoopTestManager(t)

	s := &scriptedAttempter{
		responses: []attemptResponse{
			{status: http.StatusInternalServerError, err: errors.New("apply failed: boom")},
		},
	}

	result, err := m.runForwardRetryLoop(context.Background(), time.Second, s.fn())
	if err == nil {
		t.Fatal("expected error on 500")
	}

	if result != nil {
		t.Fatalf("expected nil result on error, got %+v", result)
	}

	// Must surface the error from the leader verbatim — the caller
	// needs to see the real cause (e.g. a specific FSM failure) to
	// decide whether to retry the whole operation.
	if !contains(err.Error(), "apply failed: boom") {
		t.Fatalf("error should mention underlying cause: %v", err)
	}

	// 500 must be terminal — no retry, because a retry after an
	// ambiguous failure could double-apply a committed entry.
	if got := s.calls.Load(); got != 1 {
		t.Fatalf("expected 1 attempt on 500, got %d", got)
	}
}

func TestRunForwardRetryLoop_TransportErrorIsTerminal(t *testing.T) {
	m := newRetryLoopTestManager(t)

	s := &scriptedAttempter{
		responses: []attemptResponse{
			{status: 0, err: errors.New("connection reset")},
		},
	}

	_, err := m.runForwardRetryLoop(context.Background(), time.Second, s.fn())
	if err == nil {
		t.Fatal("expected error on transport failure")
	}

	// Same safety argument as 500: don't auto-retry.
	if got := s.calls.Load(); got != 1 {
		t.Fatalf("expected 1 attempt on transport error, got %d", got)
	}
}

func TestRunForwardRetryLoop_MaxAttemptsExhausted(t *testing.T) {
	m := newRetryLoopTestManager(t)

	s := &scriptedAttempter{
		responses: []attemptResponse{
			{status: http.StatusMisdirectedRequest, err: errors.New("not leader")},
		},
	}

	_, err := m.runForwardRetryLoop(context.Background(), time.Second, s.fn())
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}

	if !errors.Is(err, hraft.ErrNotLeader) {
		t.Fatalf("want ErrNotLeader, got %v", err)
	}

	if errors.Is(err, ErrOutcomeUnknown) {
		t.Fatal("exhausted retries must not report an unknown outcome")
	}

	if got := s.calls.Load(); got != int32(maxForwardAttempts) {
		t.Fatalf("expected %d attempts, got %d", maxForwardAttempts, got)
	}
}

func TestRunForwardRetryLoop_ContextCancelled(t *testing.T) {
	m := newRetryLoopTestManager(t)

	s := &scriptedAttempter{
		responses: []attemptResponse{
			{status: http.StatusServiceUnavailable},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled

	_, err := m.runForwardRetryLoop(ctx, time.Second, s.fn())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}

	if got := s.calls.Load(); got != 0 {
		t.Fatalf("expected 0 attempts with pre-cancelled context, got %d", got)
	}
}

func TestRunForwardRetryLoop_TimeoutBounds(t *testing.T) {
	m := newRetryLoopTestManager(t)

	s := &scriptedAttempter{
		responses: []attemptResponse{
			{status: http.StatusServiceUnavailable},
		},
	}

	const timeout = 50 * time.Millisecond

	start := time.Now()

	_, err := m.runForwardRetryLoop(context.Background(), timeout, s.fn())

	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error after timeout")
	}

	if !errors.Is(err, hraft.ErrNotLeader) {
		t.Errorf("error: got %v, want hraft.ErrNotLeader so the caller may safely retry", err)
	}

	if errors.Is(err, ErrOutcomeUnknown) {
		t.Errorf("a 503 means the entry was not applied; must not surface as outcome-unknown: %v", err)
	}

	if got := s.calls.Load(); got != 1 {
		t.Errorf("attempts: got %d, want 1 (the deadline must stop the loop before the budget does)", got)
	}

	if upper := timeout + 2*noLeaderBackoff; elapsed > upper {
		t.Errorf("loop ran for %v, want at most %v (timeout + one backoff plus slack)", elapsed, upper)
	}
}

func TestDecodeForwardError_ValidBody(t *testing.T) {
	body := []byte(`{"error":"apply failed: conflict"}`)

	err := decodeForwardError(body, http.StatusInternalServerError)
	if err == nil || err.Error() != "apply failed: conflict" {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestDecodeForwardError_MalformedFallsBack(t *testing.T) {
	err := decodeForwardError([]byte("not json"), http.StatusBadGateway)
	if err == nil {
		t.Fatal("expected fallback error")
	}

	if !contains(err.Error(), "502") {
		t.Fatalf("fallback should include status: %v", err)
	}
}

func TestDecodeForwardError_EmptyMessage(t *testing.T) {
	// Valid JSON but empty Message field: must still produce a
	// descriptive error, not an empty string — empty errors confuse
	// both callers and observability.
	err := decodeForwardError([]byte(`{"error":""}`), http.StatusInternalServerError)
	if err == nil || err.Error() == "" {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestForwardOperation_SingleServerReturnsNotLeader(t *testing.T) {
	// In standalone mode (no clusterListener) forwarding is not
	// possible. ForwardOperation is called from the typed-op dispatch
	// layer only when leader-side apply failed, which in single-server
	// mode means something is very wrong; surface ErrNotLeader rather
	// than NPE.
	m, _ := NewTestManager(t, newTestApplier(t))

	if m.clusterListener != nil {
		t.Fatal("NewTestManager unexpectedly wired a clusterListener")
	}

	_, err := m.ForwardOperation(context.Background(), "TestOp", []byte(`{}`), time.Second)
	if !errors.Is(err, hraft.ErrNotLeader) {
		t.Fatalf("want ErrNotLeader, got %v", err)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

func TestRunForwardRetryLoop_OutcomeUnknownIsTerminal(t *testing.T) {
	m := newRetryLoopTestManager(t)

	s := &scriptedAttempter{
		responses: []attemptResponse{
			{status: http.StatusConflict, err: fmt.Errorf("%w: leadership lost after propose", ErrOutcomeUnknown)},
			{status: http.StatusOK, result: &ProposeResult{Index: 9}},
		},
	}

	_, err := m.runForwardRetryLoop(context.Background(), time.Second, s.fn())
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("want ErrOutcomeUnknown, got %v", err)
	}

	if got := s.calls.Load(); got != 1 {
		t.Fatalf("an ambiguous outcome must not be retried; got %d attempts", got)
	}
}

func TestDecodeForwardError_OutcomeUnknownCode(t *testing.T) {
	body, err := json.Marshal(ProposeForwardErrorBody{
		Message: "leadership lost after propose",
		Code:    ForwardCodeOutcomeUnknown,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if got := decodeForwardError(body, http.StatusConflict); !errors.Is(got, ErrOutcomeUnknown) {
		t.Fatalf("want ErrOutcomeUnknown, got %v", got)
	}
}

func TestDecodeForwardError_NoCodeStaysPlain(t *testing.T) {
	body, err := json.Marshal(ProposeForwardErrorBody{Message: "not leader; nothing was applied, retry"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if got := decodeForwardError(body, http.StatusMisdirectedRequest); errors.Is(got, ErrOutcomeUnknown) {
		t.Fatalf("retryable error must not decode as outcome-unknown: %v", got)
	}
}

func TestDoForwardRequest_PostSendFailureIsOutcomeUnknown(t *testing.T) {
	addr := startClusterHTTPStub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)

		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}

		conn, _, err := hj.Hijack()
		if err != nil {
			return
		}

		_ = conn.Close()
	}))

	m := newRetryLoopTestManager(t)
	m.leaderClient = newLeaderHTTPClient(func(ctx context.Context, a string, _ int) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", a)
	})

	t.Cleanup(m.leaderClient.close)

	_, status, err := m.doForwardRequest(context.Background(), addr, 1, []byte(`{}`))
	if err == nil {
		t.Fatal("expected an error when the leader hangs up mid-request")
	}

	if status != 0 {
		t.Fatalf("post-send failure should carry no leader status, got %d", status)
	}

	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("a request that reached the wire may have committed and must be outcome-unknown, got %v", err)
	}
}

func TestDoForwardRequest_DialFailureIsRetryableNotOutcomeUnknown(t *testing.T) {
	m := newRetryLoopTestManager(t)
	m.leaderClient = newLeaderHTTPClient(func(context.Context, string, int) (net.Conn, error) {
		return nil, errors.New("dial refused")
	})

	t.Cleanup(m.leaderClient.close)

	_, status, err := m.doForwardRequest(context.Background(), "10.0.0.1:7000", 1, []byte(`{}`))
	if err == nil {
		t.Fatal("expected an error when the leader cannot be dialled")
	}

	if status != http.StatusServiceUnavailable {
		t.Fatalf("dial failure should map to 503, got %d", status)
	}

	if errors.Is(err, ErrOutcomeUnknown) {
		t.Fatal("nothing was sent on a dial failure, so the write must stay retryable rather than outcome-unknown")
	}
}
