// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Every cluster-internal HTTP call a follower makes to the leader —
// forwarded writes (/cluster/internal/propose), forwarded reads
// (/cluster/internal/read), and follower-side leader queries such as
// autopilot state — goes through leaderHTTPClient.
//
// The pool exists because these calls are not rare. Forwarded reads sit
// on the per-UE attach path, so a fresh mTLS handshake plus a fresh
// http.Transport per call would burn a round-trip and a pair of file
// descriptors on every authentication. Worse, a hand-built Transport
// defaults to IdleConnTimeout 0: once the response body is closed the
// connection returns to that Transport's idle pool, and if the Transport
// is then dropped on the floor nothing ever closes the socket. One
// client goroutine and one server goroutine leak per call.
//
// So: one Transport per leader identity, reused across calls, with a
// non-zero IdleConnTimeout. The timeout matters twice — it reaps
// connections this node stops using, and it bounds the lifetime of a
// Transport that clientFor retires after a leadership change while a
// request was still in flight on it.

const (
	// leaderIdleConns caps pooled connections to the leader. Cluster
	// HTTP is request/response over a handful of concurrent callers;
	// a small pool keeps descriptor use bounded without serialising
	// concurrent forwards behind one connection.
	leaderIdleConns = 8

	// leaderIdleConnTimeout closes pooled connections that go unused.
	// Also the backstop that lets a Transport retired by a leadership
	// change become garbage instead of pinning sockets forever.
	leaderIdleConnTimeout = 90 * time.Second

	// leaderResponseHeaderTimeout bounds the wait for response headers
	// after the request is written, so a leader that accepted the
	// connection and then stalled cannot hang the caller past its own
	// deadline.
	leaderResponseHeaderTimeout = 30 * time.Second
)

// peerDialFunc opens an authenticated cluster connection to a peer.
type peerDialFunc func(ctx context.Context, addr string, peerID int) (net.Conn, error)

// ErrLeaderUnreachable marks a round-trip that failed before any byte
// of the request reached the leader. The propose path keys its retry
// decision on it: re-sending a non-idempotent write is only safe when
// the previous attempt provably never arrived. Every other failure —
// including a response lost after the request was written — is
// ambiguous and must surface to the caller instead.
var ErrLeaderUnreachable = errors.New("leader unreachable")

// leaderHTTPClient holds the pooled http.Client for the current leader
// and swaps it when leadership moves.
type leaderHTTPClient struct {
	dial peerDialFunc

	mu        sync.Mutex
	key       string
	transport *http.Transport
	client    *http.Client
}

func newLeaderHTTPClient(dial peerDialFunc) *leaderHTTPClient {
	return &leaderHTTPClient{dial: dial}
}

// clientFor returns the pooled client for the given leader, building a
// new one (and retiring the previous leader's) when leadership moves.
func (c *leaderHTTPClient) clientFor(addr string, peerID int) *http.Client {
	key := addr + "|" + strconv.Itoa(peerID)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil && c.key == key {
		return c.client
	}

	if c.transport != nil {
		c.transport.CloseIdleConnections()
	}

	transport := &http.Transport{
		DialTLSContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			conn, err := c.dial(ctx, addr, peerID)
			if err != nil {
				return nil, fmt.Errorf("%w: dial %s: %w", ErrLeaderUnreachable, addr, err)
			}

			return conn, nil
		},
		MaxIdleConns:          leaderIdleConns,
		MaxIdleConnsPerHost:   leaderIdleConns,
		IdleConnTimeout:       leaderIdleConnTimeout,
		ResponseHeaderTimeout: leaderResponseHeaderTimeout,
		DisableCompression:    true,
	}

	c.key = key
	c.transport = transport
	c.client = &http.Client{Transport: transport}

	return c.client
}

// close drops every pooled connection. Called from Manager.Shutdown.
func (c *leaderHTTPClient) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.transport != nil {
		c.transport.CloseIdleConnections()
	}

	c.key = ""
	c.transport = nil
	c.client = nil
}

// leaderHTTPRequest describes one round-trip against the leader's
// cluster port.
type leaderHTTPRequest struct {
	method      string
	path        string
	contentType string
	body        []byte

	// maxResponseBytes caps the response body. Exceeding it is an
	// error, never a truncation: a silently clipped body surfaces
	// downstream as an unhelpful JSON decode failure.
	maxResponseBytes int64

	// timeout bounds the whole round-trip. Zero leaves the caller's
	// context as the only deadline.
	timeout time.Duration
}

// LeaderHTTPResponse is the result of a round-trip against the leader.
type LeaderHTTPResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

func (c *leaderHTTPClient) do(ctx context.Context, addr string, peerID int, spec leaderHTTPRequest) (*LeaderHTTPResponse, error) {
	if spec.timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, spec.timeout)

		defer cancel()
	}

	var reqBody io.Reader
	if len(spec.body) > 0 {
		reqBody = bytes.NewReader(spec.body)
	}

	req, err := http.NewRequestWithContext(ctx, spec.method, "https://"+addr+spec.path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	if spec.contentType != "" {
		req.Header.Set("Content-Type", spec.contentType)
	}

	if len(spec.body) > 0 {
		req.ContentLength = int64(len(spec.body))
	}

	resp, err := c.clientFor(addr, peerID).Do(req) // #nosec G107 -- addr comes from Raft, not user input
	if err != nil {
		return nil, fmt.Errorf("request to leader: %w", err)
	}

	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, spec.maxResponseBytes))
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, spec.maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read leader response: %w", err)
	}

	if int64(len(body)) > spec.maxResponseBytes {
		return nil, fmt.Errorf("leader response for %s exceeds %d bytes", spec.path, spec.maxResponseBytes)
	}

	return &LeaderHTTPResponse{StatusCode: resp.StatusCode, Header: resp.Header, Body: body}, nil
}
