package bgp_test

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/kernel"
	"go.uber.org/zap"
)

// fakeKernel records route operations for test assertions.
type fakeKernel struct {
	mu       sync.Mutex
	replaced []fakeRoute
	deleted  []fakeRoute
	listed   []net.IPNet // routes returned by ListRoutesByPriority
}

type fakeRoute struct {
	destination string
	gateway     string
	priority    int
}

func (fk *fakeKernel) CreateRoute(dst *net.IPNet, gw net.IP, priority int, _ kernel.NetworkInterface) error {
	return nil
}

func (fk *fakeKernel) DeleteRoute(dst *net.IPNet, gw net.IP, priority int, _ kernel.NetworkInterface) error {
	fk.mu.Lock()
	defer fk.mu.Unlock()

	gwStr := ""
	if gw != nil {
		gwStr = gw.String()
	}

	fk.deleted = append(fk.deleted, fakeRoute{
		destination: dst.String(),
		gateway:     gwStr,
		priority:    priority,
	})

	return nil
}

func (fk *fakeKernel) ReplaceRoute(dst *net.IPNet, gw net.IP, priority int, _ kernel.NetworkInterface) error {
	fk.mu.Lock()
	defer fk.mu.Unlock()

	fk.replaced = append(fk.replaced, fakeRoute{
		destination: dst.String(),
		gateway:     gw.String(),
		priority:    priority,
	})

	return nil
}

func (fk *fakeKernel) ListRoutesByPriority(priority int, _ kernel.NetworkInterface) ([]net.IPNet, error) {
	fk.mu.Lock()
	defer fk.mu.Unlock()

	return fk.listed, nil
}

func (fk *fakeKernel) InterfaceExists(_ kernel.NetworkInterface) (bool, error) { return true, nil }
func (fk *fakeKernel) RouteExists(_ *net.IPNet, _ net.IP, _ int, _ kernel.NetworkInterface) (bool, error) {
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
	_, n1, _ := net.ParseCIDR("0.0.0.0/0")
	_, n2, _ := net.ParseCIDR("10.100.0.0/16")

	fk := &fakeKernel{
		listed: []net.IPNet{*n1, *n2},
	}

	svc := newTestServiceWithLearning(t, fk, &fakeImportStore{})
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	err := svc.Start(ctx, settings, nil, nil, true)
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

	err := svc.Start(ctx, settings, nil, nil, true)
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

	err := svc.Start(ctx, settings, nil, nil, true)
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

	n6Addr := net.ParseIP("10.0.0.1")
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

	err := svc.Start(ctx, settings, peers, nil, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Inject learned routes from both peers.
	_, net1, _ := net.ParseCIDR("10.100.0.0/16")
	_, net2, _ := net.ParseCIDR("10.200.0.0/16")

	svc.InjectLearnedRouteForTest(*net1, net.ParseIP("192.168.1.1"), "192.168.1.1")
	svc.InjectLearnedRouteForTest(*net2, net.ParseIP("192.168.1.2"), "192.168.1.2")

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

	err := svc.Start(ctx, settings, peers, nil, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Inject a learned route.
	_, net1, _ := net.ParseCIDR("10.100.0.0/16")
	svc.InjectLearnedRouteForTest(*net1, net.ParseIP("192.168.1.1"), "192.168.1.1")

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

	ips := map[string]string{"10.1.1.1": "imsi-001010000000001", "10.1.1.2": "imsi-001010000000002"}

	err := svc.Start(ctx, settings, nil, ips, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	if !svc.IsAdvertising() {
		t.Fatal("expected advertising=true after start")
	}

	routes, _ := svc.GetRoutes()
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}

	// Disable advertising (simulate NAT enabled).
	svc.SetAdvertising(false, nil)

	if svc.IsAdvertising() {
		t.Fatal("expected advertising=false after SetAdvertising(false)")
	}

	routes, _ = svc.GetRoutes()
	if len(routes) != 0 {
		t.Fatalf("expected 0 advertised routes after disabling, got %d", len(routes))
	}

	// Re-enable advertising (simulate NAT disabled — pass allocated IPs from DB).
	svc.SetAdvertising(true, ips)

	if !svc.IsAdvertising() {
		t.Fatal("expected advertising=true after SetAdvertising(true)")
	}

	routes, _ = svc.GetRoutes()
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes after re-enabling, got %d", len(routes))
	}
}

func TestSetAdvertisingNoOpWhenNotRunning(t *testing.T) {
	svc := newTestService(t)

	// Should not panic or error.
	svc.SetAdvertising(true, nil)
	svc.SetAdvertising(false, nil)
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

	err := svc.Start(ctx, settings, peers, nil, true)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Inject a learned route in 10.45.0.0/16 (not yet rejected).
	_, net1, _ := net.ParseCIDR("10.45.0.5/32")
	svc.InjectLearnedRouteForTest(*net1, net.ParseIP("192.168.1.1"), "192.168.1.1")

	if len(svc.GetLearnedRoutes()) != 1 {
		t.Fatalf("expected 1 learned route, got %d", len(svc.GetLearnedRoutes()))
	}

	// Update the filter to reject 10.45.0.0/16 (new data network added).
	_, uePool, _ := net.ParseCIDR("10.45.0.0/16")
	newFilter := &bgp.RouteFilter{
		RejectPrefixes: bgp.BuildRejectPrefixes([]*net.IPNet{uePool}),
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
