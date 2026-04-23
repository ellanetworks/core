// Copyright 2026 Ella Networks

package raft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/logger"
	hraft "github.com/hashicorp/raft"
	"go.uber.org/zap"
)

// Follower→leader forwarding for in-process Propose calls.
//
// Write-path parity between the two entry points Ella Core has to the
// replicated FSM:
//
//   1. Operator HTTP writes are caught by LeaderProxyMiddleware and
//      re-issued against the leader's /cluster/proxy/ mount.
//   2. In-process NF writes (AUSF seq-num, SMF IP lease, daily usage,
//      audit logs) call raftManager.Propose directly. On a follower,
//      raft.Apply returns ErrNotLeader. Before this file, that was a
//      hard failure; now Propose forwards to the leader over the same
//      mTLS cluster port, by POSTing the already-marshalled Command
//      bytes to /cluster/internal/propose.

const (
	// ProposeForwardPath is the cluster HTTP endpoint a follower POSTs
	// raw marshalled Command bytes to when forwarding Propose.
	ProposeForwardPath = "/cluster/internal/propose"

	// ProposeForwardContentType identifies the body as raw Command bytes
	// (2-byte CommandType header + JSON payload).
	ProposeForwardContentType = "application/octet-stream"

	// HeaderAppliedIndex mirrors the X-Ella-Applied-Index header the
	// operator-API proxy uses. The leader sets it to the committed log
	// index so the forwarder can wait for local apply before returning.
	HeaderAppliedIndex = "X-Ella-Applied-Index"

	// MaxProposeForwardBodyBytes caps the request body accepted by the
	// /cluster/internal/propose handler. Sized to accommodate bulk
	// changesets (retention deletes, migrations) without enabling abuse.
	MaxProposeForwardBodyBytes = 16 * 1024 * 1024

	// maxForwardAttempts caps retries on "didn't apply" signals (421 / 503).
	// Retrying on ambiguous failures (network errors, 5xx) is unsafe:
	// the leader may have committed the entry and a blind retry would
	// double-apply. The caller re-captures and retries for those.
	maxForwardAttempts = 3

	// noLeaderBackoff pauses between attempts when no leader is known.
	noLeaderBackoff = 200 * time.Millisecond

	// dialTimeout caps the mTLS dial per attempt.
	dialTimeout = 5 * time.Second

	// maxForwardResponseBytes caps the decoded response body. The body
	// is a small JSON envelope; anything larger is hostile.
	maxForwardResponseBytes = 64 * 1024
)

// appliedIndexWaitMax caps the read-your-writes wait on a forwarding
// follower. If the follower does not catch up within the window we
// still return success — subsequent reads on this node may briefly
// miss the write, matching the behaviour of the operator-API proxy.
// var rather than const so tests can tighten it.
var appliedIndexWaitMax = 2 * time.Second

// appliedIndexPollInterval is the poll cadence while waiting for local
// apply to catch up.
var appliedIndexPollInterval = 5 * time.Millisecond

// ProposeForwardResponse is the JSON envelope the leader's
// /cluster/internal/propose handler returns on 200. Kept symmetric with
// ProposeResult so the forwarder can reconstruct one directly.
type ProposeForwardResponse struct {
	Index uint64          `json:"index"`
	Value json.RawMessage `json:"value,omitempty"`
}

// ProposeForwardErrorBody is the JSON envelope for non-2xx responses.
// The forwarder reconstructs an ordinary Go error from Message.
type ProposeForwardErrorBody struct {
	Message string `json:"error"`
}

// forwardAttemptFn performs one forward round-trip: resolve the current
// leader, POST the Command bytes, and parse the response. Returns:
//
//   - (*ProposeResult, 200, nil) on a committed entry;
//   - (nil, status, err) for any non-2xx response where the handler
//     supplied a decoded error message;
//   - (nil, 0, err) for transport errors (including "no leader known"
//     which surfaces status 0 and a nil-leader sentinel error);
//   - (nil, 503, nil) as a dedicated "no leader" signal so the retry
//     loop can back off uniformly with real-server 503 responses.
type forwardAttemptFn func(ctx context.Context) (*ProposeResult, int, error)

// forwardPropose posts pre-marshalled Command bytes to the current leader
// and returns the committed ProposeResult. Retries only on unambiguous
// "didn't apply" signals (421, 503), never on network errors or 5xx, to
// avoid double-applying non-idempotent commands if a leader commit
// crossed with a lost response.
func (m *Manager) forwardPropose(ctx context.Context, data []byte, timeout time.Duration) (*ProposeResult, error) {
	if m.clusterListener == nil {
		return nil, hraft.ErrNotLeader
	}

	return m.runForwardRetryLoop(ctx, timeout, func(attemptCtx context.Context) (*ProposeResult, int, error) {
		leaderAddr, leaderID := m.LeaderAddressAndID()
		if leaderAddr == "" || leaderID == 0 {
			return nil, http.StatusServiceUnavailable, nil
		}

		return m.doForwardRequest(attemptCtx, leaderAddr, leaderID, data)
	})
}

