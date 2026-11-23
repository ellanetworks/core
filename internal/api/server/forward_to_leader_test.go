// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	hraft "github.com/hashicorp/raft"
)

type fakeForwarder struct {
	enabled   bool
	leader    bool
	do        func(call int, body []byte) (*ellaraft.LeaderResponse, error)
	calls     int
	payloads  [][]byte
	waited    uint64
	waitedLog uint64
}

func (f *fakeForwarder) ClusterEnabled() bool { return f.enabled }
func (f *fakeForwarder) IsLeader() bool       { return f.leader }

func (f *fakeForwarder) WaitApplied(_ context.Context, index uint64) error {
	f.waited = index
	return nil
}

func (f *fakeForwarder) WaitLogApplied(_ context.Context, index uint64) error {
	f.waitedLog = index
	return nil
}

func (f *fakeForwarder) DoLeaderRequestWithTimeout(_ context.Context, method, path string, body []byte, _ string, _ time.Duration) (*ellaraft.LeaderResponse, error) {
	if method != http.MethodPost || path != InternalForwardPath {
		return nil, errors.New("unexpected leader request " + method + " " + path)
	}

	f.calls++
	f.payloads = append(f.payloads, body)

	return f.do(f.calls, body)
}

type fakeIndexer uint64

func (f fakeIndexer) AppliedIndex() uint64 { return uint64(f) }
func (f fakeIndexer) CommitIndex() uint64  { return uint64(f) + 100 }

func withFastForwardRetries(t *testing.T) {
	t.Helper()

	hold, backoff := leaderForwardHold, leaderForwardBackoff
	leaderForwardHold, leaderForwardBackoff = 50*time.Millisecond, time.Millisecond

	t.Cleanup(func() { leaderForwardHold, leaderForwardBackoff = hold, backoff })
}

func withForwardTarget(t *testing.T, h http.Handler) {
	t.Helper()

	prev := leaderForwardTarget.Load()

	SetLeaderForwardTarget(h)

	t.Cleanup(func() { leaderForwardTarget.Store(prev) })
}

func followerRequest(t *testing.T, f *fakeForwarder) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/v1/cluster/members/abc?force=true", strings.NewReader(`{"x":1}`))
	req.RemoteAddr = "192.0.2.10:4321"
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Other", "dropped")

	rec := httptest.NewRecorder()

	ForwardToLeader(f, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("follower handled the request locally")
	})).ServeHTTP(rec, req)

	return rec
}

func resultResponse(t *testing.T, result forwardResult) *ellaraft.LeaderResponse {
	t.Helper()

	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	return &ellaraft.LeaderResponse{StatusCode: http.StatusOK, Body: body}
}

func TestForwardToLeaderServesLocally(t *testing.T) {
	for _, f := range []*fakeForwarder{{enabled: false}, {enabled: true, leader: true}} {
		served := false

		ForwardToLeader(f, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served = true })).
			ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/cluster/members/abc/promote", nil))

		if !served || f.calls != 0 {
			t.Fatalf("served=%v calls=%d, want local handling", served, f.calls)
		}
	}
}

func TestForwardToLeaderSendsTheOriginalRequest(t *testing.T) {
	f := &fakeForwarder{enabled: true, do: func(int, []byte) (*ellaraft.LeaderResponse, error) {
		return resultResponse(t, forwardResult{
			Status:   http.StatusConflict,
			Header:   map[string]string{"Content-Type": "application/json"},
			Body:     []byte(`{"error":"Node is not drained"}`),
			Index:    42,
			LogIndex: 43,
		}), nil
	}}

	rec := followerRequest(t, f)

	if rec.Code != http.StatusConflict || rec.Body.String() != `{"error":"Node is not drained"}` || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("got %d %q %v, want the leader's response", rec.Code, rec.Body.String(), rec.Header())
	}

	if f.waited != 42 || f.waitedLog != 43 {
		t.Fatalf("waited for indexes %d/%d, want 42/43", f.waited, f.waitedLog)
	}

	var env forwardEnvelope
	if err := json.Unmarshal(f.payloads[0], &env); err != nil {
		t.Fatal(err)
	}

	if env.Method != http.MethodDelete || env.Path != "/api/v1/cluster/members/abc?force=true" || string(env.Body) != `{"x":1}` || env.ClientIP != "192.0.2.10" {
		t.Fatalf("envelope = %+v", env)
	}

	if env.Header["Authorization"] != "Bearer token" || env.Header["Content-Type"] != "application/json" || env.Header["X-Other"] != "" {
		t.Fatalf("envelope headers = %v", env.Header)
	}
}

