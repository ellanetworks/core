//go:build ignore

// gen.go regenerates the per-node self-signed cluster certs used by
// the HA integration tests.
//
// Run once with:  go run gen.go
// The output PEM files are checked into the repository.
//
// Post-v12: there is no CA. Each node owns a self-signed cluster
// cert; trust is established at runtime via cluster_node_certs pins.
// The integration harness either pre-seeds the pins in each node's
// DB or relies on the join-token flow to register them.

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"time"
)

func main() {
	notBefore := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Date(2036, 1, 1, 0, 0, 0, 0, time.UTC)

	clusterID := "ella-test-cluster"

	for _, nodeID := range []int{1, 2, 3, 4} {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			panic(err)
		}

		cn := fmt.Sprintf("ella-node-%d", nodeID)

		uri := &url.URL{
			Scheme: "spiffe",
			Host:   "cluster.ella",
			Path:   fmt.Sprintf("/%s/node/%d", clusterID, nodeID),
		}

		template := &x509.Certificate{
			SerialNumber:          big.NewInt(int64(nodeID + 1)),
			Subject:               pkix.Name{CommonName: cn},
			URIs:                  []*url.URL{uri},
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
		}

		der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		if err != nil {
			panic(err)
		}

		keyDER, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			panic(err)
		}

		writePEM(fmt.Sprintf("node%d-cert.pem", nodeID), "CERTIFICATE", der)
		writePEM(fmt.Sprintf("node%d-key.pem", nodeID), "EC PRIVATE KEY", keyDER)

		sum := sha256.Sum256(der)

		fmt.Printf("node %d: sha256:%s\n", nodeID, hex.EncodeToString(sum[:]))
	}

	fmt.Println("self-signed cluster certs regenerated")
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
