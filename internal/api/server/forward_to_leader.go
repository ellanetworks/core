// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	hraft "github.com/hashicorp/raft"
	"go.opentelemetry.io/otel/trace"
)

const (
	InternalForwardPath = "/cluster/internal/forward"

	maxForwardEnvelopeBytes = 1 << 20
	forwardExecTimeout      = 30 * time.Second
	forwardAttemptTimeout   = forwardExecTimeout + 5*time.Second
	forwardApplyWait        = 2 * time.Second
)

var (
	leaderForwardHold    = 7 * time.Second
	leaderForwardBackoff = 200 * time.Millisecond
	forwardedHeaders     = []string{"Authorization", "Content-Type", "Accept"}
	relayedHeaders       = []string{"Content-Type", "Retry-After"}
	leaderForwardTarget  atomic.Pointer[http.Handler]
)

type leaderForwarder interface {
	ClusterEnabled() bool
	IsLeader() bool
	WaitApplied(ctx context.Context, index uint64) error
	WaitLogApplied(ctx context.Context, index uint64) error
	DoLeaderRequestWithTimeout(ctx context.Context, method, path string, body []byte, contentType string, timeout time.Duration) (*ellaraft.LeaderResponse, error)
}

type forwardEnvelope struct {
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Header   map[string]string `json:"header,omitempty"`
	Body     []byte            `json:"body,omitempty"`
	ClientIP string            `json:"clientIp"`
}

type forwardResult struct {
	Status   int               `json:"status"`
	Header   map[string]string `json:"header,omitempty"`
	Body     []byte            `json:"body,omitempty"`
	Index    uint64            `json:"index"`
	LogIndex uint64            `json:"logIndex"`
}

type forwardedRequest struct {
	peerNodeID string
	conn       net.Conn
	closeConn  atomic.Bool
}

type forwardedRequestCtxKey struct{}

func forwardedFromContext(ctx context.Context) *forwardedRequest {
	fr, _ := ctx.Value(forwardedRequestCtxKey{}).(*forwardedRequest)
	return fr
}

func SetLeaderForwardTarget(h http.Handler) {
	leaderForwardTarget.Store(&h)
}

func ForwardToLeader(dbInstance leaderForwarder, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !dbInstance.ClusterEnabled() || dbInstance.IsLeader() {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "Failed to read request body", err, logger.APILog)
			return
		}

		env := forwardEnvelope{
			Method:   r.Method,
			Path:     r.URL.RequestURI(),
			Header:   map[string]string{},
			Body:     body,
			ClientIP: getClientIP(r),
		}

		for _, name := range forwardedHeaders {
			if v := r.Header.Get(name); v != "" {
				env.Header[name] = v
			}
		}

		payload, err := json.Marshal(env)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to forward the request to the cluster leader", err, logger.APILog)
			return
		}

		deadline := time.Now().Add(leaderForwardHold)

		for {
			resp, err := dbInstance.DoLeaderRequestWithTimeout(ctx, http.MethodPost, InternalForwardPath, payload, "application/json", forwardAttemptTimeout)

			notApplied := errors.Is(err, hraft.ErrNotLeader) || errors.Is(err, ellaraft.ErrLeaderUnreachable) ||
				(err == nil && resp.StatusCode == http.StatusMisdirectedRequest)

			if notApplied && time.Now().Add(leaderForwardBackoff).Before(deadline) {
				select {
				case <-ctx.Done():
					return
				case <-time.After(leaderForwardBackoff):
				}

				continue
			}

			switch {
			case notApplied:
				writeError(ctx, w, http.StatusServiceUnavailable,
					"No reachable cluster leader; the request was not applied, retry shortly", err, logger.APILog)
			case errors.Is(err, ellaraft.ErrLeaderRequestNotSent):
				writeError(ctx, w, http.StatusInternalServerError,
					"Failed to forward the request to the cluster leader; the request was not applied", err, logger.APILog)
			case err != nil:
				writeError(ctx, w, http.StatusBadGateway,
					"Lost contact with the cluster leader; the outcome is unknown, check the cluster members before retrying", err, logger.APILog)
			case resp.StatusCode != http.StatusOK:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(resp.StatusCode)
				_, _ = w.Write(resp.Body)
			default:
				relayForwardResult(ctx, w, dbInstance, resp.Body)
			}

			return
		}
	})
}

