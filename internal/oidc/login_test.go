// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestLoginCookie(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	c := &Client{settings: valid()}

	l := c.NewLogin(now)
	if l.State == "" || l.Nonce == "" || len(l.Verifier) < 43 || !c.Owns(l) {
		t.Fatalf("NewLogin = %+v", l)
	}

	sealed, err := Seal(key, "cookie", l)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains([]byte(sealed), []byte(l.State)) {
		t.Fatal("the sealed login shows its state")
	}

	if got, err := Open(key, "cookie", sealed, now.Add(LoginLifetime-time.Second)); err != nil || got != l {
		t.Fatalf("Open = %+v, %v, want %+v", got, err, l)
	}

	other, err := Seal(key, "cookie", l)
	if err != nil || other == sealed {
		t.Fatalf("sealing twice gave the same value: %v", err)
	}

	refused := map[string]func() (Login, error){
		"expired":      func() (Login, error) { return Open(key, "cookie", sealed, now.Add(LoginLifetime)) },
		"another name": func() (Login, error) { return Open(key, "other", sealed, now) },
		"another key":  func() (Login, error) { return Open(bytes.Repeat([]byte{8}, 32), "cookie", sealed, now) },
		"tampered":     func() (Login, error) { return Open(key, "cookie", sealed[:len(sealed)-2]+"AA", now) },
		"not base64":   func() (Login, error) { return Open(key, "cookie", "!!!", now) },
		"too short":    func() (Login, error) { return Open(key, "cookie", "AAAA", now) },
		"empty":        func() (Login, error) { return Open(key, "cookie", "", now) },
	}

	for name, open := range refused {
		if _, err := open(); !errors.Is(err, ErrLogin) {
			t.Errorf("%s: %v, want ErrLogin", name, err)
		}
	}

	moved := &Client{settings: valid()}
	moved.settings.Issuer = "https://other.example.com"

	if moved.Owns(l) {
		t.Fatal("a login of another issuer is owned")
	}
}

func TestCheckIssuer(t *testing.T) {
	for name, tc := range map[string]struct {
		advertised, present bool
		iss                 string
		ok                  bool
	}{
		"advertised and matching":       {true, true, "https://idp.example.com/realms/ella", true},
		"advertised and missing":        {true, false, "", false},
		"advertised and wrong":          {true, true, "https://evil.example.com", false},
		"not advertised and missing":    {false, false, "", true},
		"not advertised but matching":   {false, true, "https://idp.example.com/realms/ella", true},
		"not advertised and wrong":      {false, true, "https://evil.example.com", false},
		"advertised with a slash added": {true, true, "https://idp.example.com/realms/ella/", false},
	} {
		c := &Client{settings: valid(), discovery: Discovery{IssuerParameter: tc.advertised}}
		if err := c.CheckIssuer(tc.iss, tc.present); (err == nil) != tc.ok {
			t.Errorf("%s: %v", name, err)
		}
	}
}
