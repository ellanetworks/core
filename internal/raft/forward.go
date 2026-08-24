// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

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
	Code    string `json:"code,omitempty"`
}

const ForwardCodeOutcomeUnknown = "outcome_unknown"

var ErrOutcomeUnknown = errors.New("forwarded write outcome unknown")

type forwardAttemptFn func(ctx context.Context) (*ProposeResult, int, error)

// ForwardOperation posts a typed operation envelope to the current
// leader's /cluster/internal/propose endpoint and returns the committed
// ProposeResult. Retries only on unambiguous "didn't apply" signals
// (421, 503), never on network errors or 5xx, to avoid double-applying
// non-idempotent ops if a leader commit crossed with a lost response.
func (m *Manager) ForwardOperation(ctx context.Context, opName string, payload json.RawMessage, timeout time.Duration) (*ProposeResult, error) {
	if m.leaderClient == nil {
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

	lastErr := hraft.ErrNotLeader

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
		case http.StatusConflict:
			return nil, err

		case http.StatusMisdirectedRequest:
			lastErr = hraft.ErrNotLeader
			continue

		case http.StatusServiceUnavailable:
			lastErr = hraft.ErrNotLeader

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
	resp, err := m.leaderClient.do(ctx, leaderAddr, leaderID, leaderHTTPRequest{
		method:           http.MethodPost,
		path:             ProposeForwardPath,
		contentType:      ProposeForwardContentType,
		body:             data,
		maxResponseBytes: maxForwardResponseBytes,
	})
	if err != nil {
		if errors.Is(err, ErrLeaderUnreachable) {
			return nil, http.StatusServiceUnavailable, err
		}

		return nil, 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, decodeForwardError(resp.Body, resp.StatusCode)
	}

	var env ProposeForwardResponse
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		return nil, 0, fmt.Errorf("decode body: %w", err)
	}

	result := &ProposeResult{Index: env.Index}

	if len(env.Value) > 0 && !bytes.Equal(env.Value, []byte("null")) {
		// Preserve raw bytes; the typed-op dispatcher decodes into the
		// op's declared result type to avoid `any → float64` erasure.
		result.Value = env.Value
	}

	return result, http.StatusOK, nil
}

func decodeForwardError(body []byte, status int) error {
	var env ProposeForwardErrorBody
	if err := json.Unmarshal(body, &env); err == nil && env.Message != "" {
		if env.Code == ForwardCodeOutcomeUnknown {
			return fmt.Errorf("%w: %s", ErrOutcomeUnknown, env.Message)
		}

		return errors.New(env.Message)
	}

	if status == http.StatusConflict {
		return ErrOutcomeUnknown
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
	leaderAddr, leaderID := m.LeaderAddressAndID()
	if m.leaderClient == nil || leaderAddr == "" || leaderID == 0 {
		return nil, hraft.ErrNotLeader
	}

	resp, err := m.leaderClient.do(ctx, leaderAddr, leaderID, leaderHTTPRequest{
		method:           method,
		path:             path,
		contentType:      contentType,
		body:             body,
		maxResponseBytes: maxForwardResponseBytes,
		timeout:          m.ProposeTimeout(),
	})
	if err != nil {
		return nil, err
	}

	return &LeaderResponse{StatusCode: resp.StatusCode, Body: resp.Body}, nil
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
