// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/pki"
)

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCA(t *testing.T, name string) testCA {
	t.Helper()

	return newTestCAUntil(t, name, time.Now().Add(24*time.Hour))
}

func newTestCAUntil(t *testing.T, name string, notAfter time.Time) testCA {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return testCA{cert: cert, key: key}
}

type leafOpts struct {
	notBefore, notAfter time.Time
	eku                 []x509.ExtKeyUsage
}

func (ca testCA) issue(t *testing.T, dnsNames []string, opts ...leafOpts) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	o := leafOpts{notBefore: time.Now().Add(-time.Hour), notAfter: time.Now().Add(12 * time.Hour), eku: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	if len(opts) > 0 {
		o = opts[0]
	}

	cn := "leaf"
	if len(dnsNames) > 0 {
		cn = dnsNames[0]
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    o.notBefore,
		NotAfter:     o.notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  o.eku,
		DNSNames:     dnsNames,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return cert, key
}

func (ca testCA) pem() string {
	return string(pki.EncodeCertPEM(ca.cert))
}

func identityFor(t *testing.T, ca testCA, fqdn string, opts ...leafOpts) IdentityConfig {
	t.Helper()

	leaf, key := ca.issue(t, []string{fqdn}, opts...)

	keyPEM, err := pki.EncodePrivateKeyPEM(key)
	if err != nil {
		t.Fatal(err)
	}

	seppID := 1

	if label, _, ok := strings.Cut(fqdn, "."); ok {
		if n, err := strconv.Atoi(strings.TrimPrefix(label, "sepp")); err == nil {
			seppID = n
		}
	}

	return IdentityConfig{SEPPID: seppID, FQDN: fqdn, CertChainPEM: string(pki.EncodeCertPEM(leaf)) + ca.pem(), KeyPEM: string(keyPEM)}
}

func tlsCert(t *testing.T, ca testCA, leaf *x509.Certificate, key *ecdsa.PrivateKey) tls.Certificate {
	t.Helper()

	return tls.Certificate{Certificate: [][]byte{leaf.Raw, ca.cert.Raw}, PrivateKey: key, Leaf: leaf}
}
