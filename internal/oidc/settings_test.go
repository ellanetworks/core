// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func testCA(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test CA"}, IsCA: true,
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func valid() db.OIDC {
	return db.OIDC{
		Enabled:         true,
		ExternalURL:     "https://core.example.com",
		Issuer:          "https://idp.example.com/realms/ella",
		ClientID:        "ella-core",
		ClientSecret:    "secret",
		DisplayName:     "Keycloak",
		Scopes:          []string{"openid", "email"},
		SessionLifetime: 24 * time.Hour,
		GroupsClaim:     "groups",
		GroupRoles:      []db.GroupRole{{Group: "/core/admins", RoleID: db.RoleAdmin}},
	}
}

func TestValidate(t *testing.T) {
	ca := testCA(t)

	accepted := map[string]func(*db.OIDC){
		"the defaults":              func(o *db.OIDC) { *o = db.OIDC{Scopes: []string{"openid", "email"}, SessionLifetime: 24 * time.Hour} },
		"an external URL with a /":  func(o *db.OIDC) { o.ExternalURL += "/" },
		"an external URL with port": func(o *db.OIDC) { o.ExternalURL = "https://10.0.0.1:5010" },
		"an issuer with a /":        func(o *db.OIDC) { o.Issuer += "/" },
		"a CA":                      func(o *db.OIDC) { o.CACertificate = ca },
		"two CAs":                   func(o *db.OIDC) { o.CACertificate = ca + "\n" + ca },
		"an indented CA": func(o *db.OIDC) {
			o.CACertificate = "    " + strings.ReplaceAll(strings.TrimSpace(ca), "\n", "\n    ")
		},
		"a CA with CRLF":           func(o *db.OIDC) { o.CACertificate = strings.ReplaceAll(ca, "\n", "\r\n") },
		"no groups":                func(o *db.OIDC) { o.GroupsClaim, o.GroupRoles = "", nil },
		"a groups claim alone":     func(o *db.OIDC) { o.GroupRoles = nil },
		"a scope with punctuation": func(o *db.OIDC) { o.Scopes = append(o.Scopes, "api://core/.default") },
		"disabled with an issuer":  func(o *db.OIDC) { o.Enabled, o.ClientID, o.ClientSecret = false, "", "" },
	}

	for name, change := range accepted {
		o := valid()
		change(&o)

		if err := Validate(o); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	refused := map[string]func(*db.OIDC){
		"disabled with an http issuer":  func(o *db.OIDC) { o.Enabled, o.Issuer = false, "http://idp.example.com" },
		"an issuer with user info":      func(o *db.OIDC) { o.Issuer = "https://u:p@idp.example.com" },
		"an issuer with an empty query": func(o *db.OIDC) { o.Issuer = "https://idp.example.com?" },
		"an issuer that is not a URL":   func(o *db.OIDC) { o.Issuer = "idp.example.com" },
		"an external URL with a query":  func(o *db.OIDC) { o.ExternalURL = "https://core.example.com/?a=b" },
		"no client secret":              func(o *db.OIDC) { o.ClientSecret = "" },
		"a long display name":           func(o *db.OIDC) { o.DisplayName = strings.Repeat("x", 65) },
		"no email scope":                func(o *db.OIDC) { o.Scopes = []string{"openid", "profile"} },
		"no openid scope":               func(o *db.OIDC) { o.Scopes = []string{"email", "profile"} },
		"an external URL with //":       func(o *db.OIDC) { o.ExternalURL = "https://core.example.com//" },
		"an external URL with %2F":      func(o *db.OIDC) { o.ExternalURL = "https://core.example.com/%2F" },
		"a scope with a quote":          func(o *db.OIDC) { o.Scopes = append(o.Scopes, `a"b`) },
		"a scope with a backslash":      func(o *db.OIDC) { o.Scopes = append(o.Scopes, `a\b`) },
		"a scope that is not ASCII":     func(o *db.OIDC) { o.Scopes = append(o.Scopes, "é") },
		"too many scopes": func(o *db.OIDC) {
			for i := len(o.Scopes); i <= MaxScopes; i++ {
				o.Scopes = append(o.Scopes, fmt.Sprintf("s%d", i))
			}
		},
		"a lifetime under five minutes": func(o *db.OIDC) { o.SessionLifetime = 5*time.Minute - time.Second },
		"a lifetime over thirty days":   func(o *db.OIDC) { o.SessionLifetime = 30*24*time.Hour + time.Second },
		"a CA with trailing text":       func(o *db.OIDC) { o.CACertificate = ca + "junk" },
		"a private key as CA":           func(o *db.OIDC) { o.CACertificate = "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n" },
		"a group with a control char":   func(o *db.OIDC) { o.GroupRoles[0].Group = "a\x00b" },
		"a group with no role":          func(o *db.OIDC) { o.GroupRoles[0].RoleID = 0 },
	}

	for name, change := range refused {
		o := valid()
		change(&o)

		if err := Validate(o); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

type fakeStore struct {
	saved db.OIDC
	err   error
}

func (s *fakeStore) GetOIDC(context.Context) (db.OIDC, error) { return s.saved, nil }

func (s *fakeStore) UpdateOIDC(_ context.Context, o db.OIDC) error {
	if s.err != nil {
		return s.err
	}

	s.saved = o

	return nil
}

func TestLive(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	l := NewLive(store, db.OIDC{Scopes: []string{"openid"}, SessionLifetime: time.Hour})

	got := l.Get()
	got.Scopes[0] = "changed"

	if l.Get().Scopes[0] != "openid" {
		t.Fatal("Get shares its scopes with the caller")
	}

	o := valid()
	if err := l.Update(ctx, o); err != nil {
		t.Fatalf("Update: %v", err)
	}

	o.GroupRoles[0].Group = "changed"

	if l.Get().GroupRoles[0].Group != "/core/admins" || store.saved.Issuer != o.Issuer {
		t.Fatalf("after Update: %+v, stored %+v", l.Get(), store.saved)
	}

	bad := valid()
	bad.Issuer = "http://idp.example.com"

	if err := l.Update(ctx, bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Update with an http issuer: %v, want ErrInvalid", err)
	}

	for in, want := range map[string]string{
		"https://core.example.com/":      "https://core.example.com",
		"https://Core.Example.com:8443/": "https://core.example.com:8443",
		"https://[2001:db8::1]:8443":     "https://[2001:db8::1]:8443",
		"https://core.example.com":       "https://core.example.com",
	} {
		o := valid()
		o.ExternalURL = in

		if err := l.Update(ctx, o); err != nil || l.Get().ExternalURL != want || store.saved.ExternalURL != want {
			t.Errorf("Update with external URL %q: %v, stored %q, want %q", in, err, store.saved.ExternalURL, want)
		}
	}

	store.err = errors.New("disk full")

	if err := l.Update(ctx, db.OIDC{Scopes: []string{"openid", "email"}, SessionLifetime: time.Hour}); err == nil {
		t.Fatal("Update with a failing store: no error")
	}

	if !l.Get().Enabled {
		t.Fatal("a failed Update changed the settings")
	}
}
