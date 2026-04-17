// Copyright 2026 Ella Networks

// Package listener provides a multiplexed TLS listener for intra-cluster
// communication. A single TCP socket carries all cluster traffic — Raft
// consensus and cluster-internal HTTP — distinguished by ALPN protocol
// negotiation during the TLS handshake.
//
// Every connection is mutually authenticated: both sides present a leaf
// certificate signed by the shared cluster CA, with CN = "ella-node-<n>"
// where n is a valid node-id in [1, 63]. Connections without a verified
// peer cert are rejected at handshake.

package listener

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// ConnHandler processes a TLS-wrapped connection dispatched by ALPN
// protocol. Handlers own the connection lifecycle and must close the
// connection when done.
type ConnHandler func(conn net.Conn)

// Config captures the fully-loaded cluster PKI material plus the bind
// address. Built once at startup from config.ClusterTLS + ClusterConfig.
type Config struct {
	BindAddress      string
	AdvertiseAddress string
	NodeID           int
	CAPool           *x509.CertPool
	LeafCert         tls.Certificate
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
}

// New creates a Listener from the given config. Register ALPN handlers
// with Register before calling Start.
func New(cfg Config) *Listener {
	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cfg.LeafCert},
		ClientAuth:   tls.RequireAnyClientCert,
		// Skip stock hostname verification; chain and CN validation
		// happens in VerifyConnection (which also covers resumed
		// sessions, unlike VerifyPeerCertificate).
		InsecureSkipVerify: true, // #nosec G402 -- custom VerifyConnection enforces CA chain + CN identity
		VerifyConnection:   verifyConnection(cfg.CAPool),
		NextProtos:         []string{ALPNRaft, ALPNHTTP},
	}

	return &Listener{
		cfg:       cfg,
		tlsConfig: tlsCfg,
		handlers:  make(map[string]ConnHandler),
		stopCh:    make(chan struct{}),
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

	// Release the lock before waiting — dispatch() acquires mu after
	// handshake to look up the handler. Holding mu here would deadlock.
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

	handler(conn)
}
