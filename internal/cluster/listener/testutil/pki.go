// Copyright 2026 Ella Networks

// Package testutil generates test PKI material shaped like a real cluster
// CA hierarchy: a self-signed root, an active intermediate signed by the
// root, and per-node leaves with the expected URI SAN. Used by unit and
// integration tests across the listener, raft, and API packages.
package testutil

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/pki"
)

// PKI holds a test cluster hierarchy (root + intermediate + per-node
// leaves) suitable for wiring into listener.Config.
type PKI struct {
	ClusterID string

	Root            *x509.Certificate
	RootKey         crypto.Signer
	Intermediate    *x509.Certificate
	IntermediateKey crypto.Signer

	Nodes map[int]NodeCert

	nextSerial int64
}

// NodeCert holds a parsed leaf certificate and the PEM-encoded material.
type NodeCert struct {
	TLSCert tls.Certificate
	CertPEM []byte
	KeyPEM  []byte
	NodeID  int
}

// Bundle returns a pki.TrustBundle anchored at the test root, with the
// active intermediate. Safe to pass to listener.Config.TrustBundle via a
// closure.
func (p *PKI) Bundle() *pki.TrustBundle {
	return &pki.TrustBundle{
		Roots:         []*x509.Certificate{p.Root},
		Intermediates: []*x509.Certificate{p.Intermediate},
		ClusterID:     p.ClusterID,
	}
}

// BundleFunc returns a listener.TrustBundleFunc-compatible closure.
func (p *PKI) BundleFunc() func() *pki.TrustBundle {
	b := p.Bundle()
	return func() *pki.TrustBundle { return b }
}

// LeafFunc returns a closure compatible with listener.Config.Leaf that
// always returns the leaf for nodeID. Panics at call time if nodeID is
// not in the PKI.
func (p *PKI) LeafFunc(nodeID int) func() *tls.Certificate {
	n, ok := p.Nodes[nodeID]
	if !ok {
		panic(fmt.Sprintf("testutil: leaf for node %d not in PKI", nodeID))
	}

	c := n.TLSCert

	return func() *tls.Certificate { return &c }
}

// GenTestPKI creates a test cluster hierarchy with one leaf per node-id.
// All certs are valid for 24h.
func GenTestPKI(t testing.TB, nodeIDs []int) *PKI {
	t.Helper()

	clusterID := "test-cluster"

	root, rootKey, err := pki.GenerateRoot(clusterID, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate root: %v", err)
	}

	intermediate, intermediateKey, err := pki.GenerateIntermediate(clusterID, root, rootKey, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate intermediate: %v", err)
	}

	p := &PKI{
		ClusterID:       clusterID,
		Root:            root,
		RootKey:         rootKey,
		Intermediate:    intermediate,
		IntermediateKey: intermediateKey,
		Nodes:           make(map[int]NodeCert, len(nodeIDs)),
		nextSerial:      1,
	}

	for _, id := range nodeIDs {
		p.Nodes[id] = p.genLeaf(t, id)
	}

	return p
}

func (p *PKI) genLeaf(t testing.TB, nodeID int) NodeCert {
	t.Helper()

	keyPEM, csrPEM, err := pki.GenerateKeyAndCSR(nodeID, p.ClusterID)
	if err != nil {
		t.Fatalf("generate key+csr for node %d: %v", nodeID, err)
	}

	csr, err := pki.ParseCSRPEM(csrPEM)
	if err != nil {
		t.Fatalf("parse csr for node %d: %v", nodeID, err)
	}

	issuer := pki.NewIssuer(p.Intermediate, p.IntermediateKey, p.ClusterID)

	serial := p.nextSerial
	p.nextSerial++

	leafPEM, err := issuer.SignLeaf(csr, nodeID, uint64(serial), time.Hour)
	if err != nil {
		t.Fatalf("sign leaf for node %d: %v", nodeID, err)
	}

	tlsCert, err := tls.X509KeyPair(leafPEM, keyPEM)
	if err != nil {
		t.Fatalf("load TLS cert for node %d: %v", nodeID, err)
	}

	return NodeCert{TLSCert: tlsCert, CertPEM: leafPEM, KeyPEM: keyPEM, NodeID: nodeID}
}
