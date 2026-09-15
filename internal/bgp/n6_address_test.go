// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package bgp_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/bgp"
	"go.uber.org/zap"
)

// startedService returns a running speaker with a configured router ID.
func startedService(t *testing.T) *bgp.BGPService {
	t.Helper()

	svc := newTestService(t)

	err := svc.Start(context.Background(), bgp.BGPSettings{LocalAS: 65000, RouterID: "10.9.9.9"}, nil, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	t.Cleanup(func() { _ = svc.Stop() })

	return svc
}

func routesByPrefix(t *testing.T, svc *bgp.BGPService) map[string]string {
	t.Helper()

	routes, err := svc.GetRoutes()
	if err != nil {
		t.Fatalf("GetRoutes failed: %v", err)
	}

	out := make(map[string]string, len(routes))
	for _, r := range routes {
		out[r.Prefix] = r.NextHop
	}

	return out
}

// The router ID is not derived from anything at start.
func TestStartWithoutRouterIDFails(t *testing.T) {
	svc := newTestService(t)

	err := svc.Start(context.Background(), bgp.BGPSettings{LocalAS: 65000}, nil, true)
	if err == nil {
		t.Cleanup(func() { _ = svc.Stop() })
		t.Fatal("Start succeeded without a router ID")
	}

	if svc.IsRunning() {
		t.Fatal("service reports running after a failed start")
	}
}

func TestStartWithoutN6AddressSucceedsWithRouterID(t *testing.T) {
	svc := bgp.New(netip.Addr{}, netip.Addr{}, zap.NewNop())
	svc.SetListenPort(-1)

	// A stored router ID makes the speaker's identity independent of the
	// interface, so BGP comes up even before N6 has an address.
	err := svc.Start(context.Background(), bgp.BGPSettings{LocalAS: 65000, RouterID: "10.9.9.9"}, nil, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	t.Cleanup(func() { _ = svc.Stop() })

	if !svc.IsRunning() {
		t.Fatal("speaker should be running")
	}
}

func TestUpdateN6AddressesWhileStoppedCachesTheAddresses(t *testing.T) {
	svc := bgp.New(netip.Addr{}, netip.Addr{}, zap.NewNop())
	svc.SetListenPort(-1)

	v4 := netip.MustParseAddr("192.0.2.10")
	v6 := netip.MustParseAddr("2001:db8::10")

	if err := svc.UpdateN6Addresses(v4, v6); err != nil {
		t.Fatalf("UpdateN6Addresses failed: %v", err)
	}

	gotV4, gotV6 := svc.N6Addresses()
	if gotV4 != v4 || gotV6 != v6 {
		t.Fatalf("N6Addresses = %v, %v; want %v, %v", gotV4, gotV6, v4, v6)
	}
}

// An N6 address change moves the next-hop of the advertised routes and nothing
// else: the identity is stored, so the sessions must survive untouched.
func TestUpdateN6AddressesMovesNextHopWithoutRestarting(t *testing.T) {
	svc := startedService(t)

	prefix := netip.MustParsePrefix("10.45.0.2/32")
	if err := svc.Announce(prefix, "imsi-001"); err != nil {
		t.Fatalf("Announce failed: %v", err)
	}

	newV4 := netip.MustParseAddr("192.0.2.99")
	_, v6 := svc.N6Addresses()

	if err := svc.UpdateN6Addresses(newV4, v6); err != nil {
		t.Fatalf("UpdateN6Addresses failed: %v", err)
	}

	if !svc.IsRunning() {
		t.Fatal("speaker should still be running after an N6 address change")
	}

	if got, want := routesByPrefix(t, svc)[prefix.String()], newV4.String(); got != want {
		t.Fatalf("next-hop for %s = %q, want %q", prefix, got, want)
	}
}

// Losing an address leaves the speaker with no next-hop for that family. The
// affected routes have to leave the RIB — keeping them advertised points peers
// at an address this node no longer owns — while the other family is untouched.
func TestUpdateN6AddressesWithdrawsOnlyTheFamilyThatLostItsNextHop(t *testing.T) {
	svc := startedService(t)

	v4Prefix := netip.MustParsePrefix("10.45.0.2/32")
	v6Prefix := netip.MustParsePrefix("2001:db8::/64")

	if err := svc.Announce(v4Prefix, "imsi-v4"); err != nil {
		t.Fatalf("Announce IPv4 failed: %v", err)
	}

	if err := svc.Announce(v6Prefix, "imsi-v6"); err != nil {
		t.Fatalf("Announce IPv6 failed: %v", err)
	}

	origV4, origV6 := svc.N6Addresses()

	if err := svc.UpdateN6Addresses(origV4, netip.Addr{}); err != nil {
		t.Fatalf("UpdateN6Addresses failed: %v", err)
	}

	routes := routesByPrefix(t, svc)

	if _, ok := routes[v6Prefix.String()]; ok {
		t.Fatalf("%s is still advertised after its next-hop disappeared", v6Prefix)
	}

	if got, want := routes[v4Prefix.String()], origV4.String(); got != want {
		t.Fatalf("next-hop for %s = %q, want %q", v4Prefix, got, want)
	}

	// The lease is unchanged, so the path stays desired and the reconciler
	// must not see it as withdrawn.
	if _, ok := svc.Paths()[v6Prefix.String()]; !ok {
		t.Fatalf("%s was dropped from the paths map; it should be re-announced when an address returns", v6Prefix)
	}

	if err := svc.UpdateN6Addresses(origV4, origV6); err != nil {
		t.Fatalf("UpdateN6Addresses failed: %v", err)
	}

	if got, want := routesByPrefix(t, svc)[v6Prefix.String()], origV6.String(); got != want {
		t.Fatalf("next-hop for %s after the address returned = %q, want %q", v6Prefix, got, want)
	}
}

// Losing the IPv4 address costs the speaker its IPv4 next-hop but not its
// identity, so the speaker keeps running and only the IPv4 routes go away.
func TestUpdateN6AddressesKeepsRunningWhenIPv4Disappears(t *testing.T) {
	svc := startedService(t)

	prefix := netip.MustParsePrefix("10.45.0.2/32")
	if err := svc.Announce(prefix, "imsi-001"); err != nil {
		t.Fatalf("Announce failed: %v", err)
	}

	_, v6 := svc.N6Addresses()

	if err := svc.UpdateN6Addresses(netip.Addr{}, v6); err != nil {
		t.Fatalf("UpdateN6Addresses failed: %v", err)
	}

	if !svc.IsRunning() {
		t.Fatal("speaker should still be running: its identity does not depend on the N6 address")
	}

	if _, ok := routesByPrefix(t, svc)[prefix.String()]; ok {
		t.Fatalf("%s is still advertised without an IPv4 next-hop", prefix)
	}
}

// The N6 addresses became mutable when the watcher started tracking the
// interface, so every reader has to hold the lock. Run with -race.
func TestN6AddressReadersAreRaceFreeWhileUpdating(t *testing.T) {
	svc := startedService(t)

	done := make(chan struct{})

	go func() {
		defer close(done)

		for i := range 50 {
			v4 := netip.AddrFrom4([4]byte{192, 0, 2, byte(i)})
			if err := svc.UpdateN6Addresses(v4, netip.Addr{}); err != nil {
				t.Errorf("UpdateN6Addresses failed: %v", err)

				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		default:
			svc.N6Addresses()
			svc.IsRunning()
		}
	}
}