// runForwardRetryLoop drives the attempt-retry logic against an injected
// attempt function. Extracted from forwardPropose so tests can exercise
// the retry semantics without standing up a full mTLS cluster.
func (m *Manager) runForwardRetryLoop(ctx context.Context, timeout time.Duration, attempt forwardAttemptFn) (*ProposeResult, error) {
	deadline := time.Now().Add(timeout)

	// Default classification: if we never reach a responsive leader the
	// caller sees ErrLeadershipLost, which isTransientRaftErr in the db
	// package already classifies as transient.
	lastErr := hraft.ErrLeadershipLost

	for range maxForwardAttempts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, lastErr
		}

		attemptCtx, cancel := context.WithTimeout(ctx, remaining)
		result, status, err := attempt(attemptCtx)

		cancel()

		if err == nil && status == http.StatusOK {
			m.waitForLocalApply(ctx, result.Index)

			return result, nil
		}

		switch status {
		case http.StatusMisdirectedRequest:
			// 421: receiver is no longer (or never was) the leader.
			// Re-resolve immediately and retry.
			lastErr = hraft.ErrLeadershipLost
			continue

		case http.StatusServiceUnavailable:
			// 503: leader unknown on the receiver side (or no leader
			// known locally). Back off and retry — election may be in
			// progress.
			lastErr = hraft.ErrLeadershipLost

			if err := waitOrDone(ctx, noLeaderBackoff); err != nil {
				return nil, err
			}

			continue
		}

		// Any other outcome is ambiguous or terminal. Do not retry:
		// a retry after a network failure could double-apply a
		// committed entry. The caller will decide whether to retry
		// the whole operation (which re-captures a fresh changeset).
		if err != nil {
			return nil, fmt.Errorf("forward propose: %w", err)
		}

		return nil, fmt.Errorf("forward propose: leader returned status %d", status)
	}

	return nil, lastErr
}

// doForwardRequest performs one POST to the leader's propose endpoint.
// Returns:
//
//   - (*ProposeResult, http.StatusOK, nil) on a committed entry;
//   - (nil, status, nil) when the leader returned a non-2xx we want to
//     classify (421/503 for retry logic, other statuses for error return);
//   - (nil, 503, err) when the mTLS dial failed — treated as a no-leader
//     signal so the retry loop backs off and re-resolves. A dial failure
//     is the one failure mode where we KNOW no bytes reached the leader,
//     so retrying is safe from a double-apply perspective;
//   - (nil, 0, err) on a post-dial transport or decoding error.
func (m *Manager) doForwardRequest(ctx context.Context, leaderAddr string, leaderID int, data []byte) (*ProposeResult, int, error) {
	conn, err := m.clusterListener.Dial(ctx, leaderAddr, leaderID, listener.ALPNHTTP, dialTimeout)
	if err != nil {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("dial leader: %w", err)
	}

	connUsed := false

	defer func() {
		if !connUsed {
			_ = conn.Close()
		}
	}()

	transport := &http.Transport{
		DialTLSContext: func(context.Context, string, string) (net.Conn, error) {
			if connUsed {
				return nil, errors.New("cluster HTTP transport: connection already consumed")
			}

			connUsed = true

			return conn, nil
		},
	}

	client := &http.Client{Transport: transport}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://"+leaderAddr+ProposeForwardPath, bytes.NewReader(data))
	if err != nil {
		return nil, 0, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", ProposeForwardContentType)
	req.ContentLength = int64(len(data))

	resp, err := client.Do(req) // #nosec G107 -- leaderAddr comes from Raft, not user input
	if err != nil {
		return nil, 0, fmt.Errorf("post: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxForwardResponseBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Non-2xx: the handler's contract is {error: string} on any
		// non-success. We only decode when the caller (the retry
		// switch in forwardPropose) didn't already classify the
		// status as transient-retry (421/503).
		return nil, resp.StatusCode, decodeForwardError(bodyBytes, resp.StatusCode)
	}

	var env ProposeForwardResponse
	if err := json.Unmarshal(bodyBytes, &env); err != nil {
		return nil, 0, fmt.Errorf("decode body: %w", err)
	}

	result := &ProposeResult{Index: env.Index}

	if len(env.Value) > 0 && !bytes.Equal(env.Value, []byte("null")) {
		var v any
		if err := json.Unmarshal(env.Value, &v); err != nil {
			return nil, 0, fmt.Errorf("decode result value: %w", err)
		}

		result.Value = v
	}

	return result, http.StatusOK, nil
}

// decodeForwardError extracts the error message from a non-2xx body.
// Returns a descriptive fallback when the body is missing or malformed
// so the forwarder never surfaces an empty error.
func decodeForwardError(body []byte, status int) error {
	var env ProposeForwardErrorBody
	if err := json.Unmarshal(body, &env); err == nil && env.Message != "" {
		return errors.New(env.Message)
	}

	return fmt.Errorf("leader returned status %d", status)
}

// waitForLocalApply polls the local applied index until it catches up
// to target, bounded by appliedIndexWaitMax. Returns without error on
// both catch-up and timeout — timeout is a soft condition, see the
// analogous helper in internal/api/server/proxy_middleware.go.
func (m *Manager) waitForLocalApply(ctx context.Context, target uint64) {
	deadline := time.Now().Add(appliedIndexWaitMax)

	for {
		if m.AppliedIndex() >= target {
			return
		}

		if !time.Now().Before(deadline) {
			logger.RaftLog.Warn(
				"forward propose: follower did not catch up to leader applied index before response",
				zap.Uint64("targetIdx", target),
				zap.Uint64("localIdx", m.AppliedIndex()),
			)

			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(appliedIndexPollInterval):
		}
	}
}

// waitOrDone sleeps for d or returns the context's error if it fires.
func waitOrDone(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// WriteProposeForwardResponse serialises a successful ProposeResult as the
// /cluster/internal/propose success body and sets the applied-index header.
func WriteProposeForwardResponse(w http.ResponseWriter, result *ProposeResult) error {
	env := ProposeForwardResponse{Index: result.Index}

	if result.Value != nil {
		raw, err := json.Marshal(result.Value)
		if err != nil {
			return fmt.Errorf("marshal value: %w", err)
		}

		env.Value = raw
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(HeaderAppliedIndex, strconv.FormatUint(env.Index, 10))
	w.WriteHeader(http.StatusOK)

	return json.NewEncoder(w).Encode(env)
}
