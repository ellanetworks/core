// Copyright 2026 Ella Networks

package pki_test

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/pki"
)

func TestGenerateRootAndIntermediate(t *testing.T) {
	root, rootKey, err := pki.GenerateRoot("cluster-abc", 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateRoot: %v", err)
	}

	if !root.IsCA {
		t.Fatal("root must be CA")
	}

	if root.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Fatal("root missing KeyUsageCertSign")
	}

	intCert, _, err := pki.GenerateIntermediate("cluster-abc", root, rootKey, 12*time.Hour)
	if err != nil {
		t.Fatalf("GenerateIntermediate: %v", err)
	}

	if !intCert.IsCA {
		t.Fatal("intermediate must be CA")
	}

	if !intCert.MaxPathLenZero {
		t.Fatal("intermediate must have MaxPathLen=0")
	}

	// Verify intermediate chains to root.
	roots := x509.NewCertPool()
	roots.AddCert(root)

	if _, err := intCert.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		t.Fatalf("intermediate -> root chain: %v", err)
	}
}

func TestSignLeaf_HappyPath(t *testing.T) {
	clusterID := "test-cluster"

	root, rootKey, err := pki.GenerateRoot(clusterID, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	intCert, intKey, err := pki.GenerateIntermediate(clusterID, root, rootKey, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	issuer := pki.NewIssuer(intCert, intKey, clusterID)

	_, csrPEM, err := pki.GenerateKeyAndCSR(7, clusterID)
	if err != nil {
		t.Fatal(err)
	}

	csr, err := pki.ParseCSRPEM(csrPEM)
	if err != nil {
		t.Fatal(err)
	}

	leafPEM, err := issuer.SignLeaf(csr, 7, 1001, time.Hour)
	if err != nil {
		t.Fatalf("SignLeaf: %v", err)
	}

	leaf, err := pki.ParseCertPEM(leafPEM)
	if err != nil {
		t.Fatal(err)
	}

	if leaf.SerialNumber.Uint64() != 1001 {
		t.Fatalf("serial = %d, want 1001", leaf.SerialNumber.Uint64())
	}

	// 60s backdate + 1h TTL = 1h01m window.
	total := leaf.NotAfter.Sub(leaf.NotBefore)
	if total < time.Hour+50*time.Second || total > time.Hour+70*time.Second {
		t.Fatalf("validity window = %s, want ~1h01m", total)
	}

	// Chain-verify through the bundle.
	bundle := &pki.TrustBundle{
		Roots:         []*x509.Certificate{root},
		Intermediates: []*x509.Certificate{intCert},
		ClusterID:     clusterID,
	}

	nodeID, err := bundle.Verify(leaf, time.Now())
	if err != nil {
		t.Fatalf("bundle.Verify: %v", err)
	}

	if nodeID != 7 {
		t.Fatalf("verified node-id = %d, want 7", nodeID)
	}
}

func TestSignLeaf_Rejections(t *testing.T) {
	clusterID := "test-cluster"

	root, rootKey, _ := pki.GenerateRoot(clusterID, 24*time.Hour)
	intCert, intKey, _ := pki.GenerateIntermediate(clusterID, root, rootKey, 24*time.Hour)
	issuer := pki.NewIssuer(intCert, intKey, clusterID)

	cases := []struct {
		name    string
		nodeID  int
		csrNode int
		ttl     time.Duration
		wantErr string
	}{
		{"ttl too short", 5, 5, 30 * time.Minute, "outside"},
		{"ttl too long", 5, 5, 30 * 24 * time.Hour, "outside"},
		{"nodeID mismatch", 5, 9, time.Hour, "does not match"},
		{"nodeID out of range high", 100, 5, time.Hour, "outside"},
		{"nodeID zero", 0, 5, time.Hour, "outside"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, csrPEM, err := pki.GenerateKeyAndCSR(tc.csrNode, clusterID)
			if err != nil {
				t.Fatalf("GenerateKeyAndCSR: %v", err)
			}

			csr, err := pki.ParseCSRPEM(csrPEM)
			if err != nil {
				t.Fatal(err)
			}

			_, err = issuer.SignLeaf(csr, tc.nodeID, 1, tc.ttl)
			if err == nil {
				t.Fatal("expected error")
			}

			if !contains(err.Error(), tc.wantErr) {
				t.Fatalf("err %q missing substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestFingerprint(t *testing.T) {
	root, _, _ := pki.GenerateRoot("x", 24*time.Hour)

	fp := pki.Fingerprint(root)
	if len(fp) != len("sha256:")+64 {
		t.Fatalf("unexpected fingerprint length: %d", len(fp))
	}

	if fp[:7] != "sha256:" {
		t.Fatalf("fingerprint missing sha256: prefix")
	}

	// Deterministic.
	if pki.Fingerprint(root) != fp {
		t.Fatal("fingerprint is not deterministic")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}
