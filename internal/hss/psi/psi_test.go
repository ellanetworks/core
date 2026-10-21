// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package psi_test

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/hss/psi"
)

func TestCanonical(t *testing.T) {
	for in, want := range map[string]string{
		"sip:svc@Example.COM":                   "sip:svc@example.com",
		"SIP:Svc@example.com":                   "sip:Svc@example.com",
		"sips:example.com;transport=TCP":        "sips:example.com;transport=TCP",
		"sip:conf@Example.com:5060;lr":          "sip:conf@example.com:5060;lr",
		"tel:+1-555-0100":                       "tel:+15550100",
		"tel:7042;phone-context=Example.com":    "tel:7042;phone-context=Example.com",
		"sip:!.*!@MCPTT.ims.example.org":        "sip:!.*!@mcptt.ims.example.org",
		"sip:chatroom-!.*!@Example.com":         "sip:chatroom-!.*!@example.com",
		"sip:!a@b!@example.com":                 "sip:!a@b!@example.com",
		"tel:+1-212-555-!.*!":                   "tel:+1212555!.*!",
		"tel:+1-212-555-!.-!;phone-context=x.y": "tel:+1212555!.-!;phone-context=x.y",
	} {
		got, err := psi.Canonical(in)
		if err != nil || got != want {
			t.Errorf("Canonical(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestCanonicalRejects(t *testing.T) {
	for in, want := range map[string]error{
		"":                       psi.ErrInvalidURI,
		"example.com":            psi.ErrInvalidURI,
		"http://example.com":     psi.ErrInvalidURI,
		" sip:a@example.com":     psi.ErrInvalidURI,
		"sip:":                   psi.ErrInvalidURI,
		"sip:a@":                 psi.ErrInvalidURI,
		"tel:":                   psi.ErrInvalidURI,
		"tel:-":                  psi.ErrInvalidURI,
		"sip:!.*@example.com":    psi.ErrInvalidWildcard,
		"sip:a@!.*!.example.com": psi.ErrInvalidWildcard,
		"sip:!.*!":               psi.ErrInvalidWildcard,
		"sip:!([!@example.com":   psi.ErrInvalidWildcard,
		"tel:!.*!":               psi.ErrInvalidWildcard,
		"tel:+!.*!":              psi.ErrInvalidWildcard,
	} {
		if _, err := psi.Canonical(in); !errors.Is(err, want) {
			t.Errorf("Canonical(%q): err = %v, want %v", in, err, want)
		}
	}
}

func TestCanonicalServerName(t *testing.T) {
	if got, err := psi.CanonicalServerName("sip:MCPTT.Example.org;lr"); err != nil || got != "sip:mcptt.example.org;lr" {
		t.Fatalf("CanonicalServerName = %q, %v", got, err)
	}

	for _, in := range []string{"tel:+15550100", "sip:!.*!@example.org", "sip:as.example.org "} {
		if _, err := psi.CanonicalServerName(in); !errors.Is(err, psi.ErrInvalidURI) {
			t.Errorf("CanonicalServerName(%q): err = %v, want ErrInvalidURI", in, err)
		}
	}
}

func TestOverlap(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"sip:!.*!@mcptt.example.org", "sip:conf!.*!@mcptt.example.org", true},
		{"sip:!.*!@mcptt.example.org", "sip:!.*!@other.example.org", false},
		{"sip:!.*!@mcptt.example.org", "sip:conf@mcptt.example.org", false},
		{"tel:+1555!.*!", "tel:+1666!.*!", true},
		{"tel:+1555!.*!", "sip:!.*!@example.org", false},
	} {
		if got := psi.Overlap(tc.a, tc.b); got != tc.want {
			t.Errorf("Overlap(%q, %q) = %t, want %t", tc.a, tc.b, got, tc.want)
		}
	}
}
