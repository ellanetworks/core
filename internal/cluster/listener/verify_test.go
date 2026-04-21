// Copyright 2026 Ella Networks

package listener

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"

	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
	"github.com/ellanetworks/core/internal/pki"
)

func parseCerts(t *testing.T, certPEM []byte) []*x509.Certificate {
	t.Helper()

	var certs []*x509.Certificate

	rest := certPEM

	for {
		var block *pem.Block

		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatalf("parse cert: %v", err)
		}

		certs = append(certs, c)
	}

	return certs
}

func TestVerifyConnection_HappyPath(t *testing.T) {
	p := testutil.GenTestPKI(t, []int{1})
	bundle := p.Bundle()

	cb := verifyConnection(func() *pki.TrustBundle { return bundle }, func(*big.Int) bool { return false })

	cs := tls.ConnectionState{
		NegotiatedProtocol: ALPNHTTP,
		PeerCertificates:   parseCerts(t, p.Nodes[1].CertPEM),
	}

	if err := cb(cs); err != nil {
		t.Fatalf("valid leaf should verify: %v", err)
	}
}

func TestVerifyConnection_BootstrapALPN_NoCert(t *testing.T) {
	p := testutil.GenTestPKI(t, []int{1})
	bundle := p.Bundle()

	cb := verifyConnection(func() *pki.TrustBundle { return bundle }, func(*big.Int) bool { return false })

	// Bootstrap ALPN without any peer cert is allowed.
	cs := tls.ConnectionState{
		NegotiatedProtocol: ALPNPKIBootstrap,
	}

	if err := cb(cs); err != nil {
		t.Fatalf("bootstrap ALPN must pass without cert: %v", err)
	}
}

func TestVerifyConnection_NonBootstrap_NoCertRejected(t *testing.T) {
	p := testutil.GenTestPKI(t, []int{1})
	bundle := p.Bundle()

	cb := verifyConnection(func() *pki.TrustBundle { return bundle }, func(*big.Int) bool { return false })

	cs := tls.ConnectionState{NegotiatedProtocol: ALPNHTTP}

	if err := cb(cs); err == nil {
		t.Fatal("non-bootstrap ALPN without peer cert must be rejected")
	}
}

func TestVerifyConnection_Revoked(t *testing.T) {
	p := testutil.GenTestPKI(t, []int{1})
	bundle := p.Bundle()

	certs := parseCerts(t, p.Nodes[1].CertPEM)
	serial := certs[0].SerialNumber

	cb := verifyConnection(
		func() *pki.TrustBundle { return bundle },
		func(s *big.Int) bool { return s.Cmp(serial) == 0 },
	)

	cs := tls.ConnectionState{
		NegotiatedProtocol: ALPNHTTP,
		PeerCertificates:   certs,
	}

	if err := cb(cs); err == nil {
		t.Fatal("revoked leaf must be rejected")
	}
}

func TestVerifyConnection_WrongCluster(t *testing.T) {
	p := testutil.GenTestPKI(t, []int{1})

	otherBundle := &pki.TrustBundle{
		Roots:         p.Bundle().Roots,
		Intermediates: p.Bundle().Intermediates,
		ClusterID:     "other-cluster",
	}

	cb := verifyConnection(func() *pki.TrustBundle { return otherBundle }, func(*big.Int) bool { return false })

	cs := tls.ConnectionState{
		NegotiatedProtocol: ALPNHTTP,
		PeerCertificates:   parseCerts(t, p.Nodes[1].CertPEM),
	}

	if err := cb(cs); err == nil {
		t.Fatal("leaf from a different cluster must be rejected")
	}
}

func TestVerifyConnection_BundleUnavailable(t *testing.T) {
	cb := verifyConnection(func() *pki.TrustBundle { return nil }, func(*big.Int) bool { return false })

	cs := tls.ConnectionState{
		NegotiatedProtocol: ALPNHTTP,
		PeerCertificates:   []*x509.Certificate{{}},
	}

	if err := cb(cs); err == nil {
		t.Fatal("missing bundle must fail")
	}
}

func TestRequiresClientCert(t *testing.T) {
	if !RequiresClientCert(ALPNHTTP) {
		t.Fatal("http ALPN must require client cert")
	}

	if !RequiresClientCert(ALPNRaft) {
		t.Fatal("raft ALPN must require client cert")
	}

	if RequiresClientCert(ALPNPKIBootstrap) {
		t.Fatal("bootstrap ALPN must NOT require client cert")
	}
}
