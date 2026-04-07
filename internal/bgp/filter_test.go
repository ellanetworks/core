package bgp

import (
	"net/netip"
	"testing"
)

func mustParsePrefix(s string) netip.Prefix {
	p, err := netip.ParsePrefix(s)
	if err != nil {
		panic(err)
	}

	return p
}

func TestMatchesPrefixList_DefaultRouteOnly(t *testing.T) {
	entries := []ImportPrefix{
		{Prefix: mustParsePrefix("0.0.0.0/0"), MaxLength: 0},
	}

	// Exact match: default route
	if !matchesPrefixList(mustParsePrefix("0.0.0.0/0"), entries) {
		t.Fatal("expected default route to match")
	}

	// More specific route should not match (maxLength=0 means only /0)
	if matchesPrefixList(mustParsePrefix("10.0.0.0/8"), entries) {
		t.Fatal("expected /8 to not match default-route-only entry")
	}
}

func TestMatchesPrefixList_AcceptAll(t *testing.T) {
	entries := []ImportPrefix{
		{Prefix: mustParsePrefix("0.0.0.0/0"), MaxLength: 32},
	}

	testCases := []struct {
		prefix string
		match  bool
	}{
		{"0.0.0.0/0", true},
		{"10.0.0.0/8", true},
		{"192.168.1.0/24", true},
		{"10.1.2.3/32", true},
	}

	for _, tc := range testCases {
		got := matchesPrefixList(mustParsePrefix(tc.prefix), entries)
		if got != tc.match {
			t.Errorf("prefix %s: expected match=%v, got %v", tc.prefix, tc.match, got)
		}
	}
}

func TestMatchesPrefixList_CorporateSubnet(t *testing.T) {
	entries := []ImportPrefix{
		{Prefix: mustParsePrefix("10.100.0.0/16"), MaxLength: 24},
	}

	testCases := []struct {
		prefix string
		match  bool
	}{
		{"10.100.0.0/16", true},   // exact match
		{"10.100.1.0/24", true},   // within range
		{"10.100.50.0/24", true},  // within range
		{"10.100.1.0/25", false},  // too specific (25 > maxLength 24)
		{"10.100.1.1/32", false},  // too specific
		{"10.0.0.0/8", false},     // wider than entry
		{"10.101.0.0/16", false},  // different subnet
		{"192.168.0.0/16", false}, // completely different
		{"0.0.0.0/0", false},      // default route
		{"10.100.0.0/15", false},  // wider than the entry prefix itself
	}

	for _, tc := range testCases {
		got := matchesPrefixList(mustParsePrefix(tc.prefix), entries)
		if got != tc.match {
			t.Errorf("prefix %s: expected match=%v, got %v", tc.prefix, tc.match, got)
		}
	}
}

func TestMatchesPrefixList_EmptyRejectsAll(t *testing.T) {
	if matchesPrefixList(mustParsePrefix("0.0.0.0/0"), nil) {
		t.Fatal("empty prefix list should reject all routes")
	}

	if matchesPrefixList(mustParsePrefix("10.0.0.0/8"), []ImportPrefix{}) {
		t.Fatal("empty prefix list should reject all routes")
	}
}

func TestMatchesPrefixList_MultipleEntries(t *testing.T) {
	entries := []ImportPrefix{
		{Prefix: mustParsePrefix("0.0.0.0/0"), MaxLength: 0},
		{Prefix: mustParsePrefix("10.100.0.0/16"), MaxLength: 24},
	}

	testCases := []struct {
		prefix string
		match  bool
	}{
		{"0.0.0.0/0", true},       // matches first entry
		{"10.100.1.0/24", true},   // matches second entry
		{"192.168.0.0/16", false}, // matches neither
	}

	for _, tc := range testCases {
		got := matchesPrefixList(mustParsePrefix(tc.prefix), entries)
		if got != tc.match {
			t.Errorf("prefix %s: expected match=%v, got %v", tc.prefix, tc.match, got)
		}
	}
}

func TestOverlapsAny_UEPool(t *testing.T) {
	filter := &RouteFilter{
		RejectPrefixes: []netip.Prefix{
			mustParsePrefix("10.45.0.0/16"),
		},
	}

	testCases := []struct {
		prefix  string
		overlap bool
	}{
		{"10.45.0.0/16", true},    // exact match
		{"10.45.1.0/24", true},    // within UE pool
		{"10.45.0.1/32", true},    // host within UE pool
		{"10.0.0.0/8", false},     // wider prefix — not rejected (kernel LPM handles it)
		{"10.44.0.0/16", false},   // adjacent but not overlapping
		{"192.168.0.0/16", false}, // completely different
		{"0.0.0.0/0", false},      // default route — not rejected (kernel LPM handles it)
	}

	for _, tc := range testCases {
		got := filter.overlapsAny(mustParsePrefix(tc.prefix))
		if got != tc.overlap {
			t.Errorf("prefix %s: expected overlap=%v, got %v", tc.prefix, tc.overlap, got)
		}
	}
}

func TestOverlapsAny_HardcodedRejections(t *testing.T) {
	filter := &RouteFilter{
		RejectPrefixes: BuildRejectPrefixes(nil),
	}

	testCases := []struct {
		prefix  string
		overlap bool
	}{
		{"169.254.0.0/16", true}, // link-local
		{"169.254.1.0/24", true}, // within link-local
		{"224.0.0.0/4", true},    // multicast
		{"224.0.0.1/32", true},   // within multicast
		{"127.0.0.0/8", true},    // loopback
		{"127.0.0.1/32", true},   // within loopback
		{"10.0.0.0/8", false},    // normal prefix
		{"0.0.0.0/0", false},     // default route — not rejected (kernel LPM handles it)
	}

	for _, tc := range testCases {
		got := filter.overlapsAny(mustParsePrefix(tc.prefix))
		if got != tc.overlap {
			t.Errorf("prefix %s: expected overlap=%v, got %v", tc.prefix, tc.overlap, got)
		}
	}
}

func TestBuildRejectPrefixes_IncludesAllSources(t *testing.T) {
	subnets := []netip.Prefix{
		mustParsePrefix("10.45.0.0/16"),
		mustParsePrefix("192.168.1.0/24"),
	}

	prefixes := BuildRejectPrefixes(subnets)

	// Should contain: link-local, multicast, loopback, UE pool, extra
	if len(prefixes) != 5 {
		t.Fatalf("expected 5 reject prefixes, got %d", len(prefixes))
	}

	// Verify the UE pool is included
	filter := &RouteFilter{RejectPrefixes: prefixes}

	if !filter.overlapsAny(mustParsePrefix("10.45.1.0/24")) {
		t.Fatal("expected UE pool subnet to be rejected")
	}

	if !filter.overlapsAny(mustParsePrefix("192.168.1.128/25")) {
		t.Fatal("expected extra subnet to be rejected")
	}
}

func TestBuildRejectPrefixes_Empty(t *testing.T) {
	prefixes := BuildRejectPrefixes(nil)

	// Should still have the 3 hard-coded rejections
	if len(prefixes) != 3 {
		t.Fatalf("expected 3 reject prefixes, got %d", len(prefixes))
	}
}
