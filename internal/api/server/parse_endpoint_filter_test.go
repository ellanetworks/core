// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"testing"
)

func TestParseEndpointFilter(t *testing.T) {
	u16 := func(v uint16) *uint16 { return &v }

	tests := []struct {
		name     string
		input    string
		wantIP   string
		wantPort *uint16
		wantErr  bool
	}{
		// IPv4 only
		{name: "ipv4 only", input: "10.0.0.1", wantIP: "10.0.0.1", wantPort: nil},
		// IPv4 + port
		{name: "ipv4 with port", input: "10.0.0.1:443", wantIP: "10.0.0.1", wantPort: u16(443)},
		// Port only
		{name: "port only", input: ":8080", wantIP: "", wantPort: u16(8080)},
		// Port zero
		{name: "port zero", input: ":0", wantIP: "", wantPort: u16(0)},
		// IPv6 only
		{name: "ipv6 only", input: "::1", wantIP: "::1", wantPort: nil},
		{name: "ipv6 full", input: "2001:db8::1", wantIP: "2001:db8::1", wantPort: nil},
		// IPv6 + port (bracket notation)
		{name: "ipv6 with port", input: "[::1]:443", wantIP: "::1", wantPort: u16(443)},
		{name: "ipv6 full with port", input: "[2001:db8::1]:80", wantIP: "2001:db8::1", wantPort: u16(80)},
		// Error: invalid IP
		{name: "invalid ip", input: "foobar", wantErr: true},
		{name: "invalid ip with dots", input: "not.an.ip.address", wantErr: true},
		// Error: invalid port
		{name: "invalid port letters", input: ":abc", wantErr: true},
		{name: "invalid port too large", input: ":99999", wantErr: true},
		{name: "invalid port on ipv4", input: "10.0.0.1:abc", wantErr: true},
		{name: "port out of range on ipv4", input: "10.0.0.1:70000", wantErr: true},
		// Error: invalid ip with port
		{name: "bad ip with port", input: "foobar:443", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIP, gotPort, err := parseEndpointFilter(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got ip=%q port=%v", tt.input, gotIP, gotPort)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tt.input, err)
			}

			if gotIP != tt.wantIP {
				t.Errorf("ip: got %q, want %q", gotIP, tt.wantIP)
			}

			if tt.wantPort == nil && gotPort != nil {
				t.Errorf("port: got %d, want nil", *gotPort)
			} else if tt.wantPort != nil && gotPort == nil {
				t.Errorf("port: got nil, want %d", *tt.wantPort)
			} else if tt.wantPort != nil && gotPort != nil && *gotPort != *tt.wantPort {
				t.Errorf("port: got %d, want %d", *gotPort, *tt.wantPort)
			}
		})
	}
}
