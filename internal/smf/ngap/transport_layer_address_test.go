// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package ngap

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/free5gc/aper"
	"github.com/stretchr/testify/assert"
)

func TestEncodeTransportLayerAddress_HappyPath(t *testing.T) {
	tests := []struct {
		name        string
		ipv4        netip.Addr
		ipv6        netip.Addr
		expectedLen int
		expectedHex string
	}{
		{
			name:        "IPv4 only",
			ipv4:        netip.MustParseAddr("192.168.1.1"),
			ipv6:        netip.Addr{},
			expectedLen: 32,
			expectedHex: "c0a80101",
		},
		{
			name:        "IPv6 only",
			ipv4:        netip.Addr{},
			ipv6:        netip.MustParseAddr("2001:db8::1"),
			expectedLen: 128,
			expectedHex: "20010db8000000000000000000000001",
		},
		{
			name:        "dual-stack (IPv4 + IPv6)",
			ipv4:        netip.MustParseAddr("192.168.1.1"),
			ipv6:        netip.MustParseAddr("2001:db8::1"),
			expectedLen: 160,
			expectedHex: "c0a8010120010db8000000000000000000000001",
		},
		{
			name:        "IPv4 loopback",
			ipv4:        netip.MustParseAddr("127.0.0.1"),
			ipv6:        netip.Addr{},
			expectedLen: 32,
			expectedHex: "7f000001",
		},
		{
			name:        "IPv6 loopback",
			ipv4:        netip.Addr{},
			ipv6:        netip.MustParseAddr("::1"),
			expectedLen: 128,
			expectedHex: "00000000000000000000000000000001",
		},
		{
			name:        "IPv4 any address",
			ipv4:        netip.MustParseAddr("0.0.0.0"),
			ipv6:        netip.Addr{},
			expectedLen: 32,
			expectedHex: "00000000",
		},
		{
			name:        "IPv6 any address",
			ipv4:        netip.Addr{},
			ipv6:        netip.MustParseAddr("::"),
			expectedLen: 128,
			expectedHex: "00000000000000000000000000000000",
		},
		{
			name:        "dual-stack with multiple IPs",
			ipv4:        netip.MustParseAddr("10.0.0.5"),
			ipv6:        netip.MustParseAddr("fd00::1234"),
			expectedLen: 160,
			expectedHex: "0a000005fd000000000000000000000000001234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := encodeTransportLayerAddress(tt.ipv4, tt.ipv6)
			assert.NoError(t, err)

			assert.Equal(t, uint64(tt.expectedLen), result.BitLength)

			if tt.expectedHex != "" {
				expectedBytes, err := hexToBytes(tt.expectedHex)
				assert.NoError(t, err)
				assert.Equal(t, expectedBytes, result.Bytes)
			}
		})
	}
}

func TestEncodeTransportLayerAddress_ErrorCases(t *testing.T) {
	tests := []struct {
		name string
		ipv4 netip.Addr
		ipv6 netip.Addr
	}{
		{
			name: "both addresses invalid",
			ipv4: netip.Addr{},
			ipv6: netip.Addr{},
		},
		{
			name: "IPv4 is IPv6 address",
			ipv4: netip.MustParseAddr("2001:db8::1"),
			ipv6: netip.Addr{},
		},
		{
			name: "IPv6 is IPv4 address",
			ipv4: netip.Addr{},
			ipv6: netip.MustParseAddr("192.168.1.1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := encodeTransportLayerAddress(tt.ipv4, tt.ipv6)
			assert.Error(t, err)
			assert.Equal(t, aper.BitString{}, result)
		})
	}
}

func TestParseTransportLayerAddress_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		bitLength int
		hexBytes  string
		wantV4    string
		wantV6    string
	}{
		{
			name:      "IPv4 only (32 bits)",
			bitLength: 32,
			hexBytes:  "c0a80101",
			wantV4:    "192.168.1.1",
			wantV6:    "",
		},
		{
			name:      "IPv6 only (128 bits)",
			bitLength: 128,
			hexBytes:  "20010db8000000000000000000000001",
			wantV4:    "",
			wantV6:    "2001:db8::1",
		},
		{
			name:      "dual-stack (160 bits)",
			bitLength: 160,
			hexBytes:  "c0a8010120010db8000000000000000000000001",
			wantV4:    "192.168.1.1",
			wantV6:    "2001:db8::1",
		},
		{
			name:      "IPv4 loopback (32 bits)",
			bitLength: 32,
			hexBytes:  "7f000001",
			wantV4:    "127.0.0.1",
			wantV6:    "",
		},
		{
			name:      "IPv6 loopback (128 bits)",
			bitLength: 128,
			hexBytes:  "00000000000000000000000000000001",
			wantV4:    "",
			wantV6:    "::1",
		},
		{
			name:      "IPv4 any (32 bits)",
			bitLength: 32,
			hexBytes:  "00000000",
			wantV4:    "0.0.0.0",
			wantV6:    "",
		},
		{
			name:      "IPv6 any (128 bits)",
			bitLength: 128,
			hexBytes:  "00000000000000000000000000000000",
			wantV4:    "",
			wantV6:    "::",
		},
		{
			name:      "dual-stack with multiple IPs (160 bits)",
			bitLength: 160,
			hexBytes:  "0a000005fd000000000000000000000000001234",
			wantV4:    "10.0.0.5",
			wantV6:    "fd00::1234",
		},
		{
			name:      "IPv4 multicast (32 bits)",
			bitLength: 32,
			hexBytes:  "efff0001",
			wantV4:    "239.255.0.1",
			wantV6:    "",
		},
		{
			name:      "IPv6 link-local (128 bits)",
			bitLength: 128,
			hexBytes:  "fe800000000000000000000000000001",
			wantV4:    "",
			wantV6:    "fe80::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := hexToBytes(tt.hexBytes)
			assert.NoError(t, err)

			bs := aper.BitString{
				Bytes:     bytes,
				BitLength: uint64(tt.bitLength),
			}

			ipv4, ipv6 := ParseTransportLayerAddress(bs)

			if tt.wantV4 != "" {
				assert.Equal(t, tt.wantV4, ipv4.String())
			} else {
				assert.Nil(t, ipv4)
			}

			if tt.wantV6 != "" {
				assert.Equal(t, tt.wantV6, ipv6.String())
			} else {
				assert.Nil(t, ipv6)
			}
		})
	}
}

func hexToBytes(s string) ([]byte, error) {
	n := len(s) / 2

	bytes := make([]byte, n)
	for i := 0; i < n; i++ {
		var b byte

		_, err := fmt.Sscanf(s[i*2:i*2+2], "%02x", &b)
		if err != nil {
			return nil, err
		}

		bytes[i] = b
	}

	return bytes, nil
}
