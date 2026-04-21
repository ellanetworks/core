// Copyright 2026 Ella Networks

package listener_test

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
)

// parsedCert parses the first DER of a tls.Certificate into *x509.Certificate.
func parsedCert(t *testing.T, c tls.Certificate) *x509.Certificate {
	t.Helper()

	if len(c.Certificate) == 0 {
		t.Fatal("empty tls.Certificate")
	}

	p, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}

	return p
}
