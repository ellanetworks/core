// Copyright 2026 Ella Networks

package listener

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
)

func TestParseNodeCN(t *testing.T) {
	tests := []struct {
		cn      string
		wantID  int
		wantErr bool
	}{
		{"ella-node-1", 1, false},
		{"ella-node-63", 63, false},
		{"ella-node-32", 32, false},
		{"ella-node-0", 0, true},
		{"ella-node-64", 0, true},
		{"ella-node--1", 0, true},
		{"ella-node-abc", 0, true},
		{"ella-node-", 0, true},
		{"wrong-prefix-1", 0, true},
		{"", 0, true},
		{"ella-node-1extra", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.cn, func(t *testing.T) {
			id, err := parseNodeCN(tc.cn)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for CN %q, got id=%d", tc.cn, id)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error for CN %q: %v", tc.cn, err)
			}

			if id != tc.wantID {
				t.Fatalf("CN %q: expected id=%d, got %d", tc.cn, tc.wantID, id)
			}
		})
	}
}

func TestVerifyConnection_ValidChain(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1})
	verify := verifyConnection(pki.CAPool)

	node := pki.Nodes[1]

	certs := parseCerts(t, node.TLSCert.Certificate)

	if err := verify(tls.ConnectionState{PeerCertificates: certs}); err != nil {
		t.Fatalf("valid cert should pass: %v", err)
	}
}

func TestVerifyConnection_Rejections(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	// Generate a second, independent CA for "wrong CA" tests.
	wrongPKI := testutil.GenTestPKI(t, []int{1})

	tests := []struct {
		name    string
		certsFn func(t *testing.T) []*x509.Certificate
		caPool  *x509.CertPool
		wantErr string
	}{
		{
			name: "no certificates",
			certsFn: func(_ *testing.T) []*x509.Certificate {
				return nil
			},
			caPool:  pki.CAPool,
			wantErr: "no certificates",
		},
		{
			name: "wrong CA",
			certsFn: func(t *testing.T) []*x509.Certificate {
				t.Helper()

				node := wrongPKI.Nodes[1]

				return parseCerts(t, node.TLSCert.Certificate)
			},
			caPool:  pki.CAPool,
			wantErr: "chain verification failed",
		},
		{
			name: "expired leaf",
			certsFn: func(t *testing.T) []*x509.Certificate {
				t.Helper()

				expired := pki.GenExpiredLeaf(t, 1)

				return parseCerts(t, expired.TLSCert.Certificate)
			},
			caPool:  pki.CAPool,
			wantErr: "chain verification failed",
		},
		{
			name: "CN malformed - no prefix",
			certsFn: func(t *testing.T) []*x509.Certificate {
				t.Helper()

				bad := pki.GenLeafWithCN(t, "not-ella-node")

				return parseCerts(t, bad.TLSCert.Certificate)
			},
			caPool:  pki.CAPool,
			wantErr: "does not start with",
		},
		{
			name: "CN out of range - zero",
			certsFn: func(t *testing.T) []*x509.Certificate {
				t.Helper()

				bad := pki.GenLeafWithCN(t, "ella-node-0")

				return parseCerts(t, bad.TLSCert.Certificate)
			},
			caPool:  pki.CAPool,
			wantErr: "outside valid range",
		},
		{
			name: "CN out of range - 64",
			certsFn: func(t *testing.T) []*x509.Certificate {
				t.Helper()

				bad := pki.GenLeafWithCN(t, "ella-node-64")

				return parseCerts(t, bad.TLSCert.Certificate)
			},
			caPool:  pki.CAPool,
			wantErr: "outside valid range",
		},
		{
			name: "CN non-integer suffix",
			certsFn: func(t *testing.T) []*x509.Certificate {
				t.Helper()

				bad := pki.GenLeafWithCN(t, "ella-node-abc")

				return parseCerts(t, bad.TLSCert.Certificate)
			},
			caPool:  pki.CAPool,
			wantErr: "non-integer node-id suffix",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verify := verifyConnection(tc.caPool)
			certs := tc.certsFn(t)

			err := verify(tls.ConnectionState{PeerCertificates: certs})
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !containsStr(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestVerifyConnection_SelfSignedRejected(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1})
	verify := verifyConnection(pki.CAPool)

	// Create a self-signed cert (not signed by the cluster CA).
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(999),
		Subject:      pkix.Name{CommonName: "ella-node-1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	selfSignedCert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	err = verify(tls.ConnectionState{PeerCertificates: []*x509.Certificate{selfSignedCert}})
	if err == nil {
		t.Fatal("self-signed cert should be rejected")
	}

	if !containsStr(err.Error(), "chain verification failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

func parseCerts(t *testing.T, rawCerts [][]byte) []*x509.Certificate {
	t.Helper()

	certs := make([]*x509.Certificate, 0, len(rawCerts))

	for _, raw := range rawCerts {
		cert, err := x509.ParseCertificate(raw)
		if err != nil {
			t.Fatalf("parseCerts: %v", err)
		}

		certs = append(certs, cert)
	}

	return certs
}
