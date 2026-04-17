// Copyright 2026 Ella Networks

// Package testutil provides test helpers for the cluster listener package
// and its consumers. GenTestPKI generates a self-signed cluster CA and
// per-node leaf certificates for use in unit and integration tests.
package testutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"
)

// PKI holds a cluster CA and per-node leaf certificates.
type PKI struct {
	CAPool  *x509.CertPool
	CACert  *x509.Certificate
	CAKey   *ecdsa.PrivateKey
	CAPEM   []byte
	Nodes   map[int]NodeCert
	nextSer int64
}

// NodeCert holds a parsed leaf certificate and the PEM-encoded material.
type NodeCert struct {
	TLSCert tls.Certificate
	CertPEM []byte
	KeyPEM  []byte
}

// GenTestPKI creates a self-signed cluster CA and one leaf per node-id.
// Each leaf has CN=ella-node-<n>, EKU=serverAuth+clientAuth, and is
// signed by the CA. The CA and leaves are valid for 24 hours.
func GenTestPKI(t testing.TB, nodeIDs []int) *PKI {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ella-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}

	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	pki := &PKI{
		CAPool:  caPool,
		CACert:  caCert,
		CAKey:   caKey,
		CAPEM:   caPEM,
		Nodes:   make(map[int]NodeCert, len(nodeIDs)),
		nextSer: 2,
	}

	for _, id := range nodeIDs {
		pki.Nodes[id] = pki.genLeaf(t, id, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	}

	return pki
}

// GenLeafWithCN creates a leaf signed by this PKI's CA with an arbitrary
// CN. Useful for testing malformed CN rejection.
func (p *PKI) GenLeafWithCN(t testing.TB, cn string) NodeCert {
	t.Helper()

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key for CN %q: %v", cn, err)
	}

	ser := p.nextSer
	p.nextSer++

	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(ser),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, p.CACert, &leafKey.PublicKey, p.CAKey)
	if err != nil {
		t.Fatalf("create cert for CN %q: %v", cn, err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})

	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal key for CN %q: %v", cn, err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("load TLS cert for CN %q: %v", cn, err)
	}

	return NodeCert{TLSCert: tlsCert, CertPEM: certPEM, KeyPEM: keyPEM}
}

// GenExpiredLeaf creates a leaf certificate for the given node-id that
// expired one hour ago. Signed by the same CA so chain validation passes
// but temporal validation fails.
func (p *PKI) GenExpiredLeaf(t testing.TB, nodeID int) NodeCert {
	t.Helper()

	return p.genLeaf(t, nodeID, time.Now().Add(-48*time.Hour), time.Now().Add(-1*time.Hour))
}

func (p *PKI) genLeaf(t testing.TB, nodeID int, notBefore, notAfter time.Time) NodeCert {
	t.Helper()

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key for node %d: %v", nodeID, err)
	}

	cn := fmt.Sprintf("ella-node-%d", nodeID)
	ser := p.nextSer
	p.nextSer++

	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(ser),
		Subject:      pkix.Name{CommonName: cn},
		DNSNames:     []string{cn},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, p.CACert, &leafKey.PublicKey, p.CAKey)
	if err != nil {
		t.Fatalf("create cert for node %d: %v", nodeID, err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})

	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal key for node %d: %v", nodeID, err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("load TLS cert for node %d: %v", nodeID, err)
	}

	return NodeCert{TLSCert: tlsCert, CertPEM: certPEM, KeyPEM: keyPEM}
}
