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
			pool, err := NewPool(1, tt.cidr)
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
	pool, _ := NewPool(1, "192.168.1.0/24")
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
			pool, err := NewPool(1, tt.cidr)
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
	pool, _ := NewPool(1, "192.168.1.0/24")

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
	pool, _ := NewPool(1, "10.45.0.0/22")

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
	pool, _ := NewPool(1, "192.168.1.0/24")

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
	pool, _ := NewPool(1, "10.45.0.0/22")

	addr := netip.MustParseAddr("10.45.3.254")

	got := pool.OffsetOf(addr)
	if got != 1022 {
		t.Fatalf("OffsetOf(10.45.3.254): expected 1022, got %d", got)
	}
}

func TestPoolRoundTrip(t *testing.T) {
	pool, _ := NewPool(1, "10.45.0.0/22")

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
