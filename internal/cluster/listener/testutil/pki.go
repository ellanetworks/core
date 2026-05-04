// Copyright 2026 Ella Networks

// Package testutil mints self-signed cluster certs and a fingerprint
// pin map for wiring into listener.Config in unit and integration
// tests.
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

// PKI holds a fingerprint-pinning test universe: one self-signed cert
// per node-id under a single clusterID.
type PKI struct {
	ClusterID string

	Nodes map[int]NodeCert
}

// NodeCert holds a parsed cert and the matching tls.Certificate.
type NodeCert struct {
	NodeID  int
	Cert    *x509.Certificate
	Key     crypto.Signer
	TLSCert tls.Certificate
	CertPEM []byte
	KeyPEM  []byte
}

// PinFunc returns a closure compatible with listener.Config.Pin that
// resolves any fingerprint registered in the test PKI to its owner.
func (p *PKI) PinFunc() listener.PinFunc {
	pins := make(map[string]int, len(p.Nodes))
	for _, n := range p.Nodes {
		pins[pki.Fingerprint(n.Cert)] = n.NodeID
	}

	return func(fingerprint string) listener.PinResult {
		nid, ok := pins[fingerprint]
		return listener.PinResult{Found: ok, NodeID: nid}
	}
}

// LeafFunc returns a listener.Config.Leaf-compatible closure for nodeID.
// Panics if nodeID is not in the PKI.
func (p *PKI) LeafFunc(nodeID int) func() *tls.Certificate {
	n, ok := p.Nodes[nodeID]
	if !ok {
		panic(fmt.Sprintf("testutil: node %d not in PKI", nodeID))
	}

	c := n.TLSCert

	return func() *tls.Certificate { return &c }
}

// GenTestPKI creates a self-signed cert per node-id. All certs are
// valid for one hour, which is plenty for a unit test and avoids
// creating decade-long-lived test artifacts.
func GenTestPKI(t testing.TB, nodeIDs []int) *PKI {
	t.Helper()

	clusterID := "test-cluster"
	p := &PKI{ClusterID: clusterID, Nodes: make(map[int]NodeCert, len(nodeIDs))}

	for _, id := range nodeIDs {
		cert, key, err := pki.GenerateNodeCert(id, clusterID, time.Hour)
		if err != nil {
			t.Fatalf("generate cert for node %d: %v", id, err)
		}

		certPEM := pki.EncodeCertPEM(cert)

		keyPEM, err := pki.EncodePrivateKeyPEM(key)
		if err != nil {
			t.Fatalf("encode key for node %d: %v", id, err)
		}

		tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			t.Fatalf("X509KeyPair for node %d: %v", id, err)
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
