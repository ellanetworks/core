// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/pki"
)

func signCert(t *testing.T, tmpl, parent *x509.Certificate, pub crypto.PublicKey, signer crypto.Signer) *x509.Certificate {
	t.Helper()

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, signer)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return cert
}

func leafTemplate() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: partnerAFQDN},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(12 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{partnerAFQDN},
	}
}

func intermediateTemplate(name string) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(12 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
}

func TestCheckAsKeyStrength(t *testing.T) {
	ca := newTestCA(t, "A")
	partner := mustTrusted(t, "a", partnerAFQDN, ca.pem(), pki.PLMN{MCC: "001", MNC: "01"})

	rsa1536, err := rsa.GenerateKey(rand.Reader, 1536)
	if err != nil {
		t.Fatal(err)
	}

	rsa2048, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	p224, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	p256, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	edPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	leafWith := func(pub crypto.PublicKey) []*x509.Certificate {
		return []*x509.Certificate{signCert(t, leafTemplate(), ca.cert, pub, ca.key), ca.cert}
	}

	weakIntermediate := signCert(t, intermediateTemplate("weak intermediate"), ca.cert, &rsa1536.PublicKey, ca.key)
	viaWeakIntermediate := []*x509.Certificate{signCert(t, leafTemplate(), weakIntermediate, &p256.PublicKey, rsa1536), weakIntermediate, ca.cert}

	strongIntermediate := signCert(t, intermediateTemplate("strong intermediate"), ca.cert, &p384.PublicKey, ca.key)
	viaStrongIntermediate := []*x509.Certificate{signCert(t, leafTemplate(), strongIntermediate, &p256.PublicKey, p384), strongIntermediate, ca.cert}

	for name, tc := range map[string]struct {
		certs  []*x509.Certificate
		reason string
	}{
		"ECDSA P-256 leaf":         {leafWith(&p256.PublicKey), ""},
		"ECDSA P-384 leaf":         {leafWith(&p384.PublicKey), ""},
		"RSA 2048 leaf":            {leafWith(&rsa2048.PublicKey), ""},
		"Ed25519 leaf":             {leafWith(edPub), ""},
		"RSA 1536 leaf":            {leafWith(&rsa1536.PublicKey), TLSReasonWeakKey},
		"ECDSA P-224 leaf":         {leafWith(&p224.PublicKey), TLSReasonWeakKey},
		"RSA 1536 intermediate":    {viaWeakIntermediate, TLSReasonWeakKey},
		"ECDSA P-384 intermediate": {viaStrongIntermediate, ""},
	} {
		for _, usage := range []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth} {
			err := partner.checkAs(tc.certs, time.Now(), usage, partnerAFQDN)

			switch {
			case tc.reason == "" && err != nil:
				t.Errorf("%s (usage %d): got %v, want accepted", name, usage, err)
			case tc.reason != "" && (err == nil || TLSFailureReason(err) != tc.reason || !errors.Is(err, ErrPeerCertificate)):
				t.Errorf("%s (usage %d): got %v, want reason %s", name, usage, err, tc.reason)
			}
		}
	}
}

func TestIdentifyPeerReportsWeakKey(t *testing.T) {
	caA, caB := newTestCA(t, "A"), newTestCA(t, "B")
	a := mustTrusted(t, "a", partnerAFQDN, caA.pem(), pki.PLMN{MCC: "001", MNC: "01"})
	b := mustTrusted(t, "b", partnerBFQDN, caB.pem(), pki.PLMN{MCC: "002", MNC: "10"})

	rsa1536, err := rsa.GenerateKey(rand.Reader, 1536)
	if err != nil {
		t.Fatal(err)
	}

	certs := []*x509.Certificate{signCert(t, leafTemplate(), caA.cert, &rsa1536.PublicKey, caA.key), caA.cert}

	_, err = identifyPeer([]*trustedPartner{a, b}, certs, time.Now(), x509.ExtKeyUsageClientAuth)
	if TLSFailureReason(err) != TLSReasonWeakKey {
		t.Fatalf("got %v, want reason %s", err, TLSReasonWeakKey)
	}
}
