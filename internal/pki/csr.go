// Copyright 2026 Ella Networks

package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net/url"
)

// GenerateKeyAndCSR produces a fresh ECDSA P-256 leaf key and a CSR with
// the Ella-Core cluster shape:
//
//	CN      = "ella-node-<n>"
//	URI SAN = "spiffe://cluster.ella/<cluster-id>/node/<n>"
//
// Returns both PEMs; callers persist the private key locally (0600) and
// submit the CSR PEM to the issuer. The key never leaves the caller's
// process before that.
func GenerateKeyAndCSR(nodeID int, clusterID string) (privPEM, csrPEM []byte, err error) {
	if nodeID < MinNodeID || nodeID > MaxNodeID {
		return nil, nil, fmt.Errorf("node-id %d outside [%d, %d]", nodeID, MinNodeID, MaxNodeID)
	}

	if clusterID == "" {
		return nil, nil, fmt.Errorf("clusterID must not be empty")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal key: %w", err)
	}

	privPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	tmpl := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: fmt.Sprintf("ella-node-%d", nodeID)},
		URIs:    []*url.URL{SpiffeID(clusterID, nodeID)},
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create csr: %w", err)
	}

	csrPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})

	return privPEM, csrPEM, nil
}

// ParseCSRPEM decodes a PEM-encoded CSR.
func ParseCSRPEM(csrPEM []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("not a CERTIFICATE REQUEST PEM block")
	}

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse csr: %w", err)
	}

	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("csr signature invalid: %w", err)
	}

	return csr, nil
}

// ParseCertPEM decodes a PEM-encoded certificate.
func ParseCertPEM(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("not a CERTIFICATE PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	return cert, nil
}

// EncodeCertPEM returns the PEM encoding of cert. Returns nil for a nil
// cert rather than panicking; callers treat a nil return as "no cert".
func EncodeCertPEM(cert *x509.Certificate) []byte {
	if cert == nil {
		return nil
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}
