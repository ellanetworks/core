// Copyright 2026 Ella Networks

package pki_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/url"
	"testing"
	"time"
)

// forgeCert produces a signed x509 cert with the given CN and URIs,
// chained to signerCert with signerKey. Used in tests to simulate
// malformed CSR/cert inputs that the real CSR validator should reject.
func forgeCert(t *testing.T, cn string, uris []*url.URL, signerCert *x509.Certificate, signerKey crypto.Signer) *x509.Certificate {
	t.Helper()

	subjectKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(123),
		Subject:      pkix.Name{CommonName: cn},
		URIs:         uris,
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, signerCert, &subjectKey.PublicKey, signerKey)
	if err != nil {
		t.Fatal(err)
	}

	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return c
}
