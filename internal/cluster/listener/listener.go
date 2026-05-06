// Copyright 2026 Ella Networks

// Package listener provides a multiplexed TLS listener for
// intra-cluster communication. A single TCP socket carries all
// cluster traffic — Raft consensus, cluster-internal HTTP, and the
// join-flow bootstrap path — dispatched by ALPN protocol negotiated
// at handshake time.
//
// On every non-bootstrap ALPN both sides present a self-signed
// cluster certificate, and the verifier accepts the connection when
// the peer's leaf SHA-256 matches a row in cluster_node_certs (via
// the in-memory pin map) and its SPIFFE URI SAN's nodeID matches
// the pin's owner. The bootstrap ALPN (ella-pki-bootstrap-v1) skips
// client-cert presentation; joining nodes authenticate via a
// join-token HMAC in the request body and pin the server cert
// using the LeaderCertPin claim embedded in the token.
//
// Outbound dials use Dial when the caller knows which peer they
// intend to reach; Dial refuses the connection if the peer's URI
// SAN resolves to a different node-id. DialAnyPeer relaxes that
// check and is used by discovery paths that learn the peer's
// identity from the connection itself.
//
// The listener tracks active authenticated connections in a
// concurrent-safe map so CloseByPeerFingerprint can tear down a
// node's connections when its pin row has been deleted (see
// RemoveClusterMember).

package listener

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

// ConnHandler processes a TLS-wrapped connection dispatched by ALPN
// protocol. Handlers own the connection lifecycle and must close the
// connection when done.
type ConnHandler func(conn net.Conn)

// LeafFunc returns the current self-signed certificate for this node,
// or nil if the node has not yet generated one. Called at handshake
// time as tls.Config.GetCertificate / GetClientCertificate. Returning
// nil on the server side aborts the handshake; returning an empty
// tls.Certificate on the client side (bootstrap path) lets us
// handshake without a client cert.
type LeafFunc func() *tls.Certificate

// Config captures the bind address and the dynamic accessors the listener
// consults on every handshake.
type Config struct {
	BindAddress      string
	AdvertiseAddress string
	NodeID           int

	// Pin returns whether a peer cert's SHA-256 fingerprint is
	// registered (and which nodeID owns it). Called once per
	// non-bootstrap handshake.
	Pin PinFunc

	// Leaf returns this node's self-signed cluster cert.
	Leaf LeafFunc
}

// Listener is the multiplexed cluster port. One TCP socket, one TLS
// configuration, N logical protocols dispatched by ALPN NegotiatedProtocol.
//
// Lifecycle: New → Register* → Start → Stop. Stop waits for every
// goroutine the listener owns (the ctx watcher, the accept loop, and
// every in-flight dispatch handler) before returning, so callers can
// safely free state the listener referenced once Stop returns.
type Listener struct {
	cfg       Config
	tlsConfig *tls.Config
	tlsLn     net.Listener
	handlers  map[string]ConnHandler
	mu        sync.Mutex
	stopCh    chan struct{}
	// wg counts every goroutine the listener owns: the ctx watcher
	// and accept loop spawned by Start, plus every dispatch handler
	// the accept loop spawns. The accept loop holds its own +1 for
	// its full lifetime, which is what guarantees the counter cannot
	// reach zero before the loop has exited; this in turn makes it
	// safe for the loop to call wg.Add(1) for each new dispatch
	// without racing wg.Wait in Stop.
	wg sync.WaitGroup

	// connMu protects conns. Every accepted connection is
	// registered after handshake and deregistered when its handler
	// returns; CloseByPeerFingerprint scans the map to tear down
	// connections from a peer whose pin has been removed.
	connMu sync.Mutex
	conns  map[*tls.Conn]struct{}
}

// New creates a Listener from the given config. Register ALPN handlers
// with Register before calling Start.
func New(cfg Config) *Listener {
	if cfg.Pin == nil || cfg.Leaf == nil {
		// These are programmer errors at wiring time; crashing is OK.
		panic("listener.New: Pin and Leaf accessors are mandatory")
	}

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS13,
		// ClientAuth is RequestClientCert so the bootstrap ALPN can
		// complete without a peer cert. For non-bootstrap ALPNs the
		// joining node always presents its self-signed leaf, and
		// VerifyConnection rejects the handshake if the cert is
		// missing or its fingerprint is not pinned.
		// ClientAuth=RequestClientCert lets the bootstrap ALPN
		// complete without a peer cert; on every other ALPN the
		// VerifyConnection callback rejects connections whose peer
		// leaf is missing or unpinned.
		ClientAuth: tls.RequestClientCert,
		GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
			leaf := cfg.Leaf()
			if leaf == nil || len(leaf.Certificate) == 0 {
				return nil, fmt.Errorf("cluster listener: no server leaf available yet")
			}

			return leaf, nil
		},
		GetClientCertificate: func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			leaf := cfg.Leaf()
			if leaf == nil {
				return &tls.Certificate{}, nil
			}

			return leaf, nil
		},
		NextProtos:         []string{ALPNRaft, ALPNHTTP, ALPNPKIBootstrap},
		VerifyConnection:   verifyConnection(cfg.Pin),
		InsecureSkipVerify: true, // #nosec G402 -- VerifyConnection enforces fingerprint pinning.
	}

	return &Listener{
		cfg:       cfg,
		tlsConfig: tlsCfg,
		handlers:  make(map[string]ConnHandler),
		stopCh:    make(chan struct{}),
		conns:     make(map[*tls.Conn]struct{}),
	}
}

