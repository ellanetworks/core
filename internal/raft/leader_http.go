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

const (
	leaderIdleConns = 8

	leaderIdleConnTimeout = 90 * time.Second

	leaderResponseHeaderTimeout = 30 * time.Second
)

type peerDialFunc func(ctx context.Context, addr string, peerID int) (net.Conn, error)

var ErrLeaderUnreachable = errors.New("leader unreachable")

// ErrLeaderRequestNotSent marks a failure that happened before any
// bytes reached the wire, so the forwarded write definitively did not
// reach the leader.
var ErrLeaderRequestNotSent = errors.New("leader request not sent")

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

type leaderHTTPRequest struct {
	method      string
	path        string
	contentType string
	body        []byte

	maxResponseBytes int64

	timeout time.Duration
}

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
		return nil, fmt.Errorf("%w: new request: %w", ErrLeaderRequestNotSent, err)
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
