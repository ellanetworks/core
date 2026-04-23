// Copyright 2026 Ella Networks

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	hraft "github.com/hashicorp/raft"
	"go.uber.org/zap"
)

// ClusterPropose is the leader side of follower→leader Propose forwarding.
//
// A follower POSTs pre-marshalled Command bytes here. This handler
// validates leadership, applies the command through Raft, and returns
// the committed index + FSM response so the follower can reconstruct a
// ProposeResult identical to what a local commit would have produced.
//
// Counterparts:
//
//   - follower side: (*raft.Manager).forwardPropose in
//     internal/raft/forward.go
//   - HTTP route registration: newClusterMux in cluster_http_mux.go
//     wraps this handler with removedNodeFence so a node removed from
//     cluster_members cannot re-enter the write path.
func ClusterPropose(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if !dbInstance.IsLeader() {
			writeProposeForwardError(ctx, w, http.StatusMisdirectedRequest,
				"not the leader; retry against the current leader", nil)

			return
		}

		if r.Body == nil {
			writeProposeForwardError(ctx, w, http.StatusBadRequest, "empty body", nil)
			return
		}

		data, err := io.ReadAll(io.LimitReader(r.Body, ellaraft.MaxProposeForwardBodyBytes+1))
		if err != nil {
			writeProposeForwardError(ctx, w, http.StatusBadRequest, "failed to read body", err)
			return
		}

		if int64(len(data)) > ellaraft.MaxProposeForwardBodyBytes {
			writeProposeForwardError(ctx, w, http.StatusRequestEntityTooLarge,
				"request body exceeds propose forward cap", nil)

			return
		}

		// Validate the command envelope BEFORE reaching raft.Apply.
		// The FSM is fail-stop: any error returned by applyCommand
		// panics the node. A peer with a valid mTLS cert but buggy
		// (or malicious) code could otherwise crash the leader by
		// forwarding a syntactically invalid Command. We reject obvious
		// garbage here; deeper schema errors still reach the FSM and
		// are surfaced to the forwarder as 500 rather than panics
		// (see raft.FSM.ApplyBatch for the fail-stop policy).
		if err := validateForwardedCommand(data); err != nil {
			writeProposeForwardError(ctx, w, http.StatusBadRequest, "invalid command", err)
			return
		}

		timeout := dbInstance.ProposeTimeout()

		result, err := dbInstance.ApplyForwardedCommand(data, timeout)
		if err != nil {
			mapApplyErrorToHTTP(ctx, w, err)
			return
		}

		if err := ellaraft.WriteProposeForwardResponse(w, result); err != nil {
			logger.APILog.Error("cluster propose: failed to encode response", zap.Error(err))
		}
	})
}

// validateForwardedCommand checks that the POST body is a well-formed
// Command envelope. Payload fields that deserialise according to the
// per-type schema are not validated here; those errors reach the FSM
// and surface as 500. The goal is to reject garbage (non-JSON payload,
// unknown command type) before raft.Apply would panic the node.
func validateForwardedCommand(data []byte) error {
	if len(data) < 2 {
		return errors.New("command too short")
	}

	cmd, err := ellaraft.UnmarshalCommand(data)
	if err != nil {
		return err
	}

	if !cmd.Type.IsKnown() {
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}

	if len(cmd.Payload) > 0 && !json.Valid(cmd.Payload) {
		return errors.New("command payload is not valid JSON")
	}

	return nil
}

// mapApplyErrorToHTTP classifies a raft Apply error into the status
// codes the forwarder's retry logic expects:
//
//   - ErrNotLeader / ErrLeadershipLost → 421 Misdirected Request (retry
//     against the new leader).
//   - ErrEnqueueTimeout / ErrRaftShutdown → 503 Service Unavailable
//     (retry after back-off — receiver may be busy or shutting down).
//   - anything else → 500 Internal Server Error, with the message in the
//     body so the forwarder can reconstruct the error for its caller.
//     The caller will decide whether to retry; we must not silently
//     retry here because the log entry may already be committed.
func mapApplyErrorToHTTP(ctx context.Context, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, hraft.ErrNotLeader), errors.Is(err, hraft.ErrLeadershipLost):
		writeProposeForwardError(ctx, w, http.StatusMisdirectedRequest,
			"leadership changed during apply; retry", err)

	case errors.Is(err, hraft.ErrEnqueueTimeout), errors.Is(err, hraft.ErrRaftShutdown):
		writeProposeForwardError(ctx, w, http.StatusServiceUnavailable,
			"raft busy or shutting down", err)

	default:
		writeProposeForwardError(ctx, w, http.StatusInternalServerError,
			"apply failed", err)
	}
}

// writeProposeForwardError writes a small {error: string} JSON body
// that the forwarder's decodeForwardError knows how to parse. Kept
// deliberately minimal — the operator-API writeError envelope pulls in
// tracing/span context that is irrelevant on the cluster port.
func writeProposeForwardError(ctx context.Context, w http.ResponseWriter, status int, message string, cause error) {
	if cause != nil {
		logger.APILog.Warn("cluster propose forward error",
			zap.Int("status", status),
			zap.String("message", message),
			zap.Error(cause))
	}

	body := ellaraft.ProposeForwardErrorBody{Message: message}
	if cause != nil {
		body.Message = message + ": " + cause.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.WithTrace(ctx, logger.APILog).Warn("cluster propose: failed to encode error body", zap.Error(err))
	}
}
