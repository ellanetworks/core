// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"context"
	"net/netip"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
)

// sameRoute reports whether two route identities refer to the same kernel
// route. Matching is family-normalised because that is what the kernel does:
// RealKernel narrows an IPv4-mapped prefix to true IPv4 before handing it to
// netlink, so "::ffff:0.0.0.0/0" and "0.0.0.0/0" address one and the same
// route.
func sameRoute(a kernel.ManagedRoute, dest netip.Prefix, gw netip.Addr, prio int) bool {
	return kernel.UnmapPrefix(a.Destination) == kernel.UnmapPrefix(dest) &&
		a.Gateway.Unmap() == gw.Unmap() &&
		a.Priority == prio
}

// recordingKernel captures route operations and exposes a configurable
// "managed routes" view so the reconciler's deletion pass can be
// exercised without netlink.
type recordingKernel struct {
	mu             sync.Mutex
	managed        map[kernel.NetworkInterface][]kernel.ManagedRoute
	created        []routeOp
	deleted        []routeOp
	forwardingOn   bool
	enableCalled   bool
	existsCheckErr error
}

type routeOp struct {
	dest     netip.Prefix
	gateway  netip.Addr
	priority int
	ifKey    kernel.NetworkInterface
}

func newRecordingKernel() *recordingKernel {
	return &recordingKernel{
		managed:      make(map[kernel.NetworkInterface][]kernel.ManagedRoute),
		forwardingOn: true,
	}
}

func (k *recordingKernel) seedManaged(ifKey kernel.NetworkInterface, routes ...kernel.ManagedRoute) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.managed[ifKey] = append(k.managed[ifKey], routes...)
}

func (k *recordingKernel) EnableIPForwarding() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.enableCalled = true
	k.forwardingOn = true

	return nil
}

func (k *recordingKernel) IsIPForwardingEnabled() (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.forwardingOn, nil
}

func (k *recordingKernel) CreateRoute(dest netip.Prefix, gw netip.Addr, prio int, ifKey kernel.NetworkInterface) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.created = append(k.created, routeOp{dest: dest, gateway: gw, priority: prio, ifKey: ifKey})
	k.managed[ifKey] = append(k.managed[ifKey], kernel.ManagedRoute{Destination: dest, Gateway: gw, Priority: prio})

	return nil
}

func (k *recordingKernel) DeleteRoute(dest netip.Prefix, gw netip.Addr, prio int, ifKey kernel.NetworkInterface) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.deleted = append(k.deleted, routeOp{dest: dest, gateway: gw, priority: prio, ifKey: ifKey})

	filtered := k.managed[ifKey][:0]

	for _, r := range k.managed[ifKey] {
		if sameRoute(r, dest, gw, prio) {
			continue
		}

		filtered = append(filtered, r)
	}

	k.managed[ifKey] = filtered

	return nil
}

func (k *recordingKernel) ReplaceRoute(_ netip.Prefix, _ netip.Addr, _ int, _ kernel.NetworkInterface) error {
	return nil
}

func (k *recordingKernel) ListManagedRoutes(ifKey kernel.NetworkInterface) ([]kernel.ManagedRoute, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	out := make([]kernel.ManagedRoute, len(k.managed[ifKey]))
	copy(out, k.managed[ifKey])

	return out, nil
}

func (k *recordingKernel) InterfaceExists(_ kernel.NetworkInterface) (bool, error) { return true, nil }

