// Copyright 2026 Ella Networks

package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Fleet is the Core-side client for the Fleet management API. After a
// successful Register call the caller stores the returned token via
// SetToken; every subsequent request carries it in the Authorization header.
type Fleet struct {
	url    string
	client *http.Client
	token  string
}

// New constructs a Fleet client.
//
// When insecureSkipVerify is false (the default in production
// deployments), TLS server certificates are validated against the system
// trust store. Self-hosted Fleet deployments using self-signed certs
// either install the cert into the Core host's trust store, front Fleet
// with a real cert (Let's Encrypt or internal PKI), or set
// `fleet.insecure-skip-verify: true` in core.yaml to disable validation
// — appropriate for integration tests and local-development setups, but
// not for production.
func New(url string, insecureSkipVerify bool) *Fleet {
	transport := http.DefaultTransport

	if insecureSkipVerify {
		transport = &http.Transport{
			// #nosec G402 -- explicitly gated behind fleet.insecure-skip-verify in core.yaml
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return &Fleet{
		url: url,
		client: &http.Client{
			Timeout:   15 * time.Second,
			Transport: transport,
		},
	}
}

// SetToken sets the bearer token used to authenticate sync and unregister
// requests. Empty token disables the Authorization header (only valid before
// registration completes).
func (fc *Fleet) SetToken(token string) {
	fc.token = token
}

// addAuth attaches the bearer token to the request when set. Callers use
// this for every request that requires authentication (sync, unregister).
func (fc *Fleet) addAuth(req *http.Request) {
	if fc.token != "" {
		req.Header.Set("Authorization", "Bearer "+fc.token)
	}
}

// checkResponseContentType returns a user-friendly error when the fleet
// server replies with something other than JSON (usually HTML from a
// mis-configured URL).
func checkResponseContentType(res *http.Response) error {
	ct := res.Header.Get("Content-Type")
	if ct != "" && !isJSONContentType(ct) {
		return fmt.Errorf("fleet server at %s returned an unexpected response (Content-Type: %s) — verify the Fleet URL is correct", res.Request.URL.Host, ct)
	}

	return nil
}

func isJSONContentType(ct string) bool {
	for _, prefix := range []string{"application/json", "text/json"} {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}

	return false
}
