// Copyright 2026 Ella Networks

package ipam

import (
	"net/netip"
	"testing"
)

func TestNewPool(t *testing.T) {
	tests := []struct {
		name    string
		cidr    string
		wantErr bool
		wantNet string // expected normalized prefix
	}{
		{"slash24", "192.168.1.0/24", false, "192.168.1.0/24"},
		{"slash22", "10.45.0.0/22", false, "10.45.0.0/22"},
		{"slash29", "10.0.0.0/29", false, "10.0.0.0/29"},
		{"normalizes host bits", "10.45.0.5/22", false, "10.45.0.0/22"},
		{"invalid", "not-a-cidr", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewPool("test-pool", tt.cidr)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if pool.Prefix.String() != tt.wantNet {
				t.Fatalf("expected prefix %s, got %s", tt.wantNet, pool.Prefix.String())
			}
		})
	}
}

func TestPoolFirstUsable(t *testing.T) {
	pool, _ := NewPool("test-pool", "192.168.1.0/24")
	if pool.FirstUsable() != 1 {
		t.Fatalf("expected FirstUsable=1 for IPv4, got %d", pool.FirstUsable())
	}
}

func TestPoolSize(t *testing.T) {
	tests := []struct {
		cidr     string
		wantSize int
	}{
		{"192.168.1.0/24", 254},   // 256 - 2
		{"10.45.0.0/22", 1022},    // 1024 - 2
		{"10.0.0.0/29", 6},        // 8 - 2
		{"192.168.1.0/30", 2},     // 4 - 2
		{"192.168.1.128/25", 126}, // 128 - 2
	}

	for _, tt := range tests {
		t.Run(tt.cidr, func(t *testing.T) {
			pool, err := NewPool("test-pool", tt.cidr)
			if err != nil {
				t.Fatalf("NewPool: %v", err)
			}

			if pool.Size() != tt.wantSize {
				t.Fatalf("Size(%s): expected %d, got %d", tt.cidr, tt.wantSize, pool.Size())
			}
		})
	}
}

func TestPoolAddressAtOffset(t *testing.T) {
	pool, _ := NewPool("test-pool", "192.168.1.0/24")

	tests := []struct {
		offset int
		want   string
	}{
		{0, "192.168.1.0"}, // network address (offset 0)
		{1, "192.168.1.1"}, // first usable
		{2, "192.168.1.2"},
		{254, "192.168.1.254"},
		{255, "192.168.1.255"}, // broadcast
	}

	for _, tt := range tests {
		got := pool.AddressAtOffset(tt.offset)
		if got.String() != tt.want {
			t.Fatalf("AddressAtOffset(%d): expected %s, got %s", tt.offset, tt.want, got.String())
		}
	}
}

func TestPoolAddressAtOffset_Slash22(t *testing.T) {
	pool, _ := NewPool("test-pool", "10.45.0.0/22")

	// First usable = 10.45.0.1 (offset 1)
	got := pool.AddressAtOffset(1)
	if got.String() != "10.45.0.1" {
		t.Fatalf("expected 10.45.0.1, got %s", got.String())
	}

	// Last usable = 10.45.3.254 (offset 1022)
	got = pool.AddressAtOffset(1022)
	if got.String() != "10.45.3.254" {
		t.Fatalf("expected 10.45.3.254, got %s", got.String())
	}

	// Offset 256 crosses octet boundary: 10.45.1.0
	got = pool.AddressAtOffset(256)
	if got.String() != "10.45.1.0" {
		t.Fatalf("expected 10.45.1.0, got %s", got.String())
	}
}

func TestPoolOffsetOf(t *testing.T) {
	pool, _ := NewPool("test-pool", "192.168.1.0/24")

	tests := []struct {
		addr       string
		wantOffset int
	}{
		{"192.168.1.0", 0},
		{"192.168.1.1", 1},
		{"192.168.1.254", 254},
		{"192.168.1.255", 255},
		{"10.0.0.1", -1}, // outside pool
	}

	for _, tt := range tests {
		addr := netip.MustParseAddr(tt.addr)

		got := pool.OffsetOf(addr)
		if got != tt.wantOffset {
			t.Fatalf("OffsetOf(%s): expected %d, got %d", tt.addr, tt.wantOffset, got)
		}
	}
}

