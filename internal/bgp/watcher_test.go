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
		RejectPrefixes: bgp.BuildRejectPrefixes(nil, nil),
	}

	svc := bgp.New(n6Addr, logger,
		bgp.WithKernel(k),
		bgp.WithImportPrefixStore(store),
		bgp.WithRouteFilter(filter),
	)
	svc.SetListenPort(-1)

	return svc
}
