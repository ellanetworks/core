// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

// stubInterfaceIPs replaces the host lookup with a fixed set of addresses for
// the duration of the test, on an interface that is up.
func stubInterfaceIPs(t *testing.T, v4, v6 string) {
	t.Helper()

	stubLinkIsUp(t, true)

	original := config.GetInterfaceIPFunc

	t.Cleanup(func() {
		config.GetInterfaceIPFunc = original
	})

	config.GetInterfaceIPFunc = func(_ string, family config.AddressFamily) (string, error) {
		switch {
		case family == config.IPv4 && v4 != "":
			return v4, nil
		case family == config.IPv6 && v6 != "":
			return v6, nil
		}

		return "", config.ErrNoInterfaceIP
	}
}

// stubLinkIsUp fixes the administrative state of the interface for the duration
// of the test.
func stubLinkIsUp(t *testing.T, up bool) {
	t.Helper()

	original := linkIsUp

	t.Cleanup(func() {
		linkIsUp = original
	})

	linkIsUp = func(string) bool { return up }
}

func TestLookupN6Addresses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		v4, v6 string
	}{
		{name: "both", v4: "192.0.2.10", v6: "2001:db8::10"},
		{name: "ipv4 only", v4: "192.0.2.10"},
		{name: "ipv6 only", v6: "2001:db8::10"},
		{name: "neither"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubInterfaceIPs(t, tc.v4, tc.v6)

			v4, v6 := lookupN6Addresses("n6eth0")

			if got := addrString(v4); got != tc.v4 {
				t.Fatalf("IPv4 = %q, want %q", got, tc.v4)
			}

			if got := addrString(v6); got != tc.v6 {
				t.Fatalf("IPv6 = %q, want %q", got, tc.v6)
			}
		})
	}
}

func addrString(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}

	return addr.String()
}

func TestLookupN6AddressesTreatsADownedLinkAsAddressless(t *testing.T) {
	stubInterfaceIPs(t, "192.0.2.10", "2001:db8::10")
	stubLinkIsUp(t, false)

	v4, v6 := lookupN6Addresses("n6eth0")

	if v4.IsValid() || v6.IsValid() {
		t.Fatalf("addresses = %v, %v; want both empty while the link is down", v4, v6)
	}
}

func TestReconcileN6AddressesAppliesAcquiredAddresses(t *testing.T) {
	svc := bgp.New(netip.Addr{}, netip.Addr{}, zap.NewNop())
	svc.SetListenPort(-1)

	var announced []netip.Addr

	stubInterfaceIPs(t, "192.0.2.10", "2001:db8::10")
	reconcileN6Addresses("n6eth0", svc, func(v4 netip.Addr) {
		announced = append(announced, v4)
	})

	v4, v6 := svc.N6Addresses()
	if v4.String() != "192.0.2.10" || v6.String() != "2001:db8::10" {
		t.Fatalf("N6Addresses = %v, %v; want 192.0.2.10, 2001:db8::10", v4, v6)
	}

	// The callback is what lets a node that booted without an address pick up
	// a router ID, so an acquired IPv4 address has to reach it.
	if len(announced) != 1 || announced[0] != v4 {
		t.Fatalf("callback got %v, want [%v]", announced, v4)
	}
}

func TestReconcileN6AddressesAppliesLostAddresses(t *testing.T) {
	svc := bgp.New(netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("2001:db8::10"), zap.NewNop())
	svc.SetListenPort(-1)

	// Only IPv6 goes away: the watcher must not clear the address that is
	// still assigned.
	stubInterfaceIPs(t, "192.0.2.10", "")
	reconcileN6Addresses("n6eth0", svc, nil)

	v4, v6 := svc.N6Addresses()
	if v4.String() != "192.0.2.10" {
		t.Fatalf("IPv4 = %v, want 192.0.2.10", v4)
	}

	if v6.IsValid() {
		t.Fatalf("IPv6 = %v, want empty", v6)
	}
}

func TestWatchN6StreamReadsTheInterfaceOnEntry(t *testing.T) {
	svc := bgp.New(netip.Addr{}, netip.Addr{}, zap.NewNop())
	svc.SetListenPort(-1)

	stubInterfaceIPs(t, "192.0.2.10", "2001:db8::10")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// No update is ever sent and the backstop is a minute away, so only an
	// entry reconcile can populate the addresses.
	stopped := watchN6Stream(ctx, make(chan struct{}), make(chan netlink.AddrUpdate), "n6eth0", svc, nil)
	if !stopped {
		t.Fatal("watchN6Stream should report a clean stop when the context is cancelled")
	}

	v4, v6 := svc.N6Addresses()
	if v4.String() != "192.0.2.10" || v6.String() != "2001:db8::10" {
		t.Fatalf("N6Addresses = %v, %v; want 192.0.2.10, 2001:db8::10", v4, v6)
	}
}