func TestPoolOffsetOf_Slash22(t *testing.T) {
	pool, _ := NewPool("test-pool", "10.45.0.0/22")

	addr := netip.MustParseAddr("10.45.3.254")

	got := pool.OffsetOf(addr)
	if got != 1022 {
		t.Fatalf("OffsetOf(10.45.3.254): expected 1022, got %d", got)
	}
}

func TestPoolRoundTrip(t *testing.T) {
	pool, _ := NewPool("test-pool", "10.45.0.0/22")

	// For every offset in the usable range, AddressAtOffset and OffsetOf
	// must be inverses of each other.
	for offset := pool.FirstUsable(); offset < pool.FirstUsable()+pool.Size(); offset++ {
		addr := pool.AddressAtOffset(offset)

		gotOffset := pool.OffsetOf(addr)
		if gotOffset != offset {
			t.Fatalf("roundtrip failed: offset %d → %s → offset %d", offset, addr, gotOffset)
		}
	}
}

// --- IPv6 prefix delegation tests ---

func TestNewPool6(t *testing.T) {
	tests := []struct {
		name      string
		cidr      string
		prefixLen int
		wantErr   bool
		wantNet   string
	}{
		{"slash60_prefix64", "2001:db8:abcd:1230::/60", 64, false, "2001:db8:abcd:1230::/60"},
		{"slash48_prefix64", "2001:db8:abcd::/48", 64, false, "2001:db8:abcd::/48"},
		{"normalizes_host_bits", "2001:db8:abcd:1234:ffff::/60", 64, false, "2001:db8:abcd:1230::/60"},
		{"invalid_cidr", "not-a-cidr", 64, true, ""},
		{"ipv4_rejected", "10.0.0.0/24", 32, true, ""},
		{"prefixLen_too_small", "2001:db8::/48", 32, true, ""},
		{"prefixLen_too_large", "2001:db8::/48", 129, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewPool6("test-pool", tt.cidr, tt.prefixLen)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if pool.Prefix.String() != tt.wantNet {
				t.Fatalf("expected prefix %s, got %s", tt.wantNet, pool.Prefix.String())
			}

			if pool.PrefixLen != tt.prefixLen {
				t.Fatalf("expected PrefixLen %d, got %d", tt.prefixLen, pool.PrefixLen)
			}
		})
	}
}

func TestNewPool_DefaultPrefixLen(t *testing.T) {
	pool, err := NewPool("test-pool", "192.168.1.0/24")
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}

	if pool.PrefixLen != 32 {
		t.Fatalf("expected PrefixLen=32 for IPv4, got %d", pool.PrefixLen)
	}
}

func TestPool6FirstUsable(t *testing.T) {
	pool, _ := NewPool6("test-pool", "2001:db8:abcd:1230::/60", 64)
	if pool.FirstUsable() != 0 {
		t.Fatalf("expected FirstUsable=0 for IPv6, got %d", pool.FirstUsable())
	}
}

func TestPool6Size(t *testing.T) {
	tests := []struct {
		name      string
		cidr      string
		prefixLen int
		wantSize  int
	}{
		{"slash60_prefix64", "2001:db8:abcd:1230::/60", 64, 16},                   // 1 << (64-60) = 16
		{"slash56_prefix64", "2001:db8:abcd:1200::/56", 64, 256},                  // 1 << (64-56) = 256
		{"slash48_prefix64", "2001:db8:abcd::/48", 64, 65536},                     // 1 << (64-48) = 65536
		{"slash64_prefix64", "2001:db8:abcd:1234::/64", 64, 1},                    // 1 << (64-64) = 1
		{"slash60_prefix128", "2001:db8:abcd:1230::/60", 128, int(^uint(0) >> 1)}, // allocBits=68 >= 31 → max int
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewPool6("test-pool", tt.cidr, tt.prefixLen)
			if err != nil {
				t.Fatalf("NewPool6: %v", err)
			}

			if pool.Size() != tt.wantSize {
				t.Fatalf("Size(): expected %d, got %d", tt.wantSize, pool.Size())
			}
		})
	}
}

