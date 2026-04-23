// Copyright 2026 Ella Networks

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	hraft "github.com/hashicorp/raft"
	"go.uber.org/zap"
)

// ClusterPropose is the leader side of follower→leader op forwarding.
//
// A follower POSTs a ProposeForwardRequest JSON envelope (operation
// name + payload). This handler validates leadership, dispatches the
// operation against the leader's own state (for changeset ops: apply
// + capture + raft.Apply; for intent ops: raft.Apply directly), and
// returns the committed index + FSM response so the follower can
// reconstruct a ProposeResult identical to what a local commit would
// have produced.
//
// Counterparts:
//
//   - follower side: (*raft.Manager).ForwardOperation in
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

		var envelope ellaraft.ProposeForwardRequest
		if err := json.Unmarshal(data, &envelope); err != nil {
			writeProposeForwardError(ctx, w, http.StatusBadRequest, "invalid envelope", err)
			return
		}

		if envelope.Operation == "" {
			writeProposeForwardError(ctx, w, http.StatusBadRequest, "empty operation", nil)
			return
		}

		if len(envelope.Payload) > 0 && !json.Valid(envelope.Payload) {
			writeProposeForwardError(ctx, w, http.StatusBadRequest, "invalid payload", nil)
			return
		}

		result, err := dbInstance.ApplyForwardedOperation(envelope.Operation, envelope.Payload)
		if err != nil {
			mapApplyErrorToHTTP(ctx, w, err)
			return
		}

		if err := ellaraft.WriteProposeForwardResponse(w, result); err != nil {
			logger.APILog.Error("cluster propose: failed to encode response", zap.Error(err))
		}
	})
}

// mapApplyErrorToHTTP classifies a raft Apply error into status codes
// the forwarder's retry logic expects: 421 on leadership change, 503 on
// raft busy/shutdown, 500 for anything else (the caller decides whether
// to retry the whole op — we must not silently retry here because the
// log entry may already be committed).
func mapApplyErrorToHTTP(ctx context.Context, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrUnknownOperation):
		writeProposeForwardError(ctx, w, http.StatusBadRequest,
			"unknown operation", err)

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
