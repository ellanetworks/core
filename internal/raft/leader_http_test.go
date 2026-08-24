// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func startClusterHTTPStub(t *testing.T, handler http.Handler) (addr string) {
	t.Helper()

	lc := net.ListenConfig{}

	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}

	go func() { _ = srv.Serve(ln) }()

	t.Cleanup(func() { _ = srv.Close() })

	return ln.Addr().String()
}

func TestLeaderHTTPClient_ReusesConnections(t *testing.T) {
	addr := startClusterHTTPStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	var dials atomic.Int64

	c := newLeaderHTTPClient(func(ctx context.Context, a string, _ int) (net.Conn, error) {
		dials.Add(1)
		return (&net.Dialer{}).DialContext(ctx, "tcp", a)
	})

	defer c.close()

	for i := range 5 {
		resp, err := c.do(context.Background(), addr, 1, leaderHTTPRequest{
			method:           http.MethodPost,
			path:             ProposeForwardPath,
			contentType:      "application/json",
			body:             []byte(`{}`),
			maxResponseBytes: 1024,
		})
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: status %d", i, resp.StatusCode)
		}
	}

	if got := dials.Load(); got != 1 {
		t.Fatalf("5 sequential requests to one leader should share a connection; dialed %d times", got)
	}
}

func TestLeaderHTTPClient_RebuildsOnLeaderChange(t *testing.T) {
	addr := startClusterHTTPStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))

	c := newLeaderHTTPClient(func(ctx context.Context, a string, _ int) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", a)
	})

	defer c.close()

	first := c.clientFor(addr, 1)

	if same := c.clientFor(addr, 1); same != first {
		t.Fatal("same leader must reuse the same pooled client")
	}

	if other := c.clientFor(addr, 2); other == first {
		t.Fatal("a leadership change must retire the previous leader's client")
	}
}

func TestLeaderHTTPClient_OversizeResponseIsAnError(t *testing.T) {
	addr := startClusterHTTPStub(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 4096)))
	}))

	c := newLeaderHTTPClient(func(ctx context.Context, a string, _ int) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", a)
	})

	defer c.close()

	_, err := c.do(context.Background(), addr, 1, leaderHTTPRequest{
		method:           http.MethodGet,
		path:             "/cluster/status",
		maxResponseBytes: 512,
	})
	if err == nil {
		t.Fatal("an oversize response must be an error, not a silent truncation")
	}

	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error should name the size limit, got: %v", err)
	}
}

func TestLeaderHTTPClient_DialFailureIsMarkedUnreachable(t *testing.T) {
	c := newLeaderHTTPClient(func(context.Context, string, int) (net.Conn, error) {
		return nil, fmt.Errorf("no route to host")
	})

	defer c.close()

	_, err := c.do(context.Background(), "127.0.0.1:1", 1, leaderHTTPRequest{
		method:           http.MethodPost,
		path:             ProposeForwardPath,
		body:             []byte(`{}`),
		maxResponseBytes: 512,
	})
	if !errors.Is(err, ErrLeaderUnreachable) {
		t.Fatalf("dial failure must report ErrLeaderUnreachable, got: %v", err)
	}
}

func TestLeaderHTTPClient_TimeoutBoundsTheRoundTrip(t *testing.T) {
	addr := startClusterHTTPStub(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))

	c := newLeaderHTTPClient(func(ctx context.Context, a string, _ int) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", a)
	})

	defer c.close()

	start := time.Now()

	_, err := c.do(context.Background(), addr, 1, leaderHTTPRequest{
		method:           http.MethodPost,
		path:             ProposeForwardPath,
		body:             []byte(`{}`),
		maxResponseBytes: 512,
		timeout:          150 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("a stalled leader must not block the caller past the request timeout")
	}

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("request took %s; the timeout did not bound it", elapsed)
	}
}
