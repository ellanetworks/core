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

// Follower→leader forwarding for in-process replicated writes.
//
// Write-path parity between the entry points Ella Core has to the
// replicated FSM:
//
//   1. Operator HTTP writes are caught by LeaderProxyMiddleware and
//      re-issued against the leader's /cluster/proxy/ mount.
//   2. In-process replicated writes (NF code, audit logs, bulk deletes,
//      migrations) call typed-op Invoke helpers in internal/db. On a
//      follower, the helper forwards (operation name, payload JSON)
//      to the leader's /cluster/internal/propose endpoint. The
//      leader's handler dispatches to the same apply function a local
//      caller would, captures the resulting SQLite changeset against
//      leader state, and proposes it through Raft.
//
// The follower never captures: captures encode row-level deltas against
// a specific base state (auto-increment IDs, UPDATE before-images,
// UPSERT-resolved values, default-expression results), and those deltas
// are only valid when applied against the state that produced them.
// Shipping the operation intent (a typed command) rather than the
// captured bytes keeps replication correct under leader changes,
// cross-version skew, and fresh-boot state divergence.

const (
	// ProposeForwardPath is the cluster HTTP endpoint a follower POSTs
	// a typed operation envelope to when forwarding.
	ProposeForwardPath = "/cluster/internal/propose"

	// ProposeForwardContentType identifies the body as a
	// ProposeForwardRequest JSON envelope.
	ProposeForwardContentType = "application/json"

	// HeaderAppliedIndex mirrors the X-Ella-Applied-Index header the
	// operator-API proxy uses. The leader sets it to the committed log
	// index so the forwarder can wait for local apply before returning.
	HeaderAppliedIndex = "X-Ella-Applied-Index"

	// MaxProposeForwardBodyBytes caps the request body accepted by the
	// /cluster/internal/propose handler. Sized for bulk-payload ops
	// (BGP prefix sets, bootstrap envelopes) without enabling abuse.
	MaxProposeForwardBodyBytes = 16 * 1024 * 1024

	// maxForwardAttempts caps retries on "didn't apply" signals (421 / 503).
	// Retrying on ambiguous failures (network errors, 5xx) is unsafe:
	// the leader may have committed the entry and a blind retry would
	// double-apply. Non-idempotent ops surface the error to the caller
	// which decides whether to retry the whole operation.
	maxForwardAttempts = 3

	noLeaderBackoff         = 200 * time.Millisecond
	dialTimeout             = 5 * time.Second
	maxForwardResponseBytes = 64 * 1024
)

var (
	appliedIndexWaitMax      = 2 * time.Second
	appliedIndexPollInterval = 5 * time.Millisecond
)

// ProposeForwardRequest is the JSON envelope a follower sends to the
// leader's /cluster/internal/propose endpoint. The leader dispatches
// Operation through its registered op table, re-hydrates Payload into
// the typed struct the apply function expects, and runs the apply +
// capture + propose cycle against its own state.
type ProposeForwardRequest struct {
	Operation string          `json:"operation"`
	Payload   json.RawMessage `json:"payload"`
}

// ProposeForwardResponse is the JSON envelope the leader returns on
// 200 commit. Kept symmetric with ProposeResult so the forwarder can
// reconstruct one directly.
type ProposeForwardResponse struct {
	Index uint64          `json:"index"`
	Value json.RawMessage `json:"value,omitempty"`
}

// ProposeForwardErrorBody is the JSON envelope for non-2xx responses.
type ProposeForwardErrorBody struct {
	Message string `json:"error"`
}

type forwardAttemptFn func(ctx context.Context) (*ProposeResult, int, error)

// ForwardOperation posts a typed operation envelope to the current
// leader's /cluster/internal/propose endpoint and returns the committed
// ProposeResult. Retries only on unambiguous "didn't apply" signals
// (421, 503), never on network errors or 5xx, to avoid double-applying
// non-idempotent ops if a leader commit crossed with a lost response.
func (m *Manager) ForwardOperation(ctx context.Context, opName string, payload json.RawMessage, timeout time.Duration) (*ProposeResult, error) {
	if m.clusterListener == nil {
		return nil, hraft.ErrNotLeader
	}

	envelope, err := json.Marshal(ProposeForwardRequest{Operation: opName, Payload: payload})
	if err != nil {
		return nil, fmt.Errorf("marshal forward envelope: %w", err)
	}

	return m.runForwardRetryLoop(ctx, timeout, func(attemptCtx context.Context) (*ProposeResult, int, error) {
		leaderAddr, leaderID := m.LeaderAddressAndID()
		if leaderAddr == "" || leaderID == 0 {
			return nil, http.StatusServiceUnavailable, nil
		}

		return m.doForwardRequest(attemptCtx, leaderAddr, leaderID, envelope)
	})
}

func (m *Manager) runForwardRetryLoop(ctx context.Context, timeout time.Duration, attempt forwardAttemptFn) (*ProposeResult, error) {
	deadline := time.Now().Add(timeout)

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
			lastErr = hraft.ErrLeadershipLost
			continue

		case http.StatusServiceUnavailable:
			lastErr = hraft.ErrLeadershipLost

			if err := waitOrDone(ctx, noLeaderBackoff); err != nil {
				return nil, err
			}

			continue
		}

		if err != nil {
			return nil, fmt.Errorf("forward operation: %w", err)
		}

		return nil, fmt.Errorf("forward operation: leader returned status %d", status)
	}

	return nil, lastErr
}

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

func decodeForwardError(body []byte, status int) error {
	var env ProposeForwardErrorBody
	if err := json.Unmarshal(body, &env); err == nil && env.Message != "" {
		return errors.New(env.Message)
	}

	return fmt.Errorf("leader returned status %d", status)
}

func (m *Manager) waitForLocalApply(ctx context.Context, target uint64) {
	deadline := time.Now().Add(appliedIndexWaitMax)

	for {
		if m.AppliedIndex() >= target {
			return
		}

		if !time.Now().Before(deadline) {
			logger.RaftLog.Warn(
				"forward operation: follower did not catch up to leader applied index before response",
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

// LeaderResponse is the result of a one-shot HTTP round-trip against
// the current leader's cluster mTLS port via LeaderRequest.
type LeaderResponse struct {
	StatusCode int
	Body       []byte
}

// LeaderRequest performs a single HTTP request against the current
// leader's cluster mTLS port and returns the response. Used by
// follower-side handlers that read state which only exists on the
// leader (autopilot live state, etc.).
//
// Returns hraft.ErrNotLeader when no leader is currently known. The
// caller is responsible for retry semantics; this helper does not
// retry because the calls it serves are idempotent reads where a
// transient miss is preferable to amplifying load on a flapping
// leader.
func (m *Manager) LeaderRequest(ctx context.Context, method, path string, body []byte, contentType string) (*LeaderResponse, error) {
	if m.clusterListener == nil {
		return nil, hraft.ErrNotLeader
	}

	leaderAddr, leaderID := m.LeaderAddressAndID()
	if leaderAddr == "" || leaderID == 0 {
		return nil, hraft.ErrNotLeader
	}

	conn, err := m.clusterListener.Dial(ctx, leaderAddr, leaderID, listener.ALPNHTTP, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("dial leader: %w", err)
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

	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, "https://"+leaderAddr+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	if len(body) > 0 {
		req.ContentLength = int64(len(body))
	}

	resp, err := client.Do(req) // #nosec G107 -- leaderAddr comes from Raft, not user input
	if err != nil {
		return nil, fmt.Errorf("post: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxForwardResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return &LeaderResponse{StatusCode: resp.StatusCode, Body: bodyBytes}, nil
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