func (k *recordingKernel) RouteExists(dest netip.Prefix, gw netip.Addr, prio int, ifKey kernel.NetworkInterface) (bool, error) {
	if k.existsCheckErr != nil {
		return false, k.existsCheckErr
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	for _, r := range k.managed[ifKey] {
		if sameRoute(r, dest, gw, prio) {
			return true, nil
		}
	}

	return false, nil
}

func (k *recordingKernel) EnsureGatewaysOnInterfaceInNeighTable(_ kernel.NetworkInterface) error {
	return nil
}

func newReconcileTestDB(t *testing.T) *db.Database {
	t.Helper()

	tempDir := t.TempDir()

	dbInstance, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	t.Cleanup(func() {
		_ = dbInstance.Close()
	})

	return dbInstance
}

func TestReconcileKernelRouting_AddsMissingRoute(t *testing.T) {
	dbInstance := newReconcileTestDB(t)
	k := newRecordingKernel()

	if _, err := dbInstance.CreateRoute(context.Background(), &db.Route{
		Destination: "10.0.0.0/24",
		Gateway:     "192.168.1.1",
		Interface:   db.N6,
		Metric:      100,
	}); err != nil {
		t.Fatalf("seed route: %v", err)
	}

	if err := ReconcileKernelRouting(context.Background(), dbInstance, k); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if got := len(k.created); got != 1 {
		t.Fatalf("expected 1 CreateRoute, got %d (%v)", got, k.created)
	}

	if got := len(k.deleted); got != 0 {
		t.Fatalf("expected no DeleteRoute, got %d (%v)", got, k.deleted)
	}
}

func TestReconcileKernelRouting_RemovesStaleRoute(t *testing.T) {
	dbInstance := newReconcileTestDB(t)
	k := newRecordingKernel()

	stale := kernel.ManagedRoute{
		Destination: netip.MustParsePrefix("10.99.0.0/24"),
		Gateway:     netip.MustParseAddr("192.168.1.1"),
		Priority:    100,
	}
	k.seedManaged(kernel.N6, stale)

	if err := ReconcileKernelRouting(context.Background(), dbInstance, k); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if got := len(k.deleted); got != 1 {
		t.Fatalf("expected 1 DeleteRoute, got %d (%v)", got, k.deleted)
	}

	if k.deleted[0].dest != stale.Destination {
		t.Fatalf("expected delete of %s, got %s", stale.Destination, k.deleted[0].dest)
	}
}

func TestReconcileKernelRouting_PreservesBGPMetricRoutes(t *testing.T) {
	dbInstance := newReconcileTestDB(t)
	k := newRecordingKernel()

	bgpLearned := kernel.ManagedRoute{
		Destination: netip.MustParsePrefix("172.16.0.0/12"),
		Gateway:     netip.MustParseAddr("10.0.0.1"),
		Priority:    bgpRouteMetric,
	}
	k.seedManaged(kernel.N6, bgpLearned)

	if err := ReconcileKernelRouting(context.Background(), dbInstance, k); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if got := len(k.deleted); got != 0 {
		t.Fatalf("BGP-metric route must not be deleted, but DeleteRoute called %d times: %v", got, k.deleted)
	}
}

func TestReconcileKernelRouting_KeepsExistingRouteUntouched(t *testing.T) {
	dbInstance := newReconcileTestDB(t)
	k := newRecordingKernel()

	if _, err := dbInstance.CreateRoute(context.Background(), &db.Route{
		Destination: "10.0.0.0/24",
		Gateway:     "192.168.1.1",
		Interface:   db.N6,
		Metric:      100,
	}); err != nil {
		t.Fatalf("seed route: %v", err)
	}

	k.seedManaged(kernel.N6, kernel.ManagedRoute{
		Destination: netip.MustParsePrefix("10.0.0.0/24"),
		Gateway:     netip.MustParseAddr("192.168.1.1"),
		Priority:    100,
	})

	if err := ReconcileKernelRouting(context.Background(), dbInstance, k); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if got := len(k.created); got != 0 {
		t.Fatalf("CreateRoute should not be called when route already present, got %d (%v)", got, k.created)
	}

	if got := len(k.deleted); got != 0 {
		t.Fatalf("DeleteRoute should not be called when route is in DB, got %d (%v)", got, k.deleted)
	}
}

func TestReconcileKernelRouting_EnablesIPForwardingWhenOff(t *testing.T) {
	dbInstance := newReconcileTestDB(t)
	k := newRecordingKernel()
	k.forwardingOn = false

	if err := ReconcileKernelRouting(context.Background(), dbInstance, k); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if !k.enableCalled {
		t.Fatal("expected EnableIPForwarding to be called when forwarding was off")
	}
}

// TestReconcileKernelRouting_V4DefaultNotStale covers the IPv4 default route.
// netlink synthesises its Dst from the 16-byte IPv4-mapped net.IPv4zero, so an
// unnormalised listing reports "::ffff:0.0.0.0/0", which never matches the
// "0.0.0.0/0" parsed from the database. The live N6 route was then classified
// stale on every cycle and a delete attempted against it.
func TestReconcileKernelRouting_V4DefaultNotStale(t *testing.T) {
	v4MappedDefault := netip.PrefixFrom(netip.MustParseAddr("::ffff:0.0.0.0"), 0)

	for _, listed := range []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/0"), // normalised, as RealKernel now returns
		v4MappedDefault,                    // raw netlink spelling, belt and braces
	} {
		t.Run(listed.String(), func(t *testing.T) {
			dbInstance := newReconcileTestDB(t)
			k := newRecordingKernel()

			if _, err := dbInstance.CreateRoute(context.Background(), &db.Route{
				Destination: "0.0.0.0/0",
				Gateway:     "10.0.20.129",
				Interface:   db.N6,
				Metric:      100,
			}); err != nil {
				t.Fatalf("seed route: %v", err)
			}

			k.seedManaged(kernel.N6, kernel.ManagedRoute{
				Destination: listed,
				Gateway:     netip.MustParseAddr("10.0.20.129"),
				Priority:    100,
			})

			if err := ReconcileKernelRouting(context.Background(), dbInstance, k); err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			if got := len(k.deleted); got != 0 {
				t.Errorf("live default route must not be deleted, but DeleteRoute called %d times: %v", got, k.deleted)
			}

			if got := len(k.created); got != 0 {
				t.Errorf("live default route must not be re-created, but CreateRoute called %d times: %v", got, k.created)
			}
		})
	}
}

// TestReconcileKernelRouting_V4MappedDBDestination is the same mismatch from
// the other side: netip.ParsePrefix accepts "::ffff:0.0.0.0/0", so an operator
// can store a destination that no kernel listing will ever equal.
func TestReconcileKernelRouting_V4MappedDBDestination(t *testing.T) {
	dbInstance := newReconcileTestDB(t)
	k := newRecordingKernel()

	if _, err := dbInstance.CreateRoute(context.Background(), &db.Route{
		Destination: "::ffff:0.0.0.0/0",
		Gateway:     "::ffff:10.0.20.129",
		Interface:   db.N6,
		Metric:      100,
	}); err != nil {
		t.Fatalf("seed route: %v", err)
	}

	k.seedManaged(kernel.N6, kernel.ManagedRoute{
		Destination: netip.MustParsePrefix("0.0.0.0/0"),
		Gateway:     netip.MustParseAddr("10.0.20.129"),
		Priority:    100,
	})

	if err := ReconcileKernelRouting(context.Background(), dbInstance, k); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if got := len(k.deleted); got != 0 {
		t.Errorf("expected no DeleteRoute, got %d (%v)", got, k.deleted)
	}

	if got := len(k.created); got != 0 {
		t.Errorf("expected no CreateRoute, got %d (%v)", got, k.created)
	}
}