func TestForwardToLeaderRetriesWhenNotApplied(t *testing.T) {
	withFastForwardRetries(t)

	f := &fakeForwarder{enabled: true, do: func(call int, _ []byte) (*ellaraft.LeaderResponse, error) {
		switch call {
		case 1:
			return nil, hraft.ErrNotLeader
		case 2:
			return nil, ellaraft.ErrLeaderUnreachable
		case 3:
			return &ellaraft.LeaderResponse{StatusCode: http.StatusMisdirectedRequest, Body: []byte(`{}`)}, nil
		default:
			return resultResponse(t, forwardResult{Status: http.StatusOK, Body: []byte(`{"result":{}}`)}), nil
		}
	}}

	rec := followerRequest(t, f)
	if rec.Code != http.StatusOK || f.calls != 4 {
		t.Fatalf("code=%d calls=%d, want 200 after 4 attempts", rec.Code, f.calls)
	}
}

func TestForwardToLeaderFailures(t *testing.T) {
	withFastForwardRetries(t)

	for _, tc := range []struct {
		name     string
		resp     *ellaraft.LeaderResponse
		err      error
		retried  bool
		wantCode int
		wantText string
	}{
		{name: "no leader for the whole hold", err: hraft.ErrNotLeader, retried: true, wantCode: http.StatusServiceUnavailable, wantText: "was not applied"},
		{name: "not sent", err: ellaraft.ErrLeaderRequestNotSent, wantCode: http.StatusInternalServerError, wantText: "was not applied"},
		{name: "lost after sending", err: errors.New("connection reset"), wantCode: http.StatusBadGateway, wantText: "outcome is unknown"},
		{name: "invalid result", resp: &ellaraft.LeaderResponse{StatusCode: http.StatusOK, Body: []byte("nope")}, wantCode: http.StatusBadGateway, wantText: "outcome is unknown"},
		{name: "leader rejects the forward", resp: &ellaraft.LeaderResponse{StatusCode: http.StatusGone, Body: []byte(`{"error":"gone"}`)}, wantCode: http.StatusGone, wantText: "gone"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeForwarder{enabled: true, do: func(int, []byte) (*ellaraft.LeaderResponse, error) { return tc.resp, tc.err }}

			rec := followerRequest(t, f)
			if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantText) {
				t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
			}

			if (f.calls > 1) != tc.retried {
				t.Fatalf("calls = %d, retried want %v", f.calls, tc.retried)
			}
		})
	}
}

func leaderForward(t *testing.T, env forwardEnvelope, ctx context.Context) (*httptest.ResponseRecorder, forwardResult) {
	t.Helper()

	payload, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequestWithContext(ctx, http.MethodPost, InternalForwardPath, bytes.NewReader(payload))
	rec := httptest.NewRecorder()

	ClusterForward(fakeIndexer(7)).ServeHTTP(rec, req)

	var result forwardResult
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
	}

	return rec, result
}

