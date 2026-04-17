// Copyright 2026 Ella Networks

package listener

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

// Dial opens a TLS connection to a peer cluster address with the given
// ALPN protocol. The node's leaf certificate is presented as the client
// cert, and the peer's certificate is verified against the cluster CA
// via the same verifyPeer callback used on the accept path.
func (l *Listener) Dial(ctx context.Context, addr, alpn string, timeout time.Duration) (*tls.Conn, error) {
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

	return tlsConn, nil
}
