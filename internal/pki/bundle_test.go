// Copyright 2026 Ella Networks

package pki_test

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/pki"
)

// helper: mint a fresh cluster + leaf and return both.
func mintLeaf(t *testing.T, clusterID string, nodeID int) (bundle *pki.TrustBundle, leaf *x509.Certificate) {
	t.Helper()

	root, rootKey, err := pki.GenerateRoot(clusterID, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	intCert, intKey, err := pki.GenerateIntermediate(clusterID, root, rootKey, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	issuer := pki.NewIssuer(intCert, intKey, clusterID)

	_, csrPEM, err := pki.GenerateKeyAndCSR(nodeID, clusterID)
	if err != nil {
		t.Fatal(err)
	}

	csr, err := pki.ParseCSRPEM(csrPEM)
	if err != nil {
		t.Fatal(err)
	}

	leafPEM, err := issuer.SignLeaf(csr, nodeID, 1, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	leaf, err = pki.ParseCertPEM(leafPEM)
	if err != nil {
		t.Fatal(err)
	}

	return &pki.TrustBundle{
		Roots:         []*x509.Certificate{root},
		Intermediates: []*x509.Certificate{intCert},
		ClusterID:     clusterID,
	}, leaf
}

func TestBundle_Verify_CrossCluster(t *testing.T) {
	// Leaf minted under cluster "A". Bundle anchored at cluster "B".
	// Even if the CA chain validates, the cluster-id mismatch must reject.
	bundleA, leafA := mintLeaf(t, "cluster-A", 3)
	bundleB, _ := mintLeaf(t, "cluster-B", 3)

	// Make bundleB claim cluster-id "cluster-A" but keep bundleA's roots
	// under bundleB's: simulate someone copying CA material into the wrong
	// cluster. Easiest setup: reuse bundleA's roots with clusterID "B".
	rogue := &pki.TrustBundle{
		Roots:         bundleA.Roots,
		Intermediates: bundleA.Intermediates,
		ClusterID:     bundleB.ClusterID, // "cluster-B"
	}

	_, err := rogue.Verify(leafA, time.Now())
	if err == nil {
		t.Fatal("verify must reject leaf from a different cluster-id")
	}

	if !contains(err.Error(), "cluster-A") && !contains(err.Error(), "cluster-B") {
		t.Fatalf("error should mention the mismatched cluster id: %v", err)
	}
}

func TestBundle_Verify_NoURISAN(t *testing.T) {
	// Directly forge a cert lacking a URI SAN and try to verify.
	root, rootKey, _ := pki.GenerateRoot("c", 24*time.Hour)
	intCert, intKey, _ := pki.GenerateIntermediate("c", root, rootKey, 24*time.Hour)

	bundle := &pki.TrustBundle{
		Roots:         []*x509.Certificate{root},
		Intermediates: []*x509.Certificate{intCert},
		ClusterID:     "c",
	}

	// Hand-sign a cert with correct CN but no URI SAN.
	_ = intKey
	badLeaf := forgeCert(t, "ella-node-1", nil, intCert, intKey)

	if _, err := bundle.Verify(badLeaf, time.Now()); err == nil {
		t.Fatal("must reject leaf without URI SAN")
	}
}

func TestBundle_Verify_ExpiredLeaf(t *testing.T) {
	bundle, leaf := mintLeaf(t, "c", 1)

	// Verify succeeds now.
	if _, err := bundle.Verify(leaf, time.Now()); err != nil {
		t.Fatalf("fresh leaf must verify: %v", err)
	}

	// Jump past notAfter.
	future := leaf.NotAfter.Add(time.Hour)
	if _, err := bundle.Verify(leaf, future); err == nil {
		t.Fatal("expired leaf must not verify")
	}
}
