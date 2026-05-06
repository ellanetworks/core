// Copyright 2026 Ella Networks

package listener

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/ellanetworks/core/internal/pki"
)

// PinResult is the outcome of a pin lookup. Found indicates the
// peer's fingerprint is registered; NodeID is the registered
// owner. CacheSize and KnownNodeIDs describe the cache state at
// the moment of the lookup; the verifier includes them in
// rejection messages so a stale-cache failure is one log line to
// diagnose.
type PinResult struct {
	Found        bool
	NodeID       int
	CacheSize    int
	KnownNodeIDs []int
}

// PinFunc resolves a peer cert's fingerprint against the local
// cache of cluster_node_certs. Called once per non-bootstrap
// handshake.
type PinFunc func(fingerprint string) PinResult

// verifyConnection returns a tls.Config.VerifyConnection callback
// that enforces fingerprint pinning. Bootstrap-ALPN connections
// pass through (auth is the join-token HMAC in the request body).
func verifyConnection(pinFn PinFunc) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		if !RequiresClientCert(cs.NegotiatedProtocol) {
			return nil
		}

		if len(cs.PeerCertificates) == 0 {
			return fmt.Errorf("cluster TLS: peer presented no certificate for ALPN %q", cs.NegotiatedProtocol)
		}

		leaf := cs.PeerCertificates[0]

		fp := pki.Fingerprint(leaf)

		res := pinFn(fp)
		if !res.Found {
			return fmt.Errorf("cluster TLS: peer fingerprint %s is not pinned (cache size %d, known nodeIDs %v)",
				fp, res.CacheSize, res.KnownNodeIDs)
		}

		// The URI SAN's nodeID must match the pin's owner.
		// Defends against a stolen key being replayed under a
		// different nodeID if the pin map ever drifts.
		_, certNodeID, err := pki.IdentityFromCert(leaf)
		if err != nil {
			return fmt.Errorf("cluster TLS: %w", err)
		}

		if certNodeID != res.NodeID {
			return fmt.Errorf("cluster TLS: cert URI nodeID %d != pin owner %d", certNodeID, res.NodeID)
		}

		return nil
	}
}

// PeerNodeID returns the peer's nodeID by parsing the SPIFFE URI
// SAN of its leaf. The leaf has already been pinned by
// verifyConnection during the handshake, so no DB lookup is needed.
func PeerNodeID(conn *tls.Conn) (int, error) {
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return 0, fmt.Errorf("cluster TLS: no peer certificates after handshake")
	}

	return peerNodeIDFromCert(state.PeerCertificates[0])
}

func peerNodeIDFromCert(cert *x509.Certificate) (int, error) {
	_, nodeID, err := pki.IdentityFromCert(cert)
	if err != nil {
		return 0, err
	}

	return nodeID, nil
}

func (l *Listener) PeerNodeID(conn *tls.Conn) (int, error) {
	return PeerNodeID(conn)
}
