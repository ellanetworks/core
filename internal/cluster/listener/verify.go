// Copyright 2026 Ella Networks

package listener

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strconv"
	"strings"
)

const (
	cnPrefix  = "ella-node-"
	minNodeID = 1
	maxNodeID = 63
)

// verifyConnection returns a tls.Config.VerifyConnection callback that
// enforces the cluster verification rules (spec §7):
//
//  1. Chain validates against caPool.
//  2. Leaf NotBefore <= now <= NotAfter (handled by x509.Verify).
//  3. Leaf CN = "ella-node-<n>" with n in [1, 63].
//
// VerifyConnection runs for both fresh and resumed TLS sessions,
// unlike VerifyPeerCertificate which is skipped on resumption.
func verifyConnection(caPool *x509.CertPool) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return fmt.Errorf("cluster TLS: peer presented no certificates")
		}

		leaf := cs.PeerCertificates[0]

		intermediates := x509.NewCertPool()
		for _, c := range cs.PeerCertificates[1:] {
			intermediates.AddCert(c)
		}

		if _, err := leaf.Verify(x509.VerifyOptions{
			Roots:         caPool,
			Intermediates: intermediates,
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		}); err != nil {
			return fmt.Errorf("cluster TLS: peer certificate chain verification failed: %w", err)
		}

		if _, err := parseNodeCN(leaf.Subject.CommonName); err != nil {
			return fmt.Errorf("cluster TLS: %w", err)
		}

		return nil
	}
}

// PeerNodeID extracts the node-id from the peer certificate of an
// established TLS connection. Returns an error if the connection has
// no peer certificates or the CN is malformed.
func PeerNodeID(conn *tls.Conn) (int, error) {
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return 0, fmt.Errorf("cluster TLS: no peer certificates after handshake")
	}

	return parseNodeCN(state.PeerCertificates[0].Subject.CommonName)
}

// parseNodeCN extracts the integer node-id from a CN of the form
// "ella-node-<n>" where n is in [1, 63]. Returns an error if the CN
// is malformed or out of range.
func parseNodeCN(cn string) (int, error) {
	if !strings.HasPrefix(cn, cnPrefix) {
		return 0, fmt.Errorf("peer CN %q does not start with %q", cn, cnPrefix)
	}

	numStr := cn[len(cnPrefix):]

	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("peer CN %q has non-integer node-id suffix", cn)
	}

	if n < minNodeID || n > maxNodeID {
		return 0, fmt.Errorf("peer CN %q has node-id %d outside valid range [%d, %d]", cn, n, minNodeID, maxNodeID)
	}

	return n, nil
}