func TestPool6AddressAtOffset(t *testing.T) {
	// A /60 pool with PrefixLen=64 yields 16 /64 prefixes.
	pool, _ := NewPool6("test-pool", "2001:db8:abcd:1230::/60", 64)

	tests := []struct {
		offset int
		want   string
	}{
		{0, "2001:db8:abcd:1230::"},
		{1, "2001:db8:abcd:1231::"},
		{2, "2001:db8:abcd:1232::"},
		{15, "2001:db8:abcd:123f::"},
	}

	for _, tt := range tests {
		got := pool.AddressAtOffset(tt.offset)
		if got.String() != tt.want {
			t.Fatalf("AddressAtOffset(%d): expected %s, got %s", tt.offset, tt.want, got.String())
		}
	}
}

func TestPool6AddressAtOffset_Slash48(t *testing.T) {
	// A /48 pool with PrefixLen=64 yields 65536 /64 prefixes.
	pool, _ := NewPool6("test-pool", "2001:db8:abcd::/48", 64)

	tests := []struct {
		offset int
		want   string
	}{
		{0, "2001:db8:abcd::"},
		{1, "2001:db8:abcd:1::"},
		{256, "2001:db8:abcd:100::"},
		{65535, "2001:db8:abcd:ffff::"},
	}

	for _, tt := range tests {
		got := pool.AddressAtOffset(tt.offset)
		if got.String() != tt.want {
			t.Fatalf("AddressAtOffset(%d): expected %s, got %s", tt.offset, tt.want, got.String())
		}
	}
}

func TestPool6OffsetOf(t *testing.T) {
	pool, _ := NewPool6("test-pool", "2001:db8:abcd:1230::/60", 64)

	tests := []struct {
		addr       string
		wantOffset int
	}{
		{"2001:db8:abcd:1230::", 0},
		{"2001:db8:abcd:1231::", 1},
		{"2001:db8:abcd:123f::", 15},
		{"2001:db8:ffff::", -1}, // outside pool
	}

	for _, tt := range tests {
		addr := netip.MustParseAddr(tt.addr)

		got := pool.OffsetOf(addr)
		if got != tt.wantOffset {
			t.Fatalf("OffsetOf(%s): expected %d, got %d", tt.addr, tt.wantOffset, got)
		}
	}
}

func TestPool6RoundTrip(t *testing.T) {
	pool, _ := NewPool6("test-pool", "2001:db8:abcd:1230::/60", 64)

	// All 16 /64 prefixes must round-trip.
	for offset := pool.FirstUsable(); offset < pool.FirstUsable()+pool.Size(); offset++ {
		addr := pool.AddressAtOffset(offset)

		gotOffset := pool.OffsetOf(addr)
		if gotOffset != offset {
			t.Fatalf("roundtrip failed: offset %d → %s → offset %d", offset, addr, gotOffset)
		}
	}
}

func TestPool6RoundTrip_Slash48(t *testing.T) {
	pool, _ := NewPool6("test-pool", "2001:db8:abcd::/48", 64)

	// Spot-check a subset (full range is 65536).
	for _, offset := range []int{0, 1, 100, 256, 1000, 32768, 65535} {
		addr := pool.AddressAtOffset(offset)

		gotOffset := pool.OffsetOf(addr)
		if gotOffset != offset {
			t.Fatalf("roundtrip failed: offset %d → %s → offset %d", offset, addr, gotOffset)
		}
	}
}

func TestPool6LowerBitsZero(t *testing.T) {
	// Verify that delegated prefix base addresses have lower 64 bits = 0
	// (IID region is zeros).
	pool, _ := NewPool6("test-pool", "2001:db8:abcd:1230::/60", 64)

	for offset := 0; offset < pool.Size(); offset++ {
		addr := pool.AddressAtOffset(offset)

		b := addr.As16()
		for i := 8; i < 16; i++ {
			if b[i] != 0 {
				t.Fatalf("offset %d (%s): lower 64 bits not zero at byte %d (0x%02X)",
					offset, addr, i, b[i])
			}
		}
	}
}
