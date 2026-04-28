package bgp_test

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/kernel"
	"go.uber.org/zap"
)

// fakeKernel records route operations for test assertions.
type fakeKernel struct {
	mu       sync.Mutex
	replaced []fakeRoute
	deleted  []fakeRoute
	listed   []netip.Prefix // routes returned by ListRoutesByPriority
}

type fakeRoute struct {
	destination string
	gateway     string
	priority    int
}

func (fk *fakeKernel) CreateRoute(dst netip.Prefix, gw netip.Addr, priority int, _ kernel.NetworkInterface) error {
	return nil
}

func (fk *fakeKernel) DeleteRoute(dst netip.Prefix, gw netip.Addr, priority int, _ kernel.NetworkInterface) error {
	fk.mu.Lock()
	defer fk.mu.Unlock()

	gwStr := ""
	if gw.IsValid() {
		gwStr = gw.String()
	}

	fk.deleted = append(fk.deleted, fakeRoute{
		destination: dst.String(),
		gateway:     gwStr,
		priority:    priority,
	})

	return nil
}

func (fk *fakeKernel) ReplaceRoute(dst netip.Prefix, gw netip.Addr, priority int, _ kernel.NetworkInterface) error {
	fk.mu.Lock()
	defer fk.mu.Unlock()

	fk.replaced = append(fk.replaced, fakeRoute{
		destination: dst.String(),
		gateway:     gw.String(),
		priority:    priority,
	})

	return nil
}

func (fk *fakeKernel) ListRoutesByPriority(priority int, _ kernel.NetworkInterface) ([]netip.Prefix, error) {
	fk.mu.Lock()
	defer fk.mu.Unlock()

	return fk.listed, nil
}

func (fk *fakeKernel) ListManagedRoutes(_ kernel.NetworkInterface) ([]kernel.ManagedRoute, error) {
	return nil, nil
}

func (fk *fakeKernel) InterfaceExists(_ kernel.NetworkInterface) (bool, error) { return true, nil }
func (fk *fakeKernel) RouteExists(_ netip.Prefix, _ netip.Addr, _ int, _ kernel.NetworkInterface) (bool, error) {
	return false, nil
}

func (fk *fakeKernel) EnableIPForwarding() error            { return nil }
func (fk *fakeKernel) IsIPForwardingEnabled() (bool, error) { return true, nil }
func (fk *fakeKernel) EnsureGatewaysOnInterfaceInNeighTable(_ kernel.NetworkInterface) error {
	return nil
}

// fakeImportStore returns configurable import prefix entries per peer.
type fakeImportStore struct {
	entries map[int][]bgp.ImportPrefixEntry
}

func (f *fakeImportStore) ListImportPrefixes(_ context.Context, peerID int) ([]bgp.ImportPrefixEntry, error) {
	return f.entries[peerID], nil
}

func TestGetLearnedRoutes_EmptyByDefault(t *testing.T) {
	svc := newTestServiceWithLearning(t, &fakeKernel{}, &fakeImportStore{})

	routes := svc.GetLearnedRoutes()
	if len(routes) != 0 {
		t.Fatalf("expected 0 learned routes, got %d", len(routes))
	}
}

