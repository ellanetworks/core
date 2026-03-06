// Copyright 2026 Ella Networks

package client

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"time"
)

type Fleet struct {
	url    string
	client *http.Client
}

// New creates a fleet client that skips TLS verification. This is used for
// the initial registration call before we have a CA certificate.
func New(url string) *Fleet {
	return &Fleet{
		url: url,
		client: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}
}

// ConfigureMTLS replaces the HTTP transport with one that presents the given
// client certificate and verifies the server against the provided CA.
func (fc *Fleet) ConfigureMTLS(certPEM string, key *ecdsa.PrivateKey, caCertPEM string) error {
	tlsCert, err := tls.X509KeyPair([]byte(certPEM), pemEncodeECKey(key))
	if err != nil {
		return fmt.Errorf("loading client key pair: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM([]byte(caCertPEM)); !ok {
		return fmt.Errorf("failed to parse CA certificate")
	}

	fc.client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{tlsCert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: true,
		},
	}

	return nil
}

// checkResponseContentType reads the Content-Type header and returns a
// user-friendly error when the fleet server replies with something other
// than JSON (e.g. an HTML page), which usually means the URL is wrong.
func checkResponseContentType(res *http.Response) error {
	ct := res.Header.Get("Content-Type")
	if ct != "" && !isJSONContentType(ct) {
		return fmt.Errorf("fleet server at %s returned an unexpected response (Content-Type: %s) — verify the Fleet URL is correct", res.Request.URL.Host, ct)
	}

	return nil
}

func isJSONContentType(ct string) bool {
	for _, prefix := range []string{"application/json", "text/json"} {
		if len(ct) >= len(prefix) && ct[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

// pemEncodeECKey returns the PEM-encoded EC private key bytes.
func pemEncodeECKey(key *ecdsa.PrivateKey) []byte {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil
	}

	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
}
