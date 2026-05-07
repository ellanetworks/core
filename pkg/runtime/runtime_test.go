package runtime

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/config"
)

func TestResolveN3AddressesConfiguredIPv6StillFindsIPv4OnInterface(t *testing.T) {
	originalGetInterfaceIPs := getInterfaceIPs

	t.Cleanup(func() {
		getInterfaceIPs = originalGetInterfaceIPs
	})

	getInterfaceIPs = func(name string) ([]string, error) {
		if name != "n3eth0" {
			t.Fatalf("unexpected interface lookup: %s", name)
		}

		return []string{"2001:db8::10", "192.0.2.10"}, nil
	}

	n3IPv4, n3IPv6 := resolveN3Addresses(config.N3Interface{
		Name:    "n3eth0",
		Address: "2001:db8::1",
	})

	if got, want := n3IPv4, "192.0.2.10"; got != want {
		t.Fatalf("n3IPv4 = %q, want %q", got, want)
	}

	if got, want := n3IPv6, "2001:db8::1"; got != want {
		t.Fatalf("n3IPv6 = %q, want %q", got, want)
	}
}

func TestResolveN3AddressesConfiguredIPv4StillFindsIPv6OnInterface(t *testing.T) {
	originalGetInterfaceIPs := getInterfaceIPs

	t.Cleanup(func() {
		getInterfaceIPs = originalGetInterfaceIPs
	})

	getInterfaceIPs = func(name string) ([]string, error) {
		if name != "n3eth0" {
			t.Fatalf("unexpected interface lookup: %s", name)
		}

		return []string{"192.0.2.20", "2001:db8::20"}, nil
	}

	n3IPv4, n3IPv6 := resolveN3Addresses(config.N3Interface{
		Name:    "n3eth0",
		Address: "192.0.2.1",
	})

	if got, want := n3IPv4, "192.0.2.1"; got != want {
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