func relayForwardResult(ctx context.Context, w http.ResponseWriter, dbInstance leaderForwarder, raw []byte) {
	var result forwardResult
	if err := json.Unmarshal(raw, &result); err != nil || result.Status == 0 {
		writeError(ctx, w, http.StatusBadGateway,
			"Invalid response from the cluster leader; the outcome is unknown, check the cluster members before retrying", err, logger.APILog)

		return
	}

	waitCtx, cancel := context.WithTimeout(ctx, forwardApplyWait)
	_ = dbInstance.WaitApplied(waitCtx, result.Index)
	_ = dbInstance.WaitLogApplied(waitCtx, result.LogIndex)

	cancel()

	for name, v := range result.Header {
		w.Header().Set(name, v)
	}

	w.WriteHeader(result.Status)
	_, _ = w.Write(result.Body)
}

type appliedIndexer interface {
	AppliedIndex() uint64
	CommitIndex() uint64
}

func ClusterForward(dbInstance appliedIndexer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		targetPtr := leaderForwardTarget.Load()
		if targetPtr == nil {
			writeError(ctx, w, http.StatusServiceUnavailable, "API not ready", nil, logger.APILog)
			return
		}

		data, err := io.ReadAll(io.LimitReader(r.Body, maxForwardEnvelopeBytes+1))
		if err != nil || len(data) > maxForwardEnvelopeBytes {
			writeError(ctx, w, http.StatusBadRequest, "Invalid forwarded request", err, logger.APILog)
			return
		}

		var env forwardEnvelope
		if err := json.Unmarshal(data, &env); err != nil || !strings.HasPrefix(env.Path, "/api/v1/") {
			writeError(ctx, w, http.StatusBadRequest, "Invalid forwarded request", err, logger.APILog)
			return
		}

		peerID, _ := peerNodeIDFromContext(ctx)
		fr := &forwardedRequest{peerNodeID: peerID, conn: clusterConnFromContext(ctx)}

		execCtx, cancel := context.WithTimeout(trace.ContextWithSpan(context.Background(), trace.SpanFromContext(ctx)), forwardExecTimeout)
		defer cancel()

		execCtx = context.WithValue(execCtx, forwardedRequestCtxKey{}, fr)

		req, err := http.NewRequestWithContext(execCtx, env.Method, env.Path, bytes.NewReader(env.Body))
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "Invalid forwarded request", err, logger.APILog)
			return
		}

		for name, v := range env.Header {
			req.Header.Set(name, v)
		}

		clientIP := env.ClientIP
		if net.ParseIP(clientIP) == nil {
			clientIP = "0.0.0.0"
		}

		req.RemoteAddr = net.JoinHostPort(clientIP, "0")

		rec := &forwardRecorder{header: http.Header{}}
		(*targetPtr).ServeHTTP(rec, req)

		result := forwardResult{Status: rec.status, Header: map[string]string{}, Body: rec.body.Bytes(), Index: dbInstance.AppliedIndex(), LogIndex: dbInstance.CommitIndex()}
		if result.Status == 0 {
			result.Status = http.StatusOK
		}

		for _, name := range relayedHeaders {
			if v := rec.header.Get(name); v != "" {
				result.Header[name] = v
			}
		}

		if fr.closeConn.Load() {
			w.Header().Set("Connection", "close")
		}

		writeRaw(ctx, w, result)
	})
}

func writeRaw(ctx context.Context, w http.ResponseWriter, v any) {
	payload, err := json.Marshal(v)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "Failed to encode the forwarded response", err, logger.APILog)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

type forwardRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (f *forwardRecorder) Header() http.Header { return f.header }

func (f *forwardRecorder) WriteHeader(status int) {
	if f.status == 0 {
		f.status = status
	}
}

func (f *forwardRecorder) Write(p []byte) (int, error) {
	if f.status == 0 {
		f.status = http.StatusOK
	}

	return f.body.Write(p)
}
