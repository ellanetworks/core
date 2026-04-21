// Copyright 2026 Ella Networks

package pki_test

import (
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/pki"
)

func TestGenerateKeyAndCSR_Shape(t *testing.T) {
	priv, csrPEM, err := pki.GenerateKeyAndCSR(7, "my-cluster")
	if err != nil {
		t.Fatal(err)
	}

	if len(priv) == 0 || len(csrPEM) == 0 {
		t.Fatal("empty outputs")
	}

	csr, err := pki.ParseCSRPEM(csrPEM)
	if err != nil {
		t.Fatal(err)
	}

	if csr.Subject.CommonName != "ella-node-7" {
		t.Fatalf("CN = %q", csr.Subject.CommonName)
	}

	if len(csr.URIs) != 1 {
		t.Fatalf("URIs = %v", csr.URIs)
	}

	want := "spiffe://cluster.ella/my-cluster/node/7"
	if csr.URIs[0].String() != want {
		t.Fatalf("URI = %q, want %q", csr.URIs[0], want)
	}

	if len(csr.DNSNames) > 0 || len(csr.IPAddresses) > 0 || len(csr.EmailAddresses) > 0 {
		t.Fatal("unexpected non-URI SANs")
	}
}

func TestGenerateKeyAndCSR_Rejects(t *testing.T) {
	cases := []struct {
		nodeID    int
		clusterID string
	}{
		{0, "c"},
		{64, "c"},
		{-1, "c"},
		{1, ""},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d/%q", tc.nodeID, tc.clusterID), func(t *testing.T) {
			_, _, err := pki.GenerateKeyAndCSR(tc.nodeID, tc.clusterID)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseCSRPEM_BadInput(t *testing.T) {
	_, err := pki.ParseCSRPEM([]byte("not pem"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateLeafCSR_Happy(t *testing.T) {
	_, csrPEM, err := pki.GenerateKeyAndCSR(3, "c")
	if err != nil {
		t.Fatal(err)
	}

	csr, err := pki.ParseCSRPEM(csrPEM)
	if err != nil {
		t.Fatal(err)
	}

	if err := pki.ValidateLeafCSR(csr, 3, "c"); err != nil {
		t.Fatalf("valid csr rejected: %v", err)
	}
}

func TestValidateLeafCSR_WrongClusterID(t *testing.T) {
	_, csrPEM, _ := pki.GenerateKeyAndCSR(3, "c-A")

	csr, _ := pki.ParseCSRPEM(csrPEM)

	if err := pki.ValidateLeafCSR(csr, 3, "c-B"); err == nil {
		t.Fatal("cluster-id mismatch must reject")
	}
}
