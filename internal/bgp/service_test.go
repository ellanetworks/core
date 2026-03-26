package bgp_test

import (
	"context"
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/bgp"
	"go.uber.org/zap"
)

func newTestService(t *testing.T) *bgp.BGPService {
	t.Helper()

	n6Addr := net.ParseIP("10.0.0.1")
	logger := zap.NewNop()
	svc := bgp.New(n6Addr, logger)
	// Use ListenPort -1 to avoid binding to port 179 in tests
	svc.SetListenPort(-1)

	return svc
}

func TestNew(t *testing.T) {
	svc := newTestService(t)
	if svc.IsRunning() {
		t.Fatal("new service should not be running")
	}
}

func TestStartStop(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	err := svc.Start(ctx, settings, nil, nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !svc.IsRunning() {
		t.Fatal("service should be running after Start")
	}

	err = svc.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if svc.IsRunning() {
		t.Fatal("service should not be running after Stop")
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	err := svc.Start(ctx, settings, nil, nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	err = svc.Start(ctx, settings, nil, nil)
	if err == nil {
		t.Fatal("expected error when starting already running service")
	}
}

func TestStopWhenNotRunning(t *testing.T) {
	svc := newTestService(t)

	err := svc.Stop()
	if err != nil {
		t.Fatalf("Stop on non-running service should succeed, got: %v", err)
	}
}

func TestAnnounceWithdraw(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	err := svc.Start(ctx, settings, nil, nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	ip := net.ParseIP("10.1.1.1")

	err = svc.Announce(ip, "001010000000001")
	if err != nil {
		t.Fatalf("Announce failed: %v", err)
	}

	routes, err := svc.GetRoutes()
	if err != nil {
		t.Fatalf("GetRoutes failed: %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	if routes[0].Prefix != "10.1.1.1/32" {
		t.Fatalf("expected prefix 10.1.1.1/32, got %s", routes[0].Prefix)
	}

	if routes[0].NextHop != "10.0.0.1" {
		t.Fatalf("expected next-hop 10.0.0.1, got %s", routes[0].NextHop)
	}

	if routes[0].Subscriber != "001010000000001" {
		t.Fatalf("expected subscriber 001010000000001, got %s", routes[0].Subscriber)
	}

	err = svc.Withdraw(ip)
	if err != nil {
		t.Fatalf("Withdraw failed: %v", err)
	}

	routes, err = svc.GetRoutes()
	if err != nil {
		t.Fatalf("GetRoutes failed: %v", err)
	}

	if len(routes) != 0 {
		t.Fatalf("expected 0 routes after withdraw, got %d", len(routes))
	}
}

func TestAnnounceWhenNotRunning(t *testing.T) {
	svc := newTestService(t)

	// Should be a no-op, not an error
	err := svc.Announce(net.ParseIP("10.1.1.1"), "001010000000001")
	if err != nil {
		t.Fatalf("Announce on non-running service should succeed (no-op), got: %v", err)
	}

	err = svc.Withdraw(net.ParseIP("10.1.1.1"))
	if err != nil {
		t.Fatalf("Withdraw on non-running service should succeed (no-op), got: %v", err)
	}
}

func TestStartWithInitialRoutes(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	allocatedIPs := []net.IP{
		net.ParseIP("10.1.1.1"),
		net.ParseIP("10.1.1.2"),
		net.ParseIP("10.1.1.3"),
	}

	err := svc.Start(ctx, settings, nil, allocatedIPs)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	routes, err := svc.GetRoutes()
	if err != nil {
		t.Fatalf("GetRoutes failed: %v", err)
	}

	if len(routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(routes))
	}
}

func TestStartWithPeers(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	peers := []bgp.BGPPeer{
		{
			Address:  "192.168.1.1",
			RemoteAS: 65001,
			HoldTime: 90,
		},
	}

	err := svc.Start(ctx, settings, peers, nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	status, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if len(status.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(status.Peers))
	}

	if status.Peers[0].Address != "192.168.1.1" {
		t.Fatalf("expected peer address 192.168.1.1, got %s", status.Peers[0].Address)
	}

	if status.Peers[0].RemoteAS != 65001 {
		t.Fatalf("expected peer remote AS 65001, got %d", status.Peers[0].RemoteAS)
	}
}

func TestGetStatusStopped(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	status, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if len(status.Peers) != 0 {
		t.Fatalf("expected no peers when stopped, got %d", len(status.Peers))
	}
}

func TestGetRoutesWhenNotRunning(t *testing.T) {
	svc := newTestService(t)

	routes, err := svc.GetRoutes()
	if err != nil {
		t.Fatalf("GetRoutes failed: %v", err)
	}

	if routes != nil {
		t.Fatalf("expected nil routes when not running, got %v", routes)
	}
}

func TestReconfigureHotPeerAdd(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	// Start with one peer
	peers := []bgp.BGPPeer{
		{Address: "192.168.1.1", RemoteAS: 65001, HoldTime: 90},
	}

	err := svc.Start(ctx, settings, peers, nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Reconfigure with two peers (same AS/RouterID so no restart)
	newPeers := []bgp.BGPPeer{
		{Address: "192.168.1.1", RemoteAS: 65001, HoldTime: 90},
		{Address: "192.168.1.2", RemoteAS: 65002, HoldTime: 90},
	}

	err = svc.Reconfigure(ctx, settings, newPeers)
	if err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	status, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if len(status.Peers) != 2 {
		t.Fatalf("expected 2 peers after reconfigure, got %d", len(status.Peers))
	}
}

func TestReconfigureHotPeerRemove(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	peers := []bgp.BGPPeer{
		{Address: "192.168.1.1", RemoteAS: 65001, HoldTime: 90},
		{Address: "192.168.1.2", RemoteAS: 65002, HoldTime: 90},
	}

	err := svc.Start(ctx, settings, peers, nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Remove one peer
	newPeers := []bgp.BGPPeer{
		{Address: "192.168.1.1", RemoteAS: 65001, HoldTime: 90},
	}

	err = svc.Reconfigure(ctx, settings, newPeers)
	if err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	status, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if len(status.Peers) != 1 {
		t.Fatalf("expected 1 peer after reconfigure, got %d", len(status.Peers))
	}
}

func TestReconfigureWithRestart(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	allocatedIPs := []net.IP{
		net.ParseIP("10.1.1.1"),
	}

	err := svc.Start(ctx, settings, nil, allocatedIPs)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Change AS number → triggers full restart
	newSettings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65001,
	}

	err = svc.Reconfigure(ctx, newSettings, nil)
	if err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	if !svc.IsRunning() {
		t.Fatal("service should still be running after restart reconfigure")
	}

	// Routes should be re-announced after restart
	routes, err := svc.GetRoutes()
	if err != nil {
		t.Fatalf("GetRoutes failed: %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf("expected 1 route after restart, got %d", len(routes))
	}
}

func TestReconfigureWhenNotRunning(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	// Should be a no-op
	err := svc.Reconfigure(ctx, settings, nil)
	if err != nil {
		t.Fatalf("Reconfigure on non-running service should succeed (no-op), got: %v", err)
	}
}

func TestAnnounceIPv6Rejected(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	err := svc.Start(ctx, settings, nil, nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	err = svc.Announce(net.ParseIP("::1"), "test")
	if err == nil {
		t.Fatal("expected error for IPv6 address")
	}
}

func TestReconfigureHotPeerPropertyChange(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	// Start with one peer
	peers := []bgp.BGPPeer{
		{Address: "192.168.1.1", RemoteAS: 65001, HoldTime: 90},
	}

	err := svc.Start(ctx, settings, peers, nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	// Reconfigure with same address but different remoteAS and holdTime
	newPeers := []bgp.BGPPeer{
		{Address: "192.168.1.1", RemoteAS: 65099, HoldTime: 30},
	}

	err = svc.Reconfigure(ctx, settings, newPeers)
	if err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	status, err := svc.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if len(status.Peers) != 1 {
		t.Fatalf("expected 1 peer after reconfigure, got %d", len(status.Peers))
	}

	if status.Peers[0].RemoteAS != 65099 {
		t.Fatalf("expected peer remote AS 65099 after property change, got %d", status.Peers[0].RemoteAS)
	}
}

func TestMultipleAnnounceWithdraw(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	settings := bgp.BGPSettings{
		Enabled: true,
		LocalAS: 65000,
	}

	err := svc.Start(ctx, settings, nil, nil)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() { _ = svc.Stop() }()

	ips := []net.IP{
		net.ParseIP("10.1.1.1"),
		net.ParseIP("10.1.1.2"),
		net.ParseIP("10.1.1.3"),
	}

	for _, ip := range ips {
		if err := svc.Announce(ip, "test-owner"); err != nil {
			t.Fatalf("Announce %s failed: %v", ip, err)
		}
	}

	routes, err := svc.GetRoutes()
	if err != nil {
		t.Fatalf("GetRoutes failed: %v", err)
	}

	if len(routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(routes))
	}

	// Withdraw one
	if err := svc.Withdraw(ips[1]); err != nil {
		t.Fatalf("Withdraw failed: %v", err)
	}

	routes, err = svc.GetRoutes()
	if err != nil {
		t.Fatalf("GetRoutes failed: %v", err)
	}

	if len(routes) != 2 {
		t.Fatalf("expected 2 routes after withdraw, got %d", len(routes))
	}
}
