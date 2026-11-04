// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/ellanetworks/core/internal/roaming/wire"
	"go.uber.org/zap"
)

var (
	ErrNoContext        = errors.New("sepp: no N32 context with the partner")
	ErrUnreachable      = errors.New("sepp: partner SEPP unreachable")
	ErrTargetNotAllowed = errors.New("sepp: target outside the partner's network")
	ErrRedirect         = errors.New("sepp: unsupported N32-c redirection")
)

const (
	directionInbound  = "inbound"
	directionOutbound = "outbound"

	causeSEPPRedirectionWithDiscovery = "SEPP_REDIRECTION_WITH_DISCOVERY"
	maxN32cRedirects                  = 1
)

var strippedHeaders = []string{"Traceparent", "Tracestate", HeaderTargetAPIRoot, HeaderOriginatingNetworkID, HeaderN32HandshakeID}

type n32fTransport struct {
	key      [sha256.Size]byte
	peer     seppPeer
	identity *localIdentity
	tr       *http.Transport

	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool
}

func (t *n32fTransport) track(c net.Conn) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return false
	}

	t.conns[c] = struct{}{}

	return true
}

func (t *n32fTransport) close() {
	t.mu.Lock()
	t.closed = true
	conns := t.conns
	t.conns = map[net.Conn]struct{}{}
	t.mu.Unlock()

	t.tr.CloseIdleConnections()

	for c := range conns {
		_ = c.Close()
	}
}

type trackedConn struct {
	net.Conn
	once sync.Once
	t    *n32fTransport
}

func (c *trackedConn) Close() error {
	c.once.Do(func() {
		c.t.mu.Lock()
		delete(c.t.conns, c.Conn)
		c.t.mu.Unlock()
	})

	return c.Conn.Close()
}

func (m *Manager) newN32fTransport(p *partner, identity *localIdentity, peer seppPeer) *n32fTransport {
	t := &n32fTransport{key: p.key, peer: peer, identity: identity, conns: map[net.Conn]struct{}{}}

	protocols := new(http.Protocols)
	protocols.SetHTTP2(true)

	t.tr = &http.Transport{
		TLSClientConfig: m.clientTLSConfig(p, peer.n32f.Hostname(), peer.fqdn),
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			c, err := m.dialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}

			if !t.track(c) {
				_ = c.Close()
				return nil, errors.New("sepp: N32-f transport closed")
			}

			return &trackedConn{Conn: c, t: t}, nil
		},
		TLSHandshakeTimeout:    m.tuning.HandshakeTimeout,
		MaxResponseHeaderBytes: maxHeaderBytes,
		Protocols:              protocols,
		HTTP2: &http.HTTP2Config{
			SendPingTimeout: m.tuning.PingIdle,
			PingTimeout:     m.tuning.PingTimeout,
		},
	}

	return t
}

func (m *Manager) transportFor(snap *snapshot, p *partner, peer seppPeer) *n32fTransport {
	m.transportsMu.Lock()
	defer m.transportsMu.Unlock()

	if t, ok := m.transports[p.id]; ok && t.key == p.key && t.identity == snap.identity && t.peer == peer {
		return t
	}

	if old, ok := m.transports[p.id]; ok {
		go old.close()
	}

	t := m.newN32fTransport(p, snap.identity, peer)
	m.transports[p.id] = t

	return t
}

func (m *Manager) syncTransports(prev, next *snapshot) {
	m.transportsMu.Lock()
	defer m.transportsMu.Unlock()

	for id, t := range m.transports {
		p, ok := next.partners[id]
		if ok && t.key == p.key && t.identity == next.identity {
			continue
		}

		go t.close()

		delete(m.transports, id)
	}

	if prev != nil && prev.identity != next.identity {
		for _, p := range next.partners {
			go m.closeInbound(p.id, classN32f)
		}
	}
}

func (m *Manager) closeN32f(partnerID string) {
	m.transportsMu.Lock()
	t, ok := m.transports[partnerID]
	delete(m.transports, partnerID)
	m.transportsMu.Unlock()

	if ok {
		t.close()
	}

	m.closeInbound(partnerID, classN32f)
}

