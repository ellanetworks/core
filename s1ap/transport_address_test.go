// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "testing"

// TS 36.414 §5.1: the bit string carries an IPv4 address, an IPv6 address, or
// both with the IPv4 one first.
func TestTransportLayerAddressIPs(t *testing.T) {
	v4 := []byte{192, 168, 1, 1}
	v6 := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}

	tests := []struct {
		name  string
		addr  TransportLayerAddress
		want4 string
		want6 string
	}{
		{"ipv4", TransportLayerAddress(v4), "192.168.1.1", ""},
		{"ipv6", TransportLayerAddress(v6), "", "2001:db8::1"},
		{"both", TransportLayerAddress(append(append([]byte{}, v4...), v6...)), "192.168.1.1", "2001:db8::1"},
		{"truncated", TransportLayerAddress{1, 2, 3}, "", ""},
		{"empty", nil, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipv4, ipv6 := tt.addr.IPs()

			got4, got6 := "", ""
			if ipv4.IsValid() {
				got4 = ipv4.String()
			}

			if ipv6.IsValid() {
				got6 = ipv6.String()
			}

			if got4 != tt.want4 || got6 != tt.want6 {
				t.Errorf("IPs() = %q/%q, want %q/%q", got4, got6, tt.want4, tt.want6)
			}
		})
	}
}
