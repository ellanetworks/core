// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models_test

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func TestParseN3ExternalAddress(t *testing.T) {
	tests := []struct {
		name  string
		input string
		v4    string
		v6    string
	}{
		{name: "ipv4 only", input: "33.33.33.5", v4: "33.33.33.5"},
		{name: "ipv6 only", input: "2001:db8::5", v6: "2001:db8::5"},
		{name: "dual stack", input: "33.33.33.5,2001:db8::5", v4: "33.33.33.5", v6: "2001:db8::5"},
		{name: "dual stack ipv6 first", input: "2001:db8::5,33.33.33.5", v4: "33.33.33.5", v6: "2001:db8::5"},
		{name: "dual stack with spaces", input: " 33.33.33.5 , 2001:db8::5 ", v4: "33.33.33.5", v6: "2001:db8::5"},
		{name: "ipv4 mapped ipv6 is ipv4", input: "::ffff:33.33.33.5", v4: "33.33.33.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v4, v6, err := models.ParseN3ExternalAddress(tt.input)
			if err != nil {
				t.Fatalf("ParseN3ExternalAddress(%q): %v", tt.input, err)
			}

			if got := addrText(v4); got != tt.v4 {
				t.Errorf("IPv4: got %q, want %q", got, tt.v4)
			}

			if got := addrText(v6); got != tt.v6 {
				t.Errorf("IPv6: got %q, want %q", got, tt.v6)
			}
		})
	}
}

func TestParseN3ExternalAddressRejects(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "not an address", input: "not-an-ip"},
		{name: "two ipv4", input: "33.33.33.5,33.33.33.6"},
		{name: "two ipv6", input: "2001:db8::5,2001:db8::6"},
		{name: "trailing comma", input: "33.33.33.5,"},
		{name: "cidr", input: "33.33.33.5/32"},
		{name: "three addresses", input: "33.33.33.5,2001:db8::5,2001:db8::6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := models.ParseN3ExternalAddress(tt.input); err == nil {
				t.Fatalf("ParseN3ExternalAddress(%q): expected error", tt.input)
			}
		})
	}
}

func addrText(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}

	return addr.String()
}
