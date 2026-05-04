// Copyright 2026 Ella Networks

package listener

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/ellanetworks/core/internal/pki"
)

// PinResult is what the pin lookup returns. Found means the peer's
// fingerprint is registered in the cluster_node_certs table; NodeID
// carries the registered owner so the caller can compare against an
// expected peer ID.
type PinResult struct {
	Found  bool
	NodeID int
}

// PinFunc looks up a peer cert's fingerprint in the local cache of
// cluster_node_certs. The cache is rebuilt periodically by the runtime
// from the replicated DB. The hot path here must be allocation-free
// where possible — runs once per handshake.
type PinFunc func(fingerprint string) PinResult

// verifyConnection returns a tls.Config.VerifyConnection callback that
// enforces fingerprint pinning. The bootstrap ALPN (no peer cert) is
// allowed through; auth on that path is the join-token HMAC in the
// request body.
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
			return fmt.Errorf("cluster TLS: peer fingerprint %s is not pinned", fp)
		}

		// Belt-and-braces: the URI SAN's nodeID must match the
		// registered owner. Catches a stolen-key-replayed-against-
		// different-nodeID attempt should our cache ever drift.
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

// PeerNodeID extracts the node-id from the peer certificate of an
// established TLS connection by parsing its SPIFFE URI SAN. No DB
// lookup; verifyConnection has already pinned the cert during the
// handshake. Returns an error if the connection has no peer
// certificates or the URI SAN is malformed.
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

// PeerNodeID resolves the node-id of the peer cert on conn. Provided as
// a method for callers that hold a *Listener handle.
func (l *Listener) PeerNodeID(conn *tls.Conn) (int, error) {
	return PeerNodeID(conn)
}
