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
	"encoding/pem"
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

// RevokedFuncNone returns a listener.RevokedFunc that always returns
// false. Convenience for tests that don't exercise revocation.
func RevokedFuncNone() func(*bigIntAlias) bool {
	return func(*bigIntAlias) bool { return false }
}

// LeafFunc returns a listener.LeafFunc-compatible closure that always
// returns the leaf for nodeID. Panics at call time if nodeID is not in
// the PKI.
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

// GenLeafForNode creates an additional leaf for nodeID, mimicking a
// renewal. The old leaf (if any) is left in the map under its previous
// serial; callers that need renewal semantics replace the entry.
func (p *PKI) GenLeafForNode(t testing.TB, nodeID int) NodeCert {
	t.Helper()

	return p.genLeaf(t, nodeID)
}

// GenLeafWithCN produces a leaf signed by the PKI's intermediate but with
// an arbitrary CN (no URI SAN). Useful for testing malformed-identity
// rejection.
func (p *PKI) GenLeafWithCN(t testing.TB, cn string) NodeCert {
	t.Helper()

	// Build a raw CSR-like template bypassing the pki.Issuer validator
	// since we intentionally want a malformed cert. Reuse the intermediate
	// to sign.
	return p.forgeLeaf(t, cn, nil)
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

// forgeLeaf builds a signed leaf with the given CN and URI SAN, using the
// intermediate's private key directly so we can emit a chain-valid but
// identity-malformed cert.
func (p *PKI) forgeLeaf(t testing.TB, cn string, _ []string) NodeCert {
	t.Helper()

	// Generate a leaf key + CSR with a valid shape, then replace the
	// parsed CSR's CN and URIs. This bypasses the pki package's
	// validator when signing.
	keyPEM, csrPEM, _ := pki.GenerateKeyAndCSR(1, p.ClusterID)
	csr, _ := pki.ParseCSRPEM(csrPEM)

	// Manually construct an x509 leaf bypassing pki.Issuer's validator.
	serial := p.nextSerial
	p.nextSerial++

	now := time.Now().Add(-time.Minute)

	tmpl := &x509.Certificate{
		SerialNumber: bigIntFrom(serial),
		Subject:      commonName(cn),
		NotBefore:    now,
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	der, err := x509.CreateCertificate(randReader(), tmpl, p.Intermediate, csr.PublicKey, p.IntermediateKey)
	if err != nil {
		t.Fatalf("forge leaf: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("load forged TLS cert: %v", err)
	}

	return NodeCert{TLSCert: tlsCert, CertPEM: certPEM, KeyPEM: keyPEM, NodeID: -1}
}
