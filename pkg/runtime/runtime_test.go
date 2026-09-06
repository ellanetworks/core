// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/config"
)

func TestResolveN3AddressesConfiguredIPv6IsAuthoritative(t *testing.T) {
	originalGetInterfaceIPs := getInterfaceIPs

	t.Cleanup(func() {
		getInterfaceIPs = originalGetInterfaceIPs
	})

	getInterfaceIPs = func(name string) ([]string, error) {
		t.Fatalf("interface must not be scanned when an address is configured: %s", name)

		return nil, nil
	}

	n3IPv4, n3IPv6 := resolveN3Addresses(config.N3Interface{
		Name:            "n3eth0",
		Address:         "2001:db8::1",
		AddressExplicit: true,
	})

	if n3IPv4 != "" {
		t.Fatalf("n3IPv4 = %q, want empty", n3IPv4)
	}

	if got, want := n3IPv6, "2001:db8::1"; got != want {
		t.Fatalf("n3IPv6 = %q, want %q", got, want)
	}
}

func TestResolveN3AddressesConfiguredIPv4IsAuthoritative(t *testing.T) {
	originalGetInterfaceIPs := getInterfaceIPs

	t.Cleanup(func() {
		getInterfaceIPs = originalGetInterfaceIPs
	})

	getInterfaceIPs = func(name string) ([]string, error) {
		t.Fatalf("interface must not be scanned when an address is configured: %s", name)

		return nil, nil
	}

	n3IPv4, n3IPv6 := resolveN3Addresses(config.N3Interface{
		Name:            "n3eth0",
		Address:         "192.0.2.1",
		AddressExplicit: true,
	})

	if got, want := n3IPv4, "192.0.2.1"; got != want {
		t.Fatalf("n3IPv4 = %q, want %q", got, want)
	}

	if n3IPv6 != "" {
		t.Fatalf("n3IPv6 = %q, want empty", n3IPv6)
	}
}

func TestResolveN3AddressesDerivedAddressStillScansBothFamilies(t *testing.T) {
	originalGetInterfaceIPs := getInterfaceIPs

	t.Cleanup(func() {
		getInterfaceIPs = originalGetInterfaceIPs
	})

	getInterfaceIPs = func(name string) ([]string, error) {
		return []string{"192.0.2.20", "2001:db8::20"}, nil
	}

	n3IPv4, n3IPv6 := resolveN3Addresses(config.N3Interface{
		Name:    "n3eth0",
		Address: "2001:db8::20",
	})

	if got, want := n3IPv4, "192.0.2.20"; got != want {
		t.Fatalf("n3IPv4 = %q, want %q", got, want)
	}

	if got, want := n3IPv6, "2001:db8::20"; got != want {
		t.Fatalf("n3IPv6 = %q, want %q", got, want)
	}
}

func TestResolveN3AddressesUsesScannedAddressesWhenUnconfigured(t *testing.T) {
	originalGetInterfaceIPs := getInterfaceIPs

	t.Cleanup(func() {
		getInterfaceIPs = originalGetInterfaceIPs
	})

	expected := []string{"192.0.2.30", "2001:db8::30"}
	getInterfaceIPs = func(name string) ([]string, error) {
		if name != "n3eth0" {
			t.Fatalf("unexpected interface lookup: %s", name)
		}

		return expected, nil
	}

	n3IPv4, n3IPv6 := resolveN3Addresses(config.N3Interface{Name: "n3eth0"})

	if got := []string{n3IPv4, n3IPv6}; !reflect.DeepEqual(got, expected) {
		t.Fatalf("resolved addresses = %v, want %v", got, expected)
	}
}
