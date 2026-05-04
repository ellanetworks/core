// Copyright 2026 Ella Networks

package listener

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/pki"
)

// genCert is a minimal helper duplicating what testutil.GenTestPKI
// would do, but kept in-package to avoid an import cycle from the
// listener_test/testutil/listener triangle. Any test that needs the
// full PKI helper lives in the external listener_test package.
type genCert struct {
	NodeID  int
	CertPEM []byte
}

func mintForTest(t *testing.T, nodeID int) genCert {
	t.Helper()

	cert, _, err := pki.GenerateNodeCert(nodeID, "test", time.Hour)
	if err != nil {
		t.Fatalf("generate cert: %v", err)
	}

	return genCert{NodeID: nodeID, CertPEM: pki.EncodeCertPEM(cert)}
}

func pinFromCerts(certs ...genCert) PinFunc {
	pins := make(map[string]int, len(certs))
	for _, c := range certs {
		parsed, _ := pki.ParseCertPEM(c.CertPEM)
		pins[pki.Fingerprint(parsed)] = c.NodeID
	}

	return func(fp string) PinResult {
		nid, ok := pins[fp]
		return PinResult{Found: ok, NodeID: nid}
	}
}

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
	c := mintForTest(t, 1)

	cb := verifyConnection(pinFromCerts(c))

	cs := tls.ConnectionState{
		NegotiatedProtocol: ALPNHTTP,
		PeerCertificates:   parseCerts(t, c.CertPEM),
	}

	if err := cb(cs); err != nil {
		t.Fatalf("pinned cert should verify: %v", err)
	}
}

func TestVerifyConnection_BootstrapALPN_NoCert(t *testing.T) {
	cb := verifyConnection(pinFromCerts())

	cs := tls.ConnectionState{NegotiatedProtocol: ALPNPKIBootstrap}

	if err := cb(cs); err != nil {
		t.Fatalf("bootstrap ALPN must pass without cert: %v", err)
	}
}

func TestVerifyConnection_NonBootstrap_NoCertRejected(t *testing.T) {
	cb := verifyConnection(pinFromCerts())

	cs := tls.ConnectionState{NegotiatedProtocol: ALPNHTTP}

	if err := cb(cs); err == nil {
		t.Fatal("non-bootstrap ALPN without peer cert must be rejected")
	}
}

func TestVerifyConnection_UnpinnedRejected(t *testing.T) {
	known := mintForTest(t, 1)
	stranger := mintForTest(t, 1)

	cb := verifyConnection(pinFromCerts(known))

	cs := tls.ConnectionState{
		NegotiatedProtocol: ALPNHTTP,
		PeerCertificates:   parseCerts(t, stranger.CertPEM),
	}

	if err := cb(cs); err == nil {
		t.Fatal("unpinned fingerprint must be rejected")
	}
}

func TestVerifyConnection_NodeIDMismatch(t *testing.T) {
	c := mintForTest(t, 2)

	mismatched := func(_ string) PinResult {
		return PinResult{Found: true, NodeID: 1}
	}

	cb := verifyConnection(mismatched)

	cs := tls.ConnectionState{
		NegotiatedProtocol: ALPNHTTP,
		PeerCertificates:   parseCerts(t, c.CertPEM),
	}

	if err := cb(cs); err == nil {
		t.Fatal("URI nodeID / pin owner mismatch must be rejected")
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

// Sanity-check that pki.Fingerprint matches what verifyConnection
// computes — the contract this whole subsystem hinges on.
func TestVerifyConnection_FingerprintMatchesPKIPackage(t *testing.T) {
	c := mintForTest(t, 1)

	cert := parseCerts(t, c.CertPEM)[0]

	fpFromPKI := pki.Fingerprint(cert)

	pinFn := pinFromCerts(c)

	if !pinFn(fpFromPKI).Found {
		t.Fatalf("pin lookup for %s should succeed", fpFromPKI)
	}
}
