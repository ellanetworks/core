// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"net/netip"
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

	if n3IPv4.IsValid() {
		t.Fatalf("n3IPv4 = %v, want empty", n3IPv4)
	}

	if got, want := n3IPv6, netip.MustParseAddr("2001:db8::1"); got != want {
		t.Fatalf("n3IPv6 = %v, want %v", got, want)
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

	if got, want := n3IPv4, netip.MustParseAddr("192.0.2.1"); got != want {
		t.Fatalf("n3IPv4 = %v, want %v", got, want)
	}

	if n3IPv6.IsValid() {
		t.Fatalf("n3IPv6 = %v, want empty", n3IPv6)
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

	if got, want := n3IPv4, netip.MustParseAddr("192.0.2.20"); got != want {
		t.Fatalf("n3IPv4 = %v, want %v", got, want)
	}

	if got, want := n3IPv6, netip.MustParseAddr("2001:db8::20"); got != want {
		t.Fatalf("n3IPv6 = %v, want %v", got, want)
	}
}

func TestResolveN3AddressesScansVlanNetdevNotMaster(t *testing.T) {
	originalGetInterfaceIPs := getInterfaceIPs

	t.Cleanup(func() {
		getInterfaceIPs = originalGetInterfaceIPs
	})

	getInterfaceIPs = func(name string) ([]string, error) {
		if name != "ens4.100" {
			t.Fatalf("unexpected interface lookup: %s", name)
		}

		return []string{"10.1.1.5", "2001:db8::5"}, nil
	}

	n3IPv4, n3IPv6 := resolveN3Addresses(config.N3Interface{
		Name:       "ens4.100",
		Address:    "10.1.1.5",
		VlanConfig: &config.VlanConfig{MasterInterface: "ens4"},
	})

	if got, want := n3IPv4, netip.MustParseAddr("10.1.1.5"); got != want {
		t.Fatalf("n3IPv4 = %v, want %v", got, want)
	}

	if got, want := n3IPv6, netip.MustParseAddr("2001:db8::5"); got != want {
		t.Fatalf("n3IPv6 = %v, want %v", got, want)
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

	if got := []string{n3IPv4.String(), n3IPv6.String()}; !reflect.DeepEqual(got, expected) {
		t.Fatalf("resolved addresses = %v, want %v", got, expected)
	}
}
