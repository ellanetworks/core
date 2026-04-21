// Copyright 2026 Ella Networks

package listener

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/pki"
)

// verifyConnection returns a tls.Config.VerifyConnection callback that:
//
//  1. On the bootstrap ALPN (ella-pki-bootstrap-v1): allows the
//     connection even without a peer cert. Auth is the caller's
//     responsibility (HMAC token in request body).
//  2. On every other ALPN: requires a peer cert, chain-verifies it
//     against the current trust bundle, enforces cluster-id match via
//     the leaf's URI SAN, and rejects revoked serials.
//
// The callback runs for both fresh and resumed TLS sessions.
func verifyConnection(bundleFn TrustBundleFunc, revokedFn RevokedFunc) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		if !RequiresClientCert(cs.NegotiatedProtocol) {
			return nil
		}

		if len(cs.PeerCertificates) == 0 {
			return fmt.Errorf("cluster TLS: peer presented no certificate for ALPN %q", cs.NegotiatedProtocol)
		}

		bundle := bundleFn()
		if bundle == nil {
			return fmt.Errorf("cluster TLS: trust bundle not yet available")
		}

		leaf := cs.PeerCertificates[0]

		if _, err := bundle.Verify(leaf, time.Now()); err != nil {
			return fmt.Errorf("cluster TLS: %w", err)
		}

		if revokedFn(leaf.SerialNumber) {
			return fmt.Errorf("cluster TLS: peer leaf serial %s has been revoked", leaf.SerialNumber)
		}

		return nil
	}
}

// PeerNodeID extracts the node-id from the peer certificate of an
// established TLS connection by re-running the trust bundle's identity
// extraction. Returns an error if the connection has no peer certificates
// or the URI SAN is malformed for the bundle's cluster-id.
func PeerNodeID(conn *tls.Conn, bundle *pki.TrustBundle) (int, error) {
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return 0, fmt.Errorf("cluster TLS: no peer certificates after handshake")
	}

	if bundle == nil {
		return 0, fmt.Errorf("cluster TLS: trust bundle unavailable")
	}

	return bundle.Verify(state.PeerCertificates[0], time.Now())
}

// PeerNodeID resolves the node-id of the peer cert on conn using the
// listener's current trust bundle. Used by cluster HTTP handlers that
// need peer identity without threading the bundle through every call
// site.
func (l *Listener) PeerNodeID(conn *tls.Conn) (int, error) {
	return PeerNodeID(conn, l.cfg.TrustBundle())
}
