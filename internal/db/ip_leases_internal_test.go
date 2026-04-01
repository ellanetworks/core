// Copyright 2026 Ella Networks

package db

import (
	"bytes"
	"testing"
)

func TestIPToSortableBytes(t *testing.T) {
	// IPv4-mapped prefix: 10 zero bytes + ff ff.
	ipv4MappedPrefix := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff}

	tests := []struct {
		name     string
		address  string
		expected []byte
		wantErr  bool
	}{
		{
			name:     "0.0.0.0",
			address:  "0.0.0.0",
			expected: append(append([]byte{}, ipv4MappedPrefix...), 0x00, 0x00, 0x00, 0x00),
		},
		{
			name:     "255.255.255.255",
			address:  "255.255.255.255",
			expected: append(append([]byte{}, ipv4MappedPrefix...), 0xff, 0xff, 0xff, 0xff),
		},
		{
			name:     "10.45.0.1",
			address:  "10.45.0.1",
			expected: append(append([]byte{}, ipv4MappedPrefix...), 0x0a, 0x2d, 0x00, 0x01),
		},
		{
			name:    "IPv6 2001:db8::1",
			address: "2001:db8::1",
			expected: []byte{
				0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
			},
		},
		{
			name:    "invalid address",
			address: "not-an-ip",
			wantErr: true,
		},
		{
			name:    "empty string",
			address: "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ipToSortableBytes(tc.address)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tc.address)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.address, err)
			}

			if len(got) != 16 {
				t.Fatalf("expected 16 bytes, got %d", len(got))
			}

			if !bytes.Equal(got, tc.expected) {
				t.Fatalf("address %q: expected %x, got %x", tc.address, tc.expected, got)
			}
		})
	}

	// Verify all IPv4 results share the IPv4-mapped prefix.
	ipv4Addresses := []string{"0.0.0.0", "255.255.255.255", "10.45.0.1", "192.168.1.10"}

	for _, addr := range ipv4Addresses {
		b, err := ipToSortableBytes(addr)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", addr, err)
		}

		if !bytes.Equal(b[:12], ipv4MappedPrefix) {
			t.Errorf("address %q: first 12 bytes = %x, want IPv4-mapped prefix %x", addr, b[:12], ipv4MappedPrefix)
		}
	}
}
