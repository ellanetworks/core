// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"net"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

type trackedConn struct {
	state  http.ConnState
	opened time.Time
	since  time.Time
}

type trackedRequest struct {
	method    string
	path      string
	proto     string
	remote    string
	userAgent string
	start     time.Time
}

type connTracker struct {
	mu       sync.Mutex
	conns    map[net.Conn]trackedConn
	requests map[*http.Request]trackedRequest
}

func newConnTracker() *connTracker {
	return &connTracker{
		conns:    make(map[net.Conn]trackedConn),
		requests: make(map[*http.Request]trackedRequest),
	}
}

func (t *connTracker) connState(c net.Conn, state http.ConnState) {
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()

	switch state {
	case http.StateClosed, http.StateHijacked:
		delete(t.conns, c)
	case http.StateNew:
		t.conns[c] = trackedConn{state: state, opened: now, since: now}
	default:
		tc := t.conns[c]
		if tc.opened.IsZero() {
			tc.opened = now
		}

		tc.state = state
		tc.since = now
		t.conns[c] = tc
	}
}

func (t *connTracker) wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.mu.Lock()
		t.requests[r] = trackedRequest{
			method:    r.Method,
			path:      r.URL.Path,
			proto:     r.Proto,
			remote:    r.RemoteAddr,
			userAgent: r.UserAgent(),
			start:     time.Now(),
		}
		t.mu.Unlock()

		defer func() {
			t.mu.Lock()
			delete(t.requests, r)
			t.mu.Unlock()
		}()

		h.ServeHTTP(w, r)
	})
}

func (t *connTracker) logOpen(log *zap.Logger) {
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()

	log.Warn("API server still had open connections at the shutdown deadline",
		zap.Int("connections", len(t.conns)),
		zap.Int("requests_in_flight", len(t.requests)),
	)

	for c, tc := range t.conns {
		log.Warn("API connection still open at shutdown",
			zap.String("remote_address", c.RemoteAddr().String()),
			zap.String("local_address", c.LocalAddr().String()),
			zap.Stringer("state", tc.state),
			zap.Duration("age", now.Sub(tc.opened)),
			zap.Duration("in_state_for", now.Sub(tc.since)),
		)
	}

	for _, tr := range t.requests {
		log.Warn("API request still in flight at shutdown",
			zap.String("method", tr.method),
			zap.String("path", tr.path),
			zap.String("proto", tr.proto),
			zap.String("remote_address", tr.remote),
			zap.String("user_agent", tr.userAgent),
			zap.Duration("duration", now.Sub(tr.start)),
		)
	}
}
