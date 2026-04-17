//go:build ignore

// gen.go generates the static test PKI used by the HA integration tests.
// Run once with:  go run gen.go
// The output PEM files are checked into the repository.

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

func main() {
	notBefore := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Date(2036, 1, 1, 0, 0, 0, 0, time.UTC)

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ella-test-ca"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		panic(err)
	}

	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		panic(err)
	}

	writePEM("ca.pem", "CERTIFICATE", caDER)

	for _, nodeID := range []int{1, 2, 3, 4} {
		leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			panic(err)
		}

		cn := fmt.Sprintf("ella-node-%d", nodeID)

		leafTemplate := &x509.Certificate{
			SerialNumber: big.NewInt(int64(nodeID + 1)),
			Subject:      pkix.Name{CommonName: cn},
			DNSNames:     []string{cn},
			NotBefore:    notBefore,
			NotAfter:     notAfter,
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		}

		leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
		if err != nil {
			panic(err)
		}

		keyDER, err := x509.MarshalECPrivateKey(leafKey)
		if err != nil {
			panic(err)
		}

		writePEM(fmt.Sprintf("node%d-cert.pem", nodeID), "CERTIFICATE", leafDER)
		writePEM(fmt.Sprintf("node%d-key.pem", nodeID), "EC PRIVATE KEY", keyDER)
	}

	fmt.Println("PKI generated: ca.pem, node{1..4}-{cert,key}.pem")
}

func writePEM(filename, blockType string, data []byte) {
	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}

	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: data}); err != nil {
		_ = f.Close()
		panic(err)
	}

	if err := f.Close(); err != nil {
		panic(err)
	}
}
