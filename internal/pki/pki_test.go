// Copyright 2026 Ella Networks

package pki_test

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/pki"
)

func TestGenerateNodeCert_RoundTrip(t *testing.T) {
	cert, key, err := pki.GenerateNodeCert(2, "test-cluster", time.Hour)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if cert.Subject.CommonName != "ella-node-2" {
		t.Errorf("CN = %q, want ella-node-2", cert.Subject.CommonName)
	}

	clusterID, nodeID, err := pki.IdentityFromCert(cert)
	if err != nil {
		t.Fatalf("identity: %v", err)
	}

	if clusterID != "test-cluster" || nodeID != 2 {
		t.Errorf("identity = (%q, %d), want (test-cluster, 2)", clusterID, nodeID)
	}

	// Self-signed: issuer == subject.
	if string(cert.RawIssuer) != string(cert.RawSubject) {
		t.Errorf("not self-signed (issuer != subject)")
	}

	// Round-trip via PEM and tls.X509KeyPair.
	certPEM := pki.EncodeCertPEM(cert)

	keyPEM, err := pki.EncodePrivateKeyPEM(key)
	if err != nil {
		t.Fatalf("encode key: %v", err)
	}

	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		t.Errorf("tls.X509KeyPair: %v", err)
	}
}

func TestFingerprint_Stable(t *testing.T) {
	cert, _, err := pki.GenerateNodeCert(1, "c", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	fp := pki.Fingerprint(cert)
	if !strings.HasPrefix(fp, "sha256:") {
		t.Fatalf("missing prefix: %q", fp)
	}

	expected := sha256.Sum256(cert.Raw)
	if fp != "sha256:"+hex.EncodeToString(expected[:]) {
		t.Errorf("fingerprint mismatch")
	}

	raw, err := pki.ParseFingerprint(fp)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if string(raw) != string(expected[:]) {
		t.Errorf("parsed bytes != source")
	}
}

func TestIdentityFromCert_RoundTrip(t *testing.T) {
	cert, _, err := pki.GenerateNodeCert(5, "abc", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	cid, nid, err := pki.IdentityFromCert(cert)
	if err != nil {
		t.Fatalf("identity: %v", err)
	}

	if cid != "abc" || nid != 5 {
		t.Errorf("got (%s, %d) want (abc, 5)", cid, nid)
	}
}

func TestGenerateNodeCert_Bounds(t *testing.T) {
	if _, _, err := pki.GenerateNodeCert(0, "c", time.Hour); err == nil {
		t.Error("expected error for nodeID=0")
	}

	if _, _, err := pki.GenerateNodeCert(64, "c", time.Hour); err == nil {
		t.Error("expected error for nodeID=64")
	}

	if _, _, err := pki.GenerateNodeCert(1, "", time.Hour); err == nil {
		t.Error("expected error for empty clusterID")
	}
}