func (m *Manager) contextLost(partnerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.contexts[partnerID]
	if !ok || m.root == nil || c.status.State != StateEstablished {
		return
	}

	now := m.now()
	if !c.lostAt.IsZero() && now.Sub(c.lostAt) < m.tuning.ContextLostDelay {
		return
	}

	c.lostAt = now

	m.logger.Warn("The partner SEPP answered CONTEXT_NOT_FOUND: re-handshaking", zap.String("partner", c.partner.name))

	snap := m.snap.Load()
	if snap.usable() && c.partner.usable() {
		m.startLocked(partnerID, c, snap)
	}
}

func (m *Manager) Client(partnerID string) *http.Client {
	return &http.Client{
		Transport:     &forwarder{m: m, partnerID: partnerID},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

type forwarder struct {
	m         *Manager
	partnerID string
}

func serviceOf(path string) string {
	seg, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	if seg == "" || len(seg) > 32 {
		return "other"
	}

	return seg
}

func (f *forwarder) RoundTrip(req *http.Request) (*http.Response, error) {
	m := f.m

	snap, p, err := m.partner(f.partnerID)
	if err != nil {
		return nil, err
	}

	if !snap.usable() {
		return nil, fmt.Errorf("%w: %s", ErrNoContext, snap.problem)
	}

	st, peer := m.contextPeer(p)
	if st.State != StateEstablished {
		return nil, fmt.Errorf("%w %s (state %s)", ErrNoContext, p.name, st.State)
	}

	if err := p.allowsTarget(req.URL); err != nil {
		return nil, err
	}

	body, err := bufferBody(req)
	if err != nil {
		return nil, err
	}

	base := &url.URL{Scheme: peer.n32f.Scheme, Host: peer.n32f.Authority, Path: peer.n32f.Prefix}

	out, root, err := RewriteToN32f(req.URL, base)
	if err != nil {
		return nil, err
	}

	ctx := req.Context()
	cancel := context.CancelFunc(func() {})

	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, m.tuning.RequestTimeout)
	}

	outReq := req.Clone(ctx)
	outReq.URL = out
	outReq.Host = ""
	outReq.RequestURI = ""

	for _, h := range strippedHeaders {
		outReq.Header.Del(h)
	}

	outReq.Header.Set(HeaderTargetAPIRoot, root.String())
	outReq.Header.Set(HeaderOriginatingNetworkID, snap.op.String())
	outReq.Header.Add(HeaderVia, SEPPVia(snap.identity.fqdn))

	service := serviceOf(req.URL.Path)
	start := m.now()

	resp, err := m.send(outReq, body, m.transportFor(snap, p, peer).tr)

	n32fDuration.WithLabelValues(p.name, service).Observe(m.now().Sub(start).Seconds())

	if err != nil {
		cancel()
		n32fRequests.WithLabelValues(p.name, directionOutbound, service, "error").Inc()

		if errors.Is(err, ErrTargetNotAllowed) {
			return nil, err
		}

		return nil, fmt.Errorf("%w %s: %w", ErrUnreachable, p.name, err)
	}

	n32fRequests.WithLabelValues(p.name, directionOutbound, service, strconv.Itoa(resp.StatusCode)).Inc()

	if resp.StatusCode == http.StatusForbidden {
		m.checkContextNotFound(p.id, resp)
	}

	resp.Body = &cancelBody{ReadCloser: resp.Body, cancel: cancel}

	return resp, nil
}

func bufferBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}

	defer func() { _ = req.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(req.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("sepp: read request body: %w", err)
	}

	if len(body) > maxBodyBytes {
		return nil, fmt.Errorf("sepp: request body exceeds %d bytes", maxBodyBytes)
	}

	return body, nil
}

func (m *Manager) send(req *http.Request, body []byte, rt http.RoundTripper) (*http.Response, error) {
	r := req.Clone(req.Context())

	if body != nil {
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
		r.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	} else {
		r.Body = http.NoBody
		r.ContentLength = 0
	}

	return rt.RoundTrip(r)
}

func (m *Manager) checkContextNotFound(partnerID string, resp *http.Response) {
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(data))

	if err != nil {
		return
	}

	if wire.ParseProblem(resp.StatusCode, data).Cause == wire.CauseContextNotFound {
		go m.contextLost(partnerID)
	}
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()

	return err
}
