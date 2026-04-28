// Copyright 2026 Ella Networks

package server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// dialPeerHTTPClient returns an HTTP client that dials the specified
// peer via the mTLS cluster listener, enforcing that the peer's
// certificate CN resolves to expectedNodeID. The client is built per
// request because peer identity is part of the TLS dial contract and
// raft addresses can change between requests.
func dialPeerHTTPClient(ln *listener.Listener, expectedNodeID int) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return ln.Dial(ctx, addr, expectedNodeID, listener.ALPNHTTP, 10*time.Second)
			},
		},
		Timeout: 0,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// connListener is a net.Listener backed by a channel of connections.
// The cluster listener's ALPNHTTP handler pushes accepted connections
// into the channel; http.Server.Serve consumes them via Accept.
type connListener struct {
	ch     chan net.Conn
	closed chan struct{}
	addr   net.Addr
	once   sync.Once
}

func newConnListener(addr net.Addr) *connListener {
	return &connListener{
		ch:     make(chan net.Conn, 16),
		closed: make(chan struct{}),
		addr:   addr,
	}
}

func (cl *connListener) Accept() (net.Conn, error) {
	select {
	case conn, ok := <-cl.ch:
		if !ok {
			return nil, net.ErrClosed
		}

		return conn, nil
	case <-cl.closed:
		return nil, net.ErrClosed
	}
}

func (cl *connListener) Close() error {
	cl.once.Do(func() { close(cl.closed) })
	return nil
}

func (cl *connListener) Addr() net.Addr {
	return cl.addr
}

// opaqueConn wraps a net.Conn so that http.Server.Serve does not type-
// assert it to *tls.Conn. Without this wrapper the server sees the
// custom ALPN ("ella-http-v1"), finds no TLSNextProto handler, and
// drops the connection. The TLS layer is already terminated by the
// cluster listener; the HTTP server reads plaintext from Read/Write.
type opaqueConn struct {
	net.Conn
}

// clusterListenerForPeerLookup captures the listener so
// peerNodeIDConnContext can resolve peer identity via the current trust
// bundle. Swapped in by StartClusterHTTP before the server starts.
var clusterListenerForPeerLookup *listener.Listener

// StartClusterHTTP registers the ALPNHTTP handler on the cluster
// listener and starts an HTTP server serving the cluster-internal mux.
// The returned function shuts down the server.
func StartClusterHTTP(dbInstance *db.Database, ln *listener.Listener) func() {
	addr, _ := net.ResolveTCPAddr("tcp", ln.AdvertiseAddress())
	cl := newConnListener(addr)

	ln.Register(listener.ALPNHTTP, func(conn net.Conn) {
		select {
		case cl.ch <- &opaqueConn{conn}:
		case <-cl.closed:
			_ = conn.Close()
		}
	})

	clusterListenerForPeerLookup = ln

	mux := newClusterMux(dbInstance)
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ConnContext:       peerNodeIDConnContext,
	}

	go func() {
		if err := srv.Serve(cl); err != nil && err != http.ErrServerClosed {
			logger.APILog.Error("Cluster HTTP server error", zap.Error(err))
		}
	}()

	return func() {
		_ = cl.Close()
		_ = srv.Close()
	}
}

// peerNodeIDCtxKey is the context key used to carry the peer certificate's
// CN-derived node-id through cluster-port requests. It is set by
// peerNodeIDConnContext at TLS handshake time and consumed by handlers
// that need peer identity (e.g. self-registration on POST /cluster/members).
type peerNodeIDCtxKey struct{}

// peerNodeIDConnContext extracts the peer node-id from the cluster TLS
// connection and stashes it in the request context. Returns the context
// unchanged when the connection is not a cluster TLS connection (should
// not happen in production — guarded defensively).
func peerNodeIDConnContext(ctx context.Context, c net.Conn) context.Context {
	oc, ok := c.(*opaqueConn)
	if !ok {
		return ctx
	}

	tc, ok := oc.Conn.(*tls.Conn)
	if !ok {
		return ctx
	}

	if clusterListenerForPeerLookup == nil {
		return ctx
	}

	id, err := clusterListenerForPeerLookup.PeerNodeID(tc)
	if err != nil {
		return ctx
	}

	return context.WithValue(ctx, peerNodeIDCtxKey{}, id)
}

// peerNodeIDFromContext returns the peer node-id set by
// peerNodeIDConnContext. The boolean is false when the request did not
// originate on the cluster port.
func peerNodeIDFromContext(ctx context.Context) (int, bool) {
	v, ok := ctx.Value(peerNodeIDCtxKey{}).(int)
	return v, ok
}