func TestClusterForwardReplaysAsTheOriginalCaller(t *testing.T) {
	var (
		actor, ip, auth, body string
		forwarded             bool
	)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/cluster/members/{id}", func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), contextKeyEmail, "admin@example.com"))
		actor, ip, auth = getActorFromContext(r), getClientIP(r), r.Header.Get("Authorization")
		forwarded = forwardedFromContext(r.Context()) != nil

		b, _ := io.ReadAll(r.Body)
		body = string(b)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Internal", "dropped")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"result":{}}`))
	})
	withForwardTarget(t, mux)

	ctx := context.WithValue(t.Context(), peerNodeIDCtxKey{}, "node-2")

	rec, result := leaderForward(t, forwardEnvelope{
		Method: http.MethodDelete, Path: "/api/v1/cluster/members/abc?force=true",
		Header: map[string]string{"Authorization": "Bearer token"}, Body: []byte("payload"), ClientIP: "192.0.2.10",
	}, ctx)

	if rec.Code != http.StatusOK || result.Status != http.StatusAccepted || string(result.Body) != `{"result":{}}` || result.Index != 7 || result.LogIndex != 107 {
		t.Fatalf("code=%d result=%+v", rec.Code, result)
	}

	if result.Header["Content-Type"] != "application/json" || result.Header["X-Internal"] != "" {
		t.Fatalf("relayed headers = %v", result.Header)
	}

	if actor != "admin@example.com" || ip != "192.0.2.10" || auth != "Bearer token" || body != "payload" || !forwarded {
		t.Fatalf("actor=%q ip=%q auth=%q body=%q forwarded=%v", actor, ip, auth, body, forwarded)
	}

	if rec.Header().Get("Connection") == "close" {
		t.Fatal("connection closed without a self-removal")
	}
}

func TestClusterForwardOnlyServesLeaderRoutes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/cluster/members/{id}/promote", func(http.ResponseWriter, *http.Request) {})
	withForwardTarget(t, mux)

	_, result := leaderForward(t, forwardEnvelope{Method: http.MethodGet, Path: "/api/v1/subscribers"}, t.Context())
	if result.Status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a route the leader does not accept forwarded", result.Status)
	}

	for _, path := range []string{"/cluster/internal/propose", "http://evil/api/v1/x", ""} {
		rec, _ := leaderForward(t, forwardEnvelope{Method: http.MethodPost, Path: path}, t.Context())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("path %q: code = %d, want 400", path, rec.Code)
		}
	}
}

func TestClusterForwardClosesTheConnectionOfARemovedPeer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/cluster/members/{id}", func(_ http.ResponseWriter, r *http.Request) {
		fr := forwardedFromContext(r.Context())
		if fr.peerNodeID == r.PathValue("id") {
			fr.closeConn.Store(true)
		}
	})
	withForwardTarget(t, mux)

	ctx := context.WithValue(t.Context(), peerNodeIDCtxKey{}, "node-2")

	rec, _ := leaderForward(t, forwardEnvelope{Method: http.MethodDelete, Path: "/api/v1/cluster/members/node-2"}, ctx)
	if rec.Header().Get("Connection") != "close" {
		t.Fatal("the removed peer's connection stays open")
	}
}

func TestForwardRoundTrip(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/cluster/members/{id}/promote", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"Node is already a voter: ` + r.PathValue("id") + ` from ` + getClientIP(r) + `"}`))
	})
	withForwardTarget(t, mux)

	f := &fakeForwarder{enabled: true, do: func(_ int, body []byte) (*ellaraft.LeaderResponse, error) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, InternalForwardPath, bytes.NewReader(body))
		rec := httptest.NewRecorder()
		ClusterForward(fakeIndexer(3)).ServeHTTP(rec, req)

		return &ellaraft.LeaderResponse{StatusCode: rec.Code, Body: rec.Body.Bytes()}, nil
	}}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/cluster/members/abc/promote", nil)
	req.RemoteAddr = "[2001:db8::1]:4321"
	rec := httptest.NewRecorder()

	ForwardToLeader(f, http.NotFoundHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict || rec.Body.String() != `{"error":"Node is already a voter: abc from 2001:db8::1"}` || f.waited != 3 {
		t.Fatalf("code=%d body=%s waited=%d", rec.Code, rec.Body.String(), f.waited)
	}
}
