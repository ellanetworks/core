// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-jose/go-jose/v4"
)

func TestTokenEndpointAuthMethod(t *testing.T) {
	for name, tc := range map[string]struct {
		supported []string
		want      string
	}{
		"omitted":                 {nil, ClientSecretBasic},
		"basic":                   {[]string{"private_key_jwt", ClientSecretBasic, ClientSecretPost}, ClientSecretBasic},
		"post":                    {[]string{"private_key_jwt", ClientSecretPost}, ClientSecretPost},
		"neither basic nor post":  {[]string{"private_key_jwt", "client_secret_jwt"}, ""},
		"tls_client_auth only":    {[]string{"tls_client_auth"}, ""},
		"basic in another case":   {[]string{"Client_Secret_Basic"}, ""},
		"post with an empty item": {[]string{"", ClientSecretPost}, ClientSecretPost},
	} {
		got, err := tokenEndpointAuthMethod(tc.supported)
		if got != tc.want || (err == nil) != (tc.want != "") {
			t.Errorf("%s: %q, %v, want %q", name, got, err, tc.want)
		}
	}
}

func testServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, *http.Client) {
	t.Helper()

	s := httptest.NewTLSServer(h)
	t.Cleanup(s.Close)

	client, err := HTTPClient(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.Certificate().Raw})))
	if err != nil {
		t.Fatal(err)
	}

	return s, client
}

func get(t *testing.T, client *http.Client, target string) ([]byte, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	return io.ReadAll(resp.Body)
}

func TestHTTPClientCapsResponses(t *testing.T) {
	var written atomic.Int64

	s, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		chunk := []byte(strings.Repeat(" ", 64<<10))

		if r.URL.Path == "/declared" {
			w.Header().Set("Content-Length", "1073741824")
		}

		for range 1 << 14 {
			n, err := w.Write(chunk)
			written.Add(int64(n))

			if err != nil {
				return
			}
		}
	})

	for _, path := range []string{"/streamed", "/declared"} {
		if _, err := get(t, client, s.URL+path); !errors.Is(err, errTooLarge) {
			t.Errorf("%s: %v, want errTooLarge", path, err)
		}
	}

	if n := written.Load(); n > 64<<20 {
		t.Fatalf("the server wrote %d bytes", n)
	}

	small, client := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", MaxResponseSize)))
	})

	if b, err := get(t, client, small.URL); err != nil || len(b) != MaxResponseSize {
		t.Fatalf("a response of %d bytes: read %d, %v", MaxResponseSize, len(b), err)
	}
}

func TestHTTPClientRedirects(t *testing.T) {
	var target string

	s, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/to-http":
			http.Redirect(w, r, "http://127.0.0.1:1/secret", http.StatusFound)
		case "/to-https":
			http.Redirect(w, r, target+"/ok", http.StatusFound)
		case "/ok":
			_, _ = w.Write([]byte("ok"))
		}
	})
	target = s.URL

	if _, err := get(t, client, s.URL+"/to-http"); !errors.Is(err, errInsecure) {
		t.Errorf("redirect to http: %v, want errInsecure", err)
	}

	if b, err := get(t, client, s.URL+"/to-https"); err != nil || string(b) != "ok" {
		t.Fatalf("redirect to https: %q, %v", b, err)
	}

	if _, err := get(t, withoutRedirects(client), s.URL+"/to-https"); !errors.Is(err, errNoRedirect) {
		t.Errorf("redirect without redirects: %v, want errNoRedirect", err)
	}
}

func TestCountSigningKeysSkipsUnsupportedKeys(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	good, err := json.Marshal(jose.JSONWebKey{Key: key.Public(), KeyID: "sig", Algorithm: "ES256", Use: "sig"})
	if err != nil {
		t.Fatal(err)
	}

	keys := map[string]string{
		"/mixed":       `{"keys": [{"kty": "unknown", "kid": "x"}, {"kty": "EC", "crv": "P-999", "x": "AA", "y": "AA"}, ` + string(good) + `]}`,
		"/unsupported": `{"keys": [{"kty": "unknown", "kid": "x"}]}`,
		"/encryption":  `{"keys": [` + strings.Replace(string(good), `"use":"sig"`, `"use":"enc"`, 1) + `]}`,
	}

	s, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(keys[r.URL.Path]))
	})

	if n, err := countSigningKeys(context.Background(), client, s.URL+"/mixed"); err != nil || n != 1 {
		t.Errorf("a set with unsupported keys: %d, %v, want 1", n, err)
	}

	for _, path := range []string{"/unsupported", "/encryption"} {
		if n, err := countSigningKeys(context.Background(), client, s.URL+path); err == nil {
			t.Errorf("%s: %d keys, want an error", path, n)
		}
	}
}
