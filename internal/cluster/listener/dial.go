// Copyright 2026 Ella Networks

package listener

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

// Dial opens an mTLS connection to the cluster peer identified by
// expectedPeerID at addr and negotiates alpn. The peer's leaf must chain
// to the current trust bundle, have a SAN whose cluster-id matches, and
// resolve to expectedPeerID; otherwise the connection is torn down.
// Use this whenever the caller knows which node it intends to reach.
func (l *Listener) Dial(ctx context.Context, addr string, expectedPeerID int, alpn string, timeout time.Duration) (*tls.Conn, error) {
	return l.dial(ctx, addr, expectedPeerID, alpn, timeout)
}

// DialAnyPeer opens an mTLS connection to a cluster peer at addr without
// constraining which node answers. Chain + SAN shape are still verified;
// only the node-id match is relaxed.
func (l *Listener) DialAnyPeer(ctx context.Context, addr, alpn string, timeout time.Duration) (*tls.Conn, error) {
	return l.dial(ctx, addr, 0, alpn, timeout)
}

func (l *Listener) dial(ctx context.Context, addr string, expectedPeerID int, alpn string, timeout time.Duration) (*tls.Conn, error) {
	dialCfg := l.tlsConfig.Clone()
	dialCfg.NextProtos = []string{alpn}

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: timeout},
		Config:    dialCfg,
	}

	if timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, timeout)

		defer cancel()
	}

	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("cluster dial %s (ALPN %s): %w", addr, alpn, err)
	}

	tlsConn, ok := rawConn.(*tls.Conn)
	if !ok {
		_ = rawConn.Close()
		return nil, fmt.Errorf("cluster dial %s: connection is not TLS", addr)
	}

	if proto := tlsConn.ConnectionState().NegotiatedProtocol; proto != alpn {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("cluster dial %s: negotiated ALPN %q, expected %q", addr, proto, alpn)
	}

	if expectedPeerID != 0 {
		actualID, err := PeerNodeID(tlsConn, l.cfg.TrustBundle())
		if err != nil {
			_ = tlsConn.Close()
			return nil, fmt.Errorf("cluster dial %s: %w", addr, err)
		}

		if actualID != expectedPeerID {
			_ = tlsConn.Close()
			return nil, fmt.Errorf("cluster dial %s: expected peer node-id %d, peer presented %d", addr, expectedPeerID, actualID)
		}
	}

	return tlsConn, nil
}