func newTestDB(t *testing.T) *db.Database {
	t.Helper()

	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %s", err)
		}
	})

	return database
}

// Settings written before the router ID became mandatory hold an empty value.
// The first start that needs an identity adopts the N6 address and stores it,
// so later starts no longer depend on the interface.
func TestAdoptRouterIDFillsInAnEmptyRouterID(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)

	settings, err := database.GetBGPSettings(ctx)
	if err != nil {
		t.Fatalf("GetBGPSettings: %s", err)
	}

	if settings.RouterID != "" {
		t.Fatalf("seeded routerID = %q, want empty", settings.RouterID)
	}

	settings.Enabled = true

	if err := adoptRouterID(ctx, database, settings, netip.MustParseAddr("192.168.5.10")); err != nil {
		t.Fatalf("adoptRouterID: %s", err)
	}

	if settings.RouterID != "192.168.5.10" {
		t.Fatalf("in-memory routerID = %q, want 192.168.5.10", settings.RouterID)
	}

	stored, err := database.GetBGPSettings(ctx)
	if err != nil {
		t.Fatalf("GetBGPSettings: %s", err)
	}

	if stored.RouterID != "192.168.5.10" {
		t.Fatalf("stored routerID = %q, want 192.168.5.10", stored.RouterID)
	}
}

func TestAdoptRouterIDLeavesAConfiguredRouterIDAlone(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)

	settings := &db.BGPSettings{Enabled: true, LocalAS: 64512, RouterID: "10.9.9.9", ListenAddress: ":179"}
	if err := database.UpdateBGPSettings(ctx, settings); err != nil {
		t.Fatalf("UpdateBGPSettings: %s", err)
	}

	if err := adoptRouterID(ctx, database, settings, netip.MustParseAddr("192.168.5.10")); err != nil {
		t.Fatalf("adoptRouterID: %s", err)
	}

	stored, err := database.GetBGPSettings(ctx)
	if err != nil {
		t.Fatalf("GetBGPSettings: %s", err)
	}

	if stored.RouterID != "10.9.9.9" {
		t.Fatalf("stored routerID = %q, want 10.9.9.9", stored.RouterID)
	}
}

func TestAdoptRouterIDWithoutAnAddressToAdopt(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)

	settings, err := database.GetBGPSettings(ctx)
	if err != nil {
		t.Fatalf("GetBGPSettings: %s", err)
	}

	if err := adoptRouterID(ctx, database, settings, netip.Addr{}); err == nil {
		t.Fatal("adoptRouterID succeeded without an N6 address")
	}

	stored, err := database.GetBGPSettings(ctx)
	if err != nil {
		t.Fatalf("GetBGPSettings: %s", err)
	}

	if stored.RouterID != "" {
		t.Fatalf("stored routerID = %q, want empty", stored.RouterID)
	}
}

func TestReconcileN6AddressesDoesNotAnnounceAMissingIPv4(t *testing.T) {
	svc := bgp.New(netip.MustParseAddr("192.0.2.10"), netip.Addr{}, zap.NewNop())
	svc.SetListenPort(-1)

	stubInterfaceIPs(t, "", "")
	reconcileN6Addresses("n6eth0", svc, func(v4 netip.Addr) {
		t.Fatalf("callback ran with %v while the interface has no IPv4 address", v4)
	})
}

// A node that had no N6 address at startup has BGP enabled with no router ID.
// Once an address appears, the watcher's callback has to adopt it and store it,
// which is what publishes the settings topic and lets BGP start.
func TestAdoptRouterIDWhenMissingRecoversAfterStartup(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)

	enabled := &db.BGPSettings{Enabled: true, LocalAS: 64512, RouterID: "", ListenAddress: ":179"}
	if err := database.UpdateBGPSettings(ctx, enabled); err != nil {
		t.Fatalf("UpdateBGPSettings: %s", err)
	}

	adoptRouterIDWhenMissing(ctx, database, netip.MustParseAddr("192.168.5.10"))

	stored, err := database.GetBGPSettings(ctx)
	if err != nil {
		t.Fatalf("GetBGPSettings: %s", err)
	}

	if stored.RouterID != "192.168.5.10" {
		t.Fatalf("stored routerID = %q, want 192.168.5.10", stored.RouterID)
	}
}

func TestAdoptRouterIDWhenMissingIgnoresDisabledBGP(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)

	adoptRouterIDWhenMissing(ctx, database, netip.MustParseAddr("192.168.5.10"))

	stored, err := database.GetBGPSettings(ctx)
	if err != nil {
		t.Fatalf("GetBGPSettings: %s", err)
	}

	if stored.RouterID != "" {
		t.Fatalf("stored routerID = %q, want empty: a disabled speaker needs no identity", stored.RouterID)
	}
}
