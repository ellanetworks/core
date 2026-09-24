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
	"time"

	"github.com/ellanetworks/core/internal/logger"
	hraft "github.com/hashicorp/raft"
	"go.uber.org/zap"
)

// Follower-to-leader forwarding preserves write-path parity by sending typed
// operation intent to the leader, which applies the command, captures the
// resulting SQLite changeset, and proposes it through Raft. Followers never
// capture changesets because row-level deltas depend on the exact base state
// that produced them.

const (
	// ProposeForwardPath is the cluster HTTP endpoint a follower POSTs
	// a typed operation envelope to when forwarding.
	ProposeForwardPath = "/cluster/internal/propose"

	// ProposeForwardContentType identifies the body as a
	// ProposeForwardRequest JSON envelope.
	ProposeForwardContentType = "application/json"

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

const (
	ForwardCodeOutcomeUnknown = "outcome_unknown"
	ForwardCodeNotFound       = "not_found"
	ForwardCodeAlreadyExists  = "already_exists"
	ForwardCodeMigrationPend  = "migration_pending"
	ForwardCodeTokenConsumed  = "join_token_consumed"      // #nosec G101 -- response code, not a credential
	ForwardCodeTokenExpired   = "join_token_expired"       // #nosec G101 -- response code, not a credential
	ForwardCodeTokenNodeMism  = "join_token_node_mismatch" // #nosec G101 -- response code, not a credential
)

type ForwardCodedError struct {
	Code    string
	Message string
}

func (e *ForwardCodedError) Error() string { return e.Message }

func ForwardErrorCode(err error) string {
	var coded *ForwardCodedError
	if errors.As(err, &coded) {
		return coded.Code
	}

	return ""
}

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
		if leaderAddr == "" || leaderID == "" {
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
			return nil, classifyForwardDeadline(lastErr, err)
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

		case http.StatusMisdirectedRequest, http.StatusServiceUnavailable:
			if ForwardErrorCode(err) != "" {
				return nil, err
			}

			lastErr = hraft.ErrNotLeader

			if err := waitOrDone(ctx, noLeaderBackoff); err != nil {
				return nil, classifyForwardDeadline(lastErr, err)
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

func (m *Manager) doForwardRequest(ctx context.Context, leaderAddr string, leaderID string, data []byte) (*ProposeResult, int, error) {
	resp, err := m.leaderClient.do(ctx, leaderAddr, leaderID, leaderHTTPRequest{
		method:           http.MethodPost,
		path:             ProposeForwardPath,
		contentType:      ProposeForwardContentType,
		body:             data,
		maxResponseBytes: maxForwardResponseBytes,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrLeaderUnreachable):
			return nil, http.StatusServiceUnavailable, err

		case errors.Is(err, ErrLeaderRequestNotSent):
			return nil, 0, err

		default:
			return nil, 0, fmt.Errorf("%w: %w", ErrOutcomeUnknown, err)
		}
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

		if env.Code != "" {
			return &ForwardCodedError{Code: env.Code, Message: env.Message}
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
				zap.Uint64("target_idx", target),
				zap.Uint64("local_idx", m.AppliedIndex()),
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

// classifyForwardDeadline keeps a caller-context expiry from erasing what
// the loop already knows. Both call sites are reached only after an attempt
// reported a retryable status, so no entry was applied and lastErr carries
// the right classification; the context error rides along for diagnostics.
// Cancellation is propagated untouched — the caller went away.
func classifyForwardDeadline(lastErr, ctxErr error) error {
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", lastErr, ctxErr)
	}

	return ctxErr
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
	if m.leaderClient == nil || leaderAddr == "" || leaderID == "" {
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
// /cluster/internal/propose success body.
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
	w.WriteHeader(http.StatusOK)

	return json.NewEncoder(w).Encode(env)
}