func TestCleanStaleRoutes(t *testing.T) {
	n1 := netip.MustParsePrefix("0.0.0.0/0")
	n2 := netip.MustParsePrefix("10.100.0.0/16")

	fk := &fakeKernel{
		listed: []netip.Prefix{n1, n2},
	}

	svc := newTestServiceWithLearning(t, fk, &fakeImportStore{})
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	err := svc.Start(ctx, settings, nil, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// cleanStaleRoutes should have been called during Start, deleting the stale routes
	fk.mu.Lock()
	deletedCount := len(fk.deleted)
	fk.mu.Unlock()

	if deletedCount < 2 {
		t.Fatalf("expected at least 2 stale routes deleted, got %d", deletedCount)
	}
}

func TestStopRemovesLearnedRoutes(t *testing.T) {
	fk := &fakeKernel{}

	svc := newTestServiceWithLearning(t, fk, &fakeImportStore{})
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	err := svc.Start(ctx, settings, nil, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify service starts cleanly
	if !svc.IsRunning() {
		t.Fatal("expected service to be running")
	}

	err = svc.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// After stop, learned routes should be empty
	routes := svc.GetLearnedRoutes()
	if len(routes) != 0 {
		t.Fatalf("expected 0 learned routes after stop, got %d", len(routes))
	}
}

func TestRouteLearningDisabledWithoutDeps(t *testing.T) {
	// Service without learning dependencies should still work
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	err := svc.Start(ctx, settings, nil, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Should return empty (not panic)
	routes := svc.GetLearnedRoutes()
	if len(routes) != 0 {
		t.Fatalf("expected 0 learned routes without deps, got %d", len(routes))
	}
}

func newTestServiceWithLearning(t *testing.T, k kernel.Kernel, store bgp.ImportPrefixStore) *bgp.BGPService {
	t.Helper()

	n6Addr := netip.MustParseAddr("10.0.0.1")
	logger := zap.NewNop()

	filter := &bgp.RouteFilter{
		RejectPrefixes: bgp.BuildRejectPrefixes(nil),
	}

	svc := bgp.New(n6Addr, logger,
		bgp.WithKernel(k),
		bgp.WithImportPrefixStore(store),
		bgp.WithRouteFilter(filter),
	)
	svc.SetListenPort(-1)

	return svc
}

func TestReconfigurePeerRemovalCleansLearnedRoutes(t *testing.T) {
	fk := &fakeKernel{}

	store := &fakeImportStore{
		entries: map[int][]bgp.ImportPrefixEntry{
			1: {{Prefix: "0.0.0.0/0", MaxLength: 32}},
		},
	}

	svc := newTestServiceWithLearning(t, fk, store)
	ctx := context.Background()

	settings := bgp.BGPSettings{Enabled: true, LocalAS: 65000}
	peers := []bgp.BGPPeer{
		{ID: 1, Address: "192.168.1.1", RemoteAS: 65001, HoldTime: 90},
		{ID: 2, Address: "192.168.1.2", RemoteAS: 65002, HoldTime: 90},
	}

	err := svc.Start(ctx, settings, peers, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Inject learned routes from both peers.
	net1 := netip.MustParsePrefix("10.100.0.0/16")
	net2 := netip.MustParsePrefix("10.200.0.0/16")

	svc.InjectLearnedRouteForTest(net1, netip.MustParseAddr("192.168.1.1"), "192.168.1.1")
	svc.InjectLearnedRouteForTest(net2, netip.MustParseAddr("192.168.1.2"), "192.168.1.2")

	if len(svc.GetLearnedRoutes()) != 2 {
		t.Fatalf("expected 2 learned routes, got %d", len(svc.GetLearnedRoutes()))
	}

	// Remove peer 192.168.1.2 via reconfigure.
	remainingPeers := []bgp.BGPPeer{
		{ID: 1, Address: "192.168.1.1", RemoteAS: 65001, HoldTime: 90},
	}

	err = svc.Reconfigure(ctx, settings, remainingPeers)
	if err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	// Routes from the removed peer should be gone.
	routes := svc.GetLearnedRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 learned route after removing peer, got %d", len(routes))
	}

	if routes[0].Peer != "192.168.1.1" {
		t.Fatalf("expected remaining route from 192.168.1.1, got %s", routes[0].Peer)
	}

	// Verify kernel DeleteRoute was called for the removed route.
	fk.mu.Lock()
	deletedCount := len(fk.deleted)
	fk.mu.Unlock()

	if deletedCount < 1 {
		t.Fatalf("expected at least 1 kernel route deletion, got %d", deletedCount)
	}
}

func TestReconfigureImportPolicyChangeRemovesRoutes(t *testing.T) {
	fk := &fakeKernel{}

	// Start with "accept all" for peer 1.
	store := &fakeImportStore{
		entries: map[int][]bgp.ImportPrefixEntry{
			1: {{Prefix: "0.0.0.0/0", MaxLength: 32}},
		},
	}

	svc := newTestServiceWithLearning(t, fk, store)
	ctx := context.Background()

	settings := bgp.BGPSettings{Enabled: true, LocalAS: 65000}
	peers := []bgp.BGPPeer{
		{ID: 1, Address: "192.168.1.1", RemoteAS: 65001, HoldTime: 90},
	}

	err := svc.Start(ctx, settings, peers, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Inject a learned route.
	net1 := netip.MustParsePrefix("10.100.0.0/16")
	svc.InjectLearnedRouteForTest(net1, netip.MustParseAddr("192.168.1.1"), "192.168.1.1")

	if len(svc.GetLearnedRoutes()) != 1 {
		t.Fatalf("expected 1 learned route, got %d", len(svc.GetLearnedRoutes()))
	}

	// Change import policy to "accept nothing" (empty entries).
	store.entries = map[int][]bgp.ImportPrefixEntry{}

	err = svc.Reconfigure(ctx, settings, peers)
	if err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	// The route should be removed because the import policy now rejects it.
	routes := svc.GetLearnedRoutes()
	if len(routes) != 0 {
		t.Fatalf("expected 0 learned routes after policy change, got %d", len(routes))
	}
}

func TestSetAdvertisingToggle(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{Enabled: true, LocalAS: 65000}

	err := svc.Start(ctx, settings, nil, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Reconciler-style: populate the RIB via Announce.
	for _, ipStr := range []string{"10.1.1.1", "10.1.1.2"} {
		ip := netip.MustParseAddr(ipStr)
		if err := svc.Announce(ip, "imsi"); err != nil {
			t.Fatalf("Announce(%s) failed: %v", ipStr, err)
		}
	}

	if !svc.IsAdvertising() {
		t.Fatal("expected advertising=true after start")
	}

	routes, _ := svc.GetRoutes()
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}

	// Disable advertising (simulate NAT enabled): immediate withdraw, map cleared.
	svc.SetAdvertising(false)

	if svc.IsAdvertising() {
		t.Fatal("expected advertising=false after SetAdvertising(false)")
	}

	routes, _ = svc.GetRoutes()
	if len(routes) != 0 {
		t.Fatalf("expected 0 advertised routes after disabling, got %d", len(routes))
	}

	// Re-enable: flag flips but RIB stays empty. Reconciler repopulates
	// on its next tick (emulated here by explicit Announce calls).
	svc.SetAdvertising(true)

	if !svc.IsAdvertising() {
		t.Fatal("expected advertising=true after SetAdvertising(true)")
	}

	if got := len(svc.Paths()); got != 0 {
		t.Fatalf("expected empty RIB immediately after re-enable (reconciler fills it), got %d paths", got)
	}

	for _, ipStr := range []string{"10.1.1.1", "10.1.1.2"} {
		ip := netip.MustParseAddr(ipStr)
		if err := svc.Announce(ip, "imsi"); err != nil {
			t.Fatalf("Announce(%s) failed: %v", ipStr, err)
		}
	}

	routes, _ = svc.GetRoutes()
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes after re-enabling and reconcile, got %d", len(routes))
	}
}

func TestSetAdvertisingNoOpWhenNotRunning(t *testing.T) {
	svc := newTestService(t)

	// Should not panic or error.
	svc.SetAdvertising(true)
	svc.SetAdvertising(false)
}

func TestUpdateFilterRemovesNewlyRejectedRoutes(t *testing.T) {
	fk := &fakeKernel{}

	store := &fakeImportStore{
		entries: map[int][]bgp.ImportPrefixEntry{
			1: {{Prefix: "0.0.0.0/0", MaxLength: 32}},
		},
	}

	svc := newTestServiceWithLearning(t, fk, store)
	ctx := context.Background()

	settings := bgp.BGPSettings{Enabled: true, LocalAS: 65000}
	peers := []bgp.BGPPeer{
		{ID: 1, Address: "192.168.1.1", RemoteAS: 65001, HoldTime: 90},
	}

	err := svc.Start(ctx, settings, peers, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Inject a learned route in 10.45.0.0/16 (not yet rejected).
	net1 := netip.MustParsePrefix("10.45.0.5/32")
	svc.InjectLearnedRouteForTest(net1, netip.MustParseAddr("192.168.1.1"), "192.168.1.1")

	if len(svc.GetLearnedRoutes()) != 1 {
		t.Fatalf("expected 1 learned route, got %d", len(svc.GetLearnedRoutes()))
	}

	// Update the filter to reject 10.45.0.0/16 (new data network added).
	uePool := netip.MustParsePrefix("10.45.0.0/16")
	newFilter := &bgp.RouteFilter{
		RejectPrefixes: bgp.BuildRejectPrefixes([]netip.Prefix{uePool}),
	}

	svc.UpdateFilter(newFilter)

	// The route in 10.45.0.0/16 should now be rejected and removed.
	routes := svc.GetLearnedRoutes()
	if len(routes) != 0 {
		t.Fatalf("expected 0 learned routes after filter update, got %d", len(routes))
	}

	// Verify kernel route was deleted.
	fk.mu.Lock()
	deletedCount := len(fk.deleted)
	fk.mu.Unlock()

	if deletedCount < 1 {
		t.Fatalf("expected at least 1 kernel route deletion, got %d", deletedCount)
	}
}

func TestStopWithPollerDoesNotDeadlock(t *testing.T) {
	svc := newTestServiceWithLearning(t, &fakeKernel{}, &fakeImportStore{})
	ctx := context.Background()

	settings := bgp.BGPSettings{Enabled: true, LocalAS: 65000}

	err := svc.Start(ctx, settings, nil, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	done := make(chan error, 1)

	go func() {
		done <- svc.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop deadlocked — did not complete within 5 seconds")
	}

	if svc.IsRunning() {
		t.Fatal("expected service to not be running after Stop")
	}
}

func TestStartStopCyclesWithPoller(t *testing.T) {
	svc := newTestServiceWithLearning(t, &fakeKernel{}, &fakeImportStore{})
	ctx := context.Background()

	settings := bgp.BGPSettings{Enabled: true, LocalAS: 65000}

	for i := range 10 {
		err := svc.Start(ctx, settings, nil, true)
		if err != nil {
			t.Fatalf("Start cycle %d failed: %v", i, err)
		}

		err = svc.Stop()
		if err != nil {
			t.Fatalf("Stop cycle %d failed: %v", i, err)
		}
	}

	if svc.IsRunning() {
		t.Fatal("expected service to not be running after final Stop")
	}
}

func TestReconfigureRestartWithPollerDoesNotDeadlock(t *testing.T) {
	svc := newTestServiceWithLearning(t, &fakeKernel{}, &fakeImportStore{})
	ctx := context.Background()

	settings := bgp.BGPSettings{Enabled: true, LocalAS: 65000}

	err := svc.Start(ctx, settings, nil, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Change AS number → triggers full restart (stopLocked + startLocked).
	newSettings := bgp.BGPSettings{Enabled: true, LocalAS: 65001}

	done := make(chan error, 1)

	go func() {
		done <- svc.Reconfigure(ctx, newSettings, nil)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Reconfigure failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Reconfigure deadlocked — did not complete within 5 seconds")
	}

	if !svc.IsRunning() {
		t.Fatal("expected service to be running after reconfigure restart")
	}
}
