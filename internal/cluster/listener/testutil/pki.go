// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package testutil mints self-signed cluster certs and a
// fingerprint pin map for wiring into listener.Config in unit and
// integration tests.
package testutil

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/pki"
)

// PKI holds the test cluster identity: one self-signed cert per
// node-id under a shared clusterID, plus the matching pin map.
type PKI struct {
	ClusterID string

	Nodes map[string]NodeCert
}

// NodeCert holds a parsed cert and the matching tls.Certificate.
type NodeCert struct {
	NodeID  string
	Cert    *x509.Certificate
	Key     crypto.Signer
	TLSCert tls.Certificate
	CertPEM []byte
	KeyPEM  []byte
}

// PinFunc returns a listener.Config.Pin closure that resolves any
// fingerprint registered in the test PKI to its owner.
func (p *PKI) PinFunc() listener.PinFunc {
	pins := make(map[string]string, len(p.Nodes))
	for _, n := range p.Nodes {
		pins[pki.Fingerprint(n.Cert)] = n.NodeID
	}

	known := make([]string, 0, len(pins))
	for _, n := range pins {
		known = append(known, n)
	}

	return func(fingerprint string) listener.PinResult {
		nid, ok := pins[fingerprint]

		return listener.PinResult{
			Found:        ok,
			NodeID:       nid,
			CacheSize:    len(pins),
			KnownNodeIDs: known,
		}
	}
}

// LeafFunc returns a listener.Config.Leaf closure for nodeID.
// Panics if nodeID is not in the PKI.
func (p *PKI) LeafFunc(nodeID string) func() *tls.Certificate {
	n, ok := p.Nodes[nodeID]
	if !ok {
		panic(fmt.Sprintf("testutil: node %s not in PKI", nodeID))
	}

	c := n.TLSCert

	return func() *tls.Certificate { return &c }
}

// GenTestPKI creates a self-signed cert per node-id, valid for one
// hour. Sufficient for unit tests; avoids minting decade-long
// artifacts during runs.
func GenTestPKI(t testing.TB, nodeIDs []string) *PKI {
	t.Helper()

	clusterID := "test-cluster"
	p := &PKI{ClusterID: clusterID, Nodes: make(map[string]NodeCert, len(nodeIDs))}

	for _, id := range nodeIDs {
		cert, key, err := pki.GenerateNodeCert(id, clusterID, time.Hour)
		if err != nil {
			t.Fatalf("generate cert for node %s: %v", id, err)
		}

		certPEM := pki.EncodeCertPEM(cert)

		keyPEM, err := pki.EncodePrivateKeyPEM(key)
		if err != nil {
			t.Fatalf("encode key for node %s: %v", id, err)
		}

		tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			t.Fatalf("X509KeyPair for node %s: %v", id, err)
		}

		p.Nodes[id] = NodeCert{
			NodeID:  id,
			Cert:    cert,
			Key:     key,
			TLSCert: tlsCert,
			CertPEM: certPEM,
			KeyPEM:  keyPEM,
		}
	}

	return p
}
