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
		"sips:example.com;transport=TCP":        "sips:example.com",
		"sip:conf@Example.com:5060;lr":          "sip:conf@example.com:5060",
		"tel:+1-555-0100":                       "tel:+15550100",
		"tel:7042;phone-context=Example.com":    "tel:7042;phone-context=example.com",
		"tel:+15550100;ext=12":                  "tel:+15550100",
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
		{"sip:conf!.*!@ims.example.org", "sip:chat!.*!@ims.example.org", false},
		{"sip:conf!.*!@ims.example.org", "sip:con!.*!@ims.example.org", true},
		{"sip:conf!-[0-9]+!@ims.example.org", "sip:conf!-[a-z]+!@ims.example.org", false},
		{"sip:conf!-[0-9]+!@ims.example.org", "sip:conf!-[0-9a-f]+!@ims.example.org", true},
		{"sip:!(a|b)!x@ims.example.org", "sip:!b!x@ims.example.org", true},
		{"sip:group!.*!@ims.example.org", "sip:!.*!-1@ims.example.org", true},
		{"tel:+1555!.*!", "tel:+1666!.*!", false},
		{"tel:+1555!.*!", "tel:+155!.*!", true},
		{"tel:+1555!.*!", "sip:!.*!@example.org", false},
	} {
		if got := psi.Overlap(tc.a, tc.b); got != tc.want {
			t.Errorf("Overlap(%q, %q) = %t, want %t", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCovers(t *testing.T) {
	for _, tc := range []struct {
		stored, pattern string
		want            bool
	}{
		{"sip:!9.*!@ims.example.org", `^sip:[0-9]{6,15}@ims\.example\.org$`, true},
		{"sip:conf!.*!@ims.example.org", `^sip:[0-9]{6,15}@ims\.example\.org$`, false},
		{"sip:!.*!@mcptt.example.org", `^sip:[0-9]{6,15}@ims\.example\.org$`, false},
		{"sip:conf@ims.example.org", `^sip:conf@ims\.example\.org$`, false},
	} {
		if got := psi.Covers(tc.stored, tc.pattern); got != tc.want {
			t.Errorf("Covers(%q, %q) = %t, want %t", tc.stored, tc.pattern, got, tc.want)
		}
	}
}

func TestLiteralTreatsExclamationMarksAsCharacters(t *testing.T) {
	for in, want := range map[string]string{
		"sip:chatlist!1@Example.com":       "sip:chatlist!1@example.com",
		"sip:chatlist1@example.com;user=x": "sip:chatlist1@example.com",
		"tel:+1-555-0100;isub=1":           "tel:+15550100",
	} {
		got, err := psi.Literal(in)
		if err != nil || got != want {
			t.Errorf("Literal(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestMatcher(t *testing.T) {
	m := psi.NewMatcher(map[string]string{
		"sip:chatlist!.*!@example.com":  "chat",
		"sip:chatlist1@example.com":     "distinct",
		"sip:!.*!@mcptt.example.org":    "mcptt",
		"tel:+1555!.*!":                 "numbers",
		"tel:+15550100":                 "operator",
		"sip:!([!@broken.example.org":   "broken",
		"sip:conf-!(a|b)[0-9]+!@ex.org": "conf",
	})

	for identity, want := range map[string]psi.Match{
		"sip:chatlist1@example.com":            {Owner: "distinct"},
		"sip:chatlist2@example.com":            {Owner: "chat", Wildcard: "sip:chatlist!.*!@example.com"},
		"sip:chatlist42@Example.COM":           {Owner: "chat", Wildcard: "sip:chatlist!.*!@example.com"},
		"sip:chatlistAbC@example.com":          {Owner: "chat", Wildcard: "sip:chatlist!.*!@example.com"},
		"sip:chatlist!1@example.com":           {Owner: "chat", Wildcard: "sip:chatlist!.*!@example.com"},
		"sip:group-7@mcptt.example.org;user=x": {Owner: "mcptt", Wildcard: "sip:!.*!@mcptt.example.org"},
		"tel:+1-555-0100":                      {Owner: "operator"},
		"tel:+15559999":                        {Owner: "numbers", Wildcard: "tel:+1555!.*!"},
		"sip:conf-a12@ex.org":                  {Owner: "conf", Wildcard: "sip:conf-!(a|b)[0-9]+!@ex.org"},
	} {
		got, ok := m.Match(identity)
		if !ok || got != want {
			t.Errorf("Match(%q) = %+v, %t; want %+v", identity, got, ok, want)
		}
	}

	for _, identity := range []string{"sip:chatlist1@example.org", "sip:mcptt.example.org", "tel:+1666", "sip:conf-c12@ex.org", "sip:xconf-a1@ex.org", "not a uri"} {
		if got, ok := m.Match(identity); ok {
			t.Errorf("Match(%q) = %+v, want no match", identity, got)
		}
	}
}

func FuzzWildcard(f *testing.F) {
	for _, seed := range []string{"sip:chatlist!.*!@example.com", "sip:!([!@example.com", "tel:+1555!.*!", "sip:!a!b!@x.org", "sip:!!@x.org"} {
		f.Add(seed, "sip:chatlist1@example.com")
	}

	f.Fuzz(func(t *testing.T, stored, identity string) {
		canonical, err := psi.Canonical(stored)
		if err != nil {
			return
		}

		again, err := psi.Canonical(canonical)
		if err != nil || again != canonical {
			t.Fatalf("Canonical(%q) = %q is not stable: %q, %v", stored, canonical, again, err)
		}

		psi.NewMatcher(map[string]string{canonical: "owner"}).Match(identity)
	})
}
