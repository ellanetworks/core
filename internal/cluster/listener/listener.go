// Copyright 2026 Ella Networks

// Package listener provides a multiplexed TLS listener for intra-cluster
// communication. A single TCP socket carries all cluster traffic — Raft
// consensus, cluster-internal HTTP, PKI key transfer, and the join-flow
// bootstrap path — distinguished by ALPN protocol negotiation during the
// TLS handshake.
//
// Every non-bootstrap connection is mutually authenticated: both sides
// present a leaf certificate signed by the cluster CA, with a URI SAN
// whose cluster-id segment matches this node's local cluster-id. The
// bootstrap ALPN (ella-pki-bootstrap-v1) is the exception: joining nodes
// have no leaf yet and authenticate via a join-token HMAC in the request
// body. The server side is authenticated via a root-fingerprint pin the
// operator transfers alongside the join token.
//
// Outbound dials use Dial when the caller knows which peer they intend
// to reach; Dial refuses the connection if the peer's URI SAN resolves
// to a different node-id. DialAnyPeer relaxes that last check and is
// used by discovery paths that need to learn the peer's identity from
// the connection itself.
//
// The listener tracks active authenticated connections in a concurrent-
// safe map so CloseByPeerSerial can tear down a node's connections when
// its leaf has been revoked (see RemoveClusterMember).

package listener

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math/big"
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

// TrustBundleFunc returns the current cluster trust bundle. Called at
// handshake time on every accepted connection.
type TrustBundleFunc func() *pki.TrustBundle

// LeafFunc returns the current leaf certificate for this node, or nil if
// the node has not yet been issued one. Called at handshake time as
// tls.Config.GetCertificate / GetClientCertificate. Returning nil on the
// server side aborts the handshake; returning an empty tls.Certificate on
// the client side (bootstrap path) lets us handshake without a client cert.
type LeafFunc func() *tls.Certificate

// RevokedFunc returns true if the given certificate serial has been
// revoked. Called at handshake time after chain validation.
type RevokedFunc func(serial *big.Int) bool

// Config captures the bind address and the dynamic accessors the listener
// consults on every handshake. All accessors are mandatory.
type Config struct {
	BindAddress      string
	AdvertiseAddress string
	NodeID           int

	TrustBundle TrustBundleFunc
	Leaf        LeafFunc
	Revoked     RevokedFunc
}

// Listener is the multiplexed cluster port. One TCP socket, one TLS
// configuration, N logical protocols dispatched by ALPN NegotiatedProtocol.
type Listener struct {
	cfg       Config
	tlsConfig *tls.Config
	tcpLn     net.Listener
	handlers  map[string]ConnHandler
	mu        sync.Mutex
	stopCh    chan struct{}
	wg        sync.WaitGroup

	// connMu protects conns. Every authenticated connection is registered
	// after handshake and deregistered when the handler returns, so
	// CloseByPeerSerial can scan for matches. Bootstrap-ALPN connections
	// are tracked too (serial is zero-ish, but they're short-lived).
	connMu sync.Mutex
	conns  map[*tls.Conn]struct{}
}

// New creates a Listener from the given config. Register ALPN handlers
// with Register before calling Start.
func New(cfg Config) *Listener {
	if cfg.TrustBundle == nil || cfg.Leaf == nil || cfg.Revoked == nil {
		// These are programmer errors at wiring time; crashing is OK.
		panic("listener.New: TrustBundle, Leaf, and Revoked accessors are mandatory")
	}

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS13,
		// ClientAuth is RequestClientCert so the bootstrap ALPN can
		// complete without a peer cert. For non-bootstrap ALPNs the
		// joining node always presents its leaf, and VerifyConnection
		// rejects the handshake if the cert is missing or invalid.
		//
		// Native x509 chain verification is bypassed in favour of
		// verifyConnection (which uses the replicated trust bundle):
		// VerifyPeerCertificate returns nil unconditionally, and
		// InsecureSkipVerify covers the client side.
		ClientAuth: tls.RequestClientCert,
		VerifyPeerCertificate: func(_ [][]byte, _ [][]*x509.Certificate) error {
			return nil
		},
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
		NextProtos:         []string{ALPNRaft, ALPNHTTP, ALPNPKIKeyTransfer, ALPNPKIBootstrap},
		VerifyConnection:   verifyConnection(cfg.TrustBundle, cfg.Revoked),
		InsecureSkipVerify: true, // #nosec G402 -- verifyConnection + VerifyPeerCertificate own cert validation.
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

	l.mu.Lock()
	l.tcpLn = tcpLn
	l.mu.Unlock()

	tlsLn := tls.NewListener(tcpLn, l.tlsConfig)

	go func() {
		select {
		case <-ctx.Done():
			l.Stop()
		case <-l.stopCh:
		}
	}()

	go l.acceptLoop(ctx, tlsLn)

	return nil
}

func (l *Listener) acceptLoop(ctx context.Context, tlsLn net.Listener) {
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

// Stop gracefully shuts down the listener. In-flight connections are
// allowed to drain.
func (l *Listener) Stop() {
	l.mu.Lock()

	select {
	case <-l.stopCh:
		l.mu.Unlock()
		return
	default:
	}

	close(l.stopCh)

	if l.tcpLn != nil {
		_ = l.tcpLn.Close()
	}

	l.mu.Unlock()

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

// trackConn registers a post-handshake connection so CloseByPeerSerial
// can close it if its peer's serial is later revoked.
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

// CloseByPeerSerial closes every tracked connection whose peer leaf has
// the given serial. Used by RemoveClusterMember to tear down a removed
// voter's active mTLS sessions immediately, rather than waiting for
// next-dial revocation enforcement. Returns the count of connections
// closed.
func (l *Listener) CloseByPeerSerial(serial *big.Int) int {
	if serial == nil {
		return 0
	}

	l.connMu.Lock()

	var victims []*tls.Conn

	for c := range l.conns {
		state := c.ConnectionState()
		if len(state.PeerCertificates) == 0 {
			continue
		}

		if state.PeerCertificates[0].SerialNumber.Cmp(serial) == 0 {
			victims = append(victims, c)
		}
	}
	l.connMu.Unlock()

	for _, c := range victims {
		_ = c.Close()
	}

	return len(victims)
}