// Register attaches a handler for one ALPN protocol. Must be called
// before Start. Panics if the protocol is already registered.
func (l *Listener) Register(alpn string, handler ConnHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.handlers[alpn]; exists {
		panic(fmt.Sprintf("cluster listener: ALPN %q already registered", alpn))
	}

	l.handlers[alpn] = handler
}

// Start binds the TCP socket and begins accepting and dispatching
// connections in the background. Returns immediately after the socket
// is bound (or with an error if binding fails). Cancel ctx or call
// Stop to shut down the accept loop.
func (l *Listener) Start(ctx context.Context) error {
	lc := net.ListenConfig{}

	tcpLn, err := lc.Listen(ctx, "tcp", l.cfg.BindAddress)
	if err != nil {
		return fmt.Errorf("cluster listener bind %s: %w", l.cfg.BindAddress, err)
	}

	tlsLn := tls.NewListener(tcpLn, l.tlsConfig)

	l.mu.Lock()
	l.tlsLn = tlsLn
	l.mu.Unlock()

	l.wg.Add(2)

	go l.watchCtx(ctx)
	go l.acceptLoop(ctx, tlsLn)

	return nil
}

// watchCtx mirrors ctx cancellation onto the listener's stop signal.
// Tracked in l.wg so Stop blocks until this goroutine has exited.
// Calls signalStop (close stopCh + close tlsLn) rather than Stop itself,
// because Stop's wg.Wait would deadlock waiting for this goroutine.
func (l *Listener) watchCtx(ctx context.Context) {
	defer l.wg.Done()

	select {
	case <-ctx.Done():
		l.signalStop()
	case <-l.stopCh:
	}
}

// signalStop closes stopCh and the TLS listener, the two side-effects
// that cause the accept loop and ctx watcher to wake up and exit.
// Idempotent: a second caller observes stopCh already closed and returns.
// Does not block on goroutine completion; that is Stop's job.
func (l *Listener) signalStop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	select {
	case <-l.stopCh:
		return
	default:
	}

	close(l.stopCh)

	if l.tlsLn != nil {
		_ = l.tlsLn.Close()
	}
}

func (l *Listener) acceptLoop(ctx context.Context, tlsLn net.Listener) {
	defer l.wg.Done()

	const (
		baseDelay = 5 * time.Millisecond
		maxDelay  = 1 * time.Second
	)

	delay := baseDelay

	for {
		conn, err := tlsLn.Accept()
		if err != nil {
			select {
			case <-l.stopCh:
				return
			default:
			}

			if errors.Is(err, net.ErrClosed) {
				return
			}

			logger.RaftLog.Warn("Cluster listener accept error, backing off",
				zap.Error(err), zap.Duration("delay", delay))

			time.Sleep(delay)

			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}

			continue
		}

		delay = baseDelay

		l.wg.Add(1)

		go l.dispatch(ctx, conn)
	}
}

// Stop shuts the listener down and blocks until every goroutine the
// listener owns has returned: the ctx watcher, the accept loop, and
// every in-flight dispatch handler. Once Stop returns the caller can
// safely free state the listener referenced (sockets, TLS configs,
// captured context).
//
// Must not be called from a goroutine that is itself tracked in l.wg
// (the ctx watcher uses signalStop instead, for that reason).
func (l *Listener) Stop() {
	l.signalStop()
	l.wg.Wait()
}

// AdvertiseAddress returns the address peers use to reach this listener.
func (l *Listener) AdvertiseAddress() string {
	return l.cfg.AdvertiseAddress
}

const handshakeTimeout = 30 * time.Second

func (l *Listener) dispatch(ctx context.Context, conn net.Conn) {
	defer l.wg.Done()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		_ = conn.Close()
		return
	}

	_ = tlsConn.SetDeadline(time.Now().Add(handshakeTimeout))

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		logger.RaftLog.Warn("Cluster TLS handshake failed",
			zap.String("remote", conn.RemoteAddr().String()),
			zap.Error(err))
		_ = conn.Close()

		return
	}

	_ = tlsConn.SetDeadline(time.Time{})

	proto := tlsConn.ConnectionState().NegotiatedProtocol

	l.mu.Lock()
	handler, exists := l.handlers[proto]
	l.mu.Unlock()

	if !exists {
		logger.RaftLog.Warn("Cluster connection with unknown ALPN protocol",
			zap.String("remote", conn.RemoteAddr().String()),
			zap.String("protocol", proto))
		_ = conn.Close()

		return
	}

	l.trackConn(tlsConn)
	defer l.untrackConn(tlsConn)

	handler(conn)
}

// trackConn registers a post-handshake connection so
// CloseByPeerFingerprint can close it on pin removal.
func (l *Listener) trackConn(c *tls.Conn) {
	l.connMu.Lock()
	l.conns[c] = struct{}{}
	l.connMu.Unlock()
}

func (l *Listener) untrackConn(c *tls.Conn) {
	l.connMu.Lock()
	delete(l.conns, c)
	l.connMu.Unlock()
}

// CloseByPeerFingerprint closes every tracked connection whose peer
// leaf matches fingerprint. RemoveClusterMember calls this so a
// removed voter's active mTLS sessions drop sub-second instead of
// at the next handshake. Returns the count of connections closed.
func (l *Listener) CloseByPeerFingerprint(fingerprint string) int {
	if fingerprint == "" {
		return 0
	}

	l.connMu.Lock()

	var victims []*tls.Conn

	for c := range l.conns {
		state := c.ConnectionState()
		if len(state.PeerCertificates) == 0 {
			continue
		}

		if pki.Fingerprint(state.PeerCertificates[0]) == fingerprint {
			victims = append(victims, c)
		}
	}
	l.connMu.Unlock()

	for _, c := range victims {
		_ = c.Close()
	}

	return len(victims)
}
