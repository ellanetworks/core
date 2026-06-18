// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

const (
	bgpComposeDir = "compose/bgp/"
	bgpPeerAS     = 65001

	bgpEllaCoreBGPAS      = 64512
	bgpEllaCoreListenAddr = ":179"
)

func bgpComposeFile() string {
	if DetectIPFamily() == IPv6Only {
		return "compose-ipv6.yaml"
	}

	return "compose.yaml"
}

func bgpGoBGPPeerAddress() string {
	if DetectIPFamily() == IPv6Only {
		return "fd00:6::4"
	}

	return "10.6.0.4"
}

func bgpEllaCoreN6IP() string {
	if DetectIPFamily() == IPv6Only {
		return "fd00:6::2"
	}

	return "10.6.0.2"
}

func bgpEllaCoreN2IP() string {
	if DetectIPFamily() == IPv6Only {
		return "2001:db8:1::10"
	}

	return "10.3.0.2"
}

func bgpRouterN6IP() string {
	if DetectIPFamily() == IPv6Only {
		return "fd00:6::3"
	}

	return "10.6.0.3"
}

func bgpTesterN3IP() string {
	if DetectIPFamily() == IPv6Only {
		return "2001:db8:3::11"
	}

	return "10.3.0.3"
}

func bgpTesterN3SecondaryIP() string {
	if DetectIPFamily() == IPv6Only {
		return "2001:db8:3::13"
	}

	return "10.3.0.4"
}

func bgpTesterDefaultIP() string {
	if DetectIPFamily() == IPv6Only {
		return "2001:db8:1::11"
	}

	return "10.3.0.3"
}

func bgpAPIAddress() string {
	addr := bgpEllaCoreN2IP()
	if DetectIPFamily() == IPv6Only {
		addr = "[" + addr + "]"
	}

	return fmt.Sprintf("http://%s:5002", addr)
}

func bgpDefaultRoute() (destination, gateway string) {
	if DetectIPFamily() == IPv6Only {
		return "2001:4860:4860::8888/128", bgpRouterN6IP()
	}

	return "8.8.8.8/32", bgpRouterN6IP()
}

func bgpUEPool() string {
	if DetectIPFamily() == IPv6Only {
		return "fd45::/48"
	}

	return "10.45.0.0/16"
}

func bgpAddressFamily() string {
	if DetectIPFamily() == IPv6Only {
		return "ipv6"
	}

	return "ipv4"
}

type bgpTestPrefixes struct {
	learnable   string
	filtered    string
	benign      string
	dangerous   []string
	importAllow client.BGPImportPrefix
	importAll   client.BGPImportPrefix
}

func bgpPrefixes() bgpTestPrefixes {
	if DetectIPFamily() == IPv6Only {
		return bgpTestPrefixes{
			learnable:   "2001:db8:100::/48",
			filtered:    "2001:db8:200::/48",
			benign:      "2001:db8:300::/48",
			dangerous:   []string{"fe80::/64", "ff02::/64", "::1/128"},
			importAllow: client.BGPImportPrefix{Prefix: "2001:db8:100::/48", MaxLength: 64},
			importAll:   client.BGPImportPrefix{Prefix: "::/0", MaxLength: 128},
		}
	}

	return bgpTestPrefixes{
		learnable:   "192.168.100.0/24",
		filtered:    "172.16.50.0/24",
		benign:      "192.168.200.0/24",
		dangerous:   []string{"169.254.1.0/24", "224.0.0.0/24", "127.0.0.1/32"},
		importAllow: client.BGPImportPrefix{Prefix: "192.168.100.0/24", MaxLength: 32},
		importAll:   client.BGPImportPrefix{Prefix: "0.0.0.0/0", MaxLength: 32},
	}
}

func TestIntegrationBGP(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	ctx := context.Background()
	family := DetectIPFamily()
	prefixes := bgpPrefixes()
	af := bgpAddressFamily()
	peerAddr := bgpGoBGPPeerAddress()
	n6IP := bgpEllaCoreN6IP()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	dc.ComposeCleanup(ctx)

	composeFile := bgpComposeFile()

	if err := dc.ComposeUpWithFile(ctx, bgpComposeDir, composeFile); err != nil {
		t.Fatalf("compose up: %v", err)
	}

	t.Cleanup(func() {
		for _, svc := range []string{"ella-core", "gobgp"} {
			logs, err := dc.ComposeLogs(ctx, bgpComposeDir, svc)
			if err == nil && t.Failed() {
				t.Logf("=== %s container logs ===\n%s", svc, logs)
			}
		}

		dc.ComposeDownWithFile(ctx, bgpComposeDir, composeFile)
	})

	cl, err := client.New(&client.Config{BaseURL: bgpAPIAddress()})
	if err != nil {
		t.Fatalf("ella client: %v", err)
	}

	if err := waitForEllaCoreReady(ctx, cl); err != nil {
		t.Fatalf("wait for ella core: %v", err)
	}

	if err := cl.Initialize(ctx, &client.InitializeOptions{
		Email:    "admin@ellanetworks.com",
		Password: "admin",
	}); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	resp, err := cl.CreateMyAPIToken(ctx, &client.CreateAPITokenOptions{
		Name: "bgp-integration-test-token",
	})
	if err != nil {
		t.Fatalf("create API token: %v", err)
	}

	cl.SetToken(resp.Token)

	if err := cl.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: false}); err != nil {
		t.Fatalf("disable NAT: %v", err)
	}

	routeDst, routeGw := bgpDefaultRoute()

	if err := cl.CreateRoute(ctx, &client.CreateRouteOptions{
		Destination: routeDst,
		Gateway:     routeGw,
		Interface:   "n6",
		Metric:      0,
	}); err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("create route: %v", err)
	}

	baseline := fixture.New(t, ctx, cl)
	baseline.OperatorDefault()
	baseline.Profile(fixture.DefaultProfileSpec())
	baseline.Slice(fixture.DefaultSliceSpec())
	baseline.DataNetwork(fixture.DefaultDataNetworkSpec())
	baseline.Policy(fixture.DefaultPolicySpec())

	routerID := bgpEllaCoreN6IP()
	if family == IPv6Only {
		routerID = "10.6.0.99"
	}

	if err := cl.UpdateBGPSettings(ctx, &client.UpdateBGPSettingsOptions{
		Enabled:       true,
		LocalAS:       bgpEllaCoreBGPAS,
		RouterID:      routerID,
		ListenAddress: bgpEllaCoreListenAddr,
	}); err != nil {
		t.Fatalf("enable BGP: %v", err)
	}

	if err := cl.CreateBGPPeer(ctx, &client.CreateBGPPeerOptions{
		Address:  peerAddr,
		RemoteAS: bgpPeerAS,
		HoldTime: 90,
		ImportPrefixes: []client.BGPImportPrefix{
			prefixes.importAllow,
		},
	}); err != nil {
		t.Fatalf("create BGP peer: %v", err)
	}

	gobgpContainer, err := dc.ResolveComposeContainer(ctx, "bgp", "gobgp")
	if err != nil {
		t.Fatalf("resolve gobgp container: %v", err)
	}

	testerContainer, err := dc.ResolveComposeContainer(ctx, "bgp", "ella-core-tester")
	if err != nil {
		t.Fatalf("resolve tester container: %v", err)
	}

	waitForGoBGPReady(ctx, t, dc, gobgpContainer, 60*time.Second)
	waitForBGPSession(ctx, t, cl, peerAddr, 60*time.Second)

	// --- Subtests ---

	t.Run("SessionEstablishment", func(t *testing.T) {
		peers, err := cl.ListBGPPeers(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list peers: %v", err)
		}

		peer := findPeerByAddress(peers.Items, peerAddr)
		if peer == nil {
			t.Fatalf("peer %s not found", peerAddr)
		}

		if peer.State != "established" {
			t.Fatalf("peer state: got %q, want %q", peer.State, "established")
		}

		if peer.Uptime == "" {
			t.Fatalf("peer uptime is empty")
		}

		out, err := gobgpCmd(ctx, dc, gobgpContainer, "neighbor")
		if err != nil {
			t.Fatalf("gobgp neighbor: %v", err)
		}

		Assert(t, strings.Contains(out, n6IP), fmt.Sprintf("gobgp neighbor does not contain %s:\n%s", n6IP, out))
	})

	t.Run("RouteAdvertisement", func(t *testing.T) {
		sc, _ := scenarios.Get("ue/session_hold")
		spec := sc.Fixture(scenarios.Env{})

		fx := fixture.New(t, ctx, cl)
		fx.Apply(spec)

		n2Addr := bgpEllaCoreN2IP()
		n3Addr := bgpTesterN3IP()
		n3Sec := bgpTesterN3SecondaryIP()

		argv := []string{
			"core-tester", "run", "ue/session_hold",
			"--ella-core-n2-address", net.JoinHostPort(n2Addr, "38412"),
			"--gnb", fmt.Sprintf("gnb1,n2=%s,n3=%s,n3-secondary=%s", bgpTesterDefaultIP(), n3Addr, n3Sec),
			"--verbose",
		}

		if family == IPv6Only {
			argv = append(argv, "--ip-version", "ipv6")
		}

		if _, err := dc.Exec(ctx, testerContainer, argv, true, 10*time.Second, nil); err != nil {
			t.Fatalf("start session_hold scenario: %v", err)
		}

		t.Cleanup(func() {
			_, _ = dc.Exec(ctx, testerContainer, []string{"pkill", "-f", "ue/session_hold"}, true, 5*time.Second, nil)
		})

		advertised := waitForAdvertisedRouteInPool(ctx, t, cl, bgpUEPool(), 60*time.Second)
		t.Logf("advertised UE route: %s (subscriber %s, next-hop %s)", advertised.Prefix, advertised.Subscriber, advertised.NextHop)

		out, err := gobgpCmd(ctx, dc, gobgpContainer, "global", "rib", "-a", af)
		if err != nil {
			t.Fatalf("gobgp global rib: %v", err)
		}

		uePrefix := advertised.Prefix
		if strings.Contains(uePrefix, "/") {
			uePrefix = strings.Split(uePrefix, "/")[0]
		}

		Assert(t, strings.Contains(out, uePrefix), fmt.Sprintf("gobgp did not receive UE route %s:\n%s", advertised.Prefix, out))

		peers, err := cl.ListBGPPeers(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list peers: %v", err)
		}

		peer := findPeerByAddress(peers.Items, peerAddr)
		if peer == nil {
			t.Fatalf("peer %s not found", peerAddr)
		}

		if peer.PrefixesSent < 1 {
			t.Fatalf("PrefixesSent: got %d, want >= 1", peer.PrefixesSent)
		}
	})

	t.Run("RouteLearning", func(t *testing.T) {
		if _, err := gobgpCmd(ctx, dc, gobgpContainer,
			"global", "rib", "add", prefixes.learnable, "-a", af,
		); err != nil {
			t.Fatalf("gobgp announce route: %v", err)
		}

		t.Cleanup(func() {
			_, _ = gobgpCmd(ctx, dc, gobgpContainer,
				"global", "rib", "del", prefixes.learnable, "-a", af,
			)
		})

		waitForLearnedRoute(ctx, t, cl, prefixes.learnable)

		routes, err := cl.GetBGPLearnedRoutes(ctx)
		if err != nil {
			t.Fatalf("get learned routes: %v", err)
		}

		found := findLearnedRoute(routes.Routes, prefixes.learnable)
		if found == nil {
			t.Fatalf("learned route %s not found", prefixes.learnable)
		}

		if found.NextHop != peerAddr {
			t.Fatalf("NextHop: got %q, want %q", found.NextHop, peerAddr)
		}

		if found.Peer != peerAddr {
			t.Fatalf("Peer: got %q, want %q", found.Peer, peerAddr)
		}

		peers, err := cl.ListBGPPeers(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list peers: %v", err)
		}

		peer := findPeerByAddress(peers.Items, peerAddr)
		if peer == nil {
			t.Fatalf("peer not found")
		}

		if peer.PrefixesReceived < 1 {
			t.Fatalf("PrefixesReceived: got %d, want >= 1", peer.PrefixesReceived)
		}
	})

	t.Run("ImportPrefixFiltering", func(t *testing.T) {
		if _, err := gobgpCmd(ctx, dc, gobgpContainer,
			"global", "rib", "add", prefixes.filtered, "-a", af,
		); err != nil {
			t.Fatalf("gobgp announce filtered route: %v", err)
		}

		t.Cleanup(func() {
			_, _ = gobgpCmd(ctx, dc, gobgpContainer,
				"global", "rib", "del", prefixes.filtered, "-a", af,
			)
		})

		time.Sleep(10 * time.Second)

		routes, err := cl.GetBGPLearnedRoutes(ctx)
		if err != nil {
			t.Fatalf("get learned routes: %v", err)
		}

		if findLearnedRoute(routes.Routes, prefixes.filtered) != nil {
			t.Fatalf("%s should NOT be learned (not in import prefix list)", prefixes.filtered)
		}
	})

	t.Run("SafetyFilter", func(t *testing.T) {
		peerID := findPeerIDByAddress(ctx, t, cl, peerAddr)

		if err := cl.UpdateBGPPeer(ctx, &client.UpdateBGPPeerOptions{
			ID:       peerID,
			Address:  peerAddr,
			RemoteAS: bgpPeerAS,
			HoldTime: 90,
			ImportPrefixes: []client.BGPImportPrefix{
				prefixes.importAll,
			},
		}); err != nil {
			t.Fatalf("update peer import prefixes: %v", err)
		}

		t.Cleanup(func() {
			_ = cl.UpdateBGPPeer(ctx, &client.UpdateBGPPeerOptions{
				ID:       peerID,
				Address:  peerAddr,
				RemoteAS: bgpPeerAS,
				HoldTime: 90,
				ImportPrefixes: []client.BGPImportPrefix{
					prefixes.importAllow,
				},
			})
		})

		for _, prefix := range append(prefixes.dangerous, prefixes.benign) {
			if _, err := gobgpCmd(ctx, dc, gobgpContainer,
				"global", "rib", "add", prefix, "-a", af,
			); err != nil {
				t.Fatalf("gobgp announce %s: %v", prefix, err)
			}
		}

		t.Cleanup(func() {
			for _, prefix := range append(prefixes.dangerous, prefixes.benign) {
				_, _ = gobgpCmd(ctx, dc, gobgpContainer,
					"global", "rib", "del", prefix, "-a", af,
				)
			}
		})

		waitForLearnedRoute(ctx, t, cl, prefixes.benign)

		routes, err := cl.GetBGPLearnedRoutes(ctx)
		if err != nil {
			t.Fatalf("get learned routes: %v", err)
		}

		for _, prefix := range prefixes.dangerous {
			if findLearnedRoute(routes.Routes, prefix) != nil {
				t.Errorf("dangerous prefix %s should NOT be learned", prefix)
			}
		}

		if findLearnedRoute(routes.Routes, prefixes.benign) == nil {
			t.Fatalf("benign prefix %s should be learned", prefixes.benign)
		}
	})

	t.Run("RouteWithdrawal", func(t *testing.T) {
		if _, err := gobgpCmd(ctx, dc, gobgpContainer,
			"global", "rib", "add", prefixes.learnable, "-a", af,
		); err != nil {
			t.Fatalf("gobgp announce route: %v", err)
		}

		waitForLearnedRoute(ctx, t, cl, prefixes.learnable)

		if _, err := gobgpCmd(ctx, dc, gobgpContainer,
			"global", "rib", "del", prefixes.learnable, "-a", af,
		); err != nil {
			t.Fatalf("gobgp withdraw route: %v", err)
		}

		waitForNoLearnedRoute(ctx, t, cl, prefixes.learnable, 30*time.Second)
	})

	t.Run("PeerDeletion", func(t *testing.T) {
		if _, err := gobgpCmd(ctx, dc, gobgpContainer,
			"global", "rib", "add", prefixes.learnable, "-a", af,
		); err != nil {
			t.Fatalf("gobgp announce route: %v", err)
		}

		peerID := findPeerIDByAddress(ctx, t, cl, peerAddr)

		waitForLearnedRoute(ctx, t, cl, prefixes.learnable)

		if err := cl.DeleteBGPPeer(ctx, &client.DeleteBGPPeerOptions{ID: peerID}); err != nil {
			t.Fatalf("delete peer: %v", err)
		}

		waitForNoLearnedRoute(ctx, t, cl, prefixes.learnable, 15*time.Second)

		if err := cl.CreateBGPPeer(ctx, &client.CreateBGPPeerOptions{
			Address:  peerAddr,
			RemoteAS: bgpPeerAS,
			HoldTime: 90,
			ImportPrefixes: []client.BGPImportPrefix{
				prefixes.importAllow,
			},
		}); err != nil {
			t.Fatalf("re-create peer: %v", err)
		}

		waitForBGPSession(ctx, t, cl, peerAddr, 60*time.Second)
		waitForLearnedRoute(ctx, t, cl, prefixes.learnable)

		t.Cleanup(func() {
			_, _ = gobgpCmd(ctx, dc, gobgpContainer,
				"global", "rib", "del", prefixes.learnable, "-a", af,
			)
		})
	})

	t.Run("NATToggleSuppressesAdvertising", func(t *testing.T) {
		if err := cl.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: true}); err != nil {
			t.Fatalf("enable NAT: %v", err)
		}

		t.Cleanup(func() {
			_ = cl.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: false})
		})

		time.Sleep(5 * time.Second)

		peers, err := cl.ListBGPPeers(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list peers: %v", err)
		}

		peer := findPeerByAddress(peers.Items, peerAddr)
		if peer == nil {
			t.Fatalf("peer %s not found", peerAddr)
		}

		if peer.State != "established" {
			t.Fatalf("peer state after NAT enable: got %q, want %q", peer.State, "established")
		}

		advertised, err := cl.GetBGPAdvertisedRoutes(ctx)
		if err != nil {
			t.Fatalf("get advertised routes: %v", err)
		}

		if len(advertised.Routes) != 0 {
			t.Fatalf("advertised routes after NAT enable: got %d, want 0", len(advertised.Routes))
		}

		if err := cl.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{Enabled: false}); err != nil {
			t.Fatalf("disable NAT: %v", err)
		}

		time.Sleep(5 * time.Second)

		peers, err = cl.ListBGPPeers(ctx, &client.ListParams{Page: 1, PerPage: 100})
		if err != nil {
			t.Fatalf("list peers after NAT disable: %v", err)
		}

		peer = findPeerByAddress(peers.Items, peerAddr)
		if peer == nil {
			t.Fatalf("peer %s not found after NAT disable", peerAddr)
		}

		if peer.State != "established" {
			t.Fatalf("peer state after NAT disable: got %q, want %q", peer.State, "established")
		}
	})
}

// --- Helper functions ---

func waitForGoBGPReady(ctx context.Context, t *testing.T, dc *DockerClient, container string, timeout time.Duration) {
	t.Helper()

	deadline := time.After(timeout)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for GoBGP to be ready")
		case <-ticker.C:
			if _, err := gobgpCmd(ctx, dc, container, "global"); err == nil {
				return
			}
		}
	}
}

func waitForBGPSession(ctx context.Context, t *testing.T, cl *client.Client, peerAddr string, timeout time.Duration) {
	t.Helper()

	deadline := time.After(timeout)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for BGP session with %s to reach established", peerAddr)
		case <-ticker.C:
			peers, err := cl.ListBGPPeers(ctx, &client.ListParams{Page: 1, PerPage: 100})
			if err != nil {
				continue
			}

			peer := findPeerByAddress(peers.Items, peerAddr)
			if peer != nil && peer.State == "established" {
				return
			}
		}
	}
}

func gobgpCmd(ctx context.Context, dc *DockerClient, container string, args ...string) (string, error) {
	argv := append([]string{"gobgp"}, args...)

	return dc.Exec(ctx, container, argv, false, 10*time.Second, nil)
}

func waitForLearnedRoute(ctx context.Context, t *testing.T, cl *client.Client, prefix string) {
	t.Helper()

	deadline := time.After(30 * time.Second)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for learned route %s", prefix)
		case <-ticker.C:
			routes, err := cl.GetBGPLearnedRoutes(ctx)
			if err != nil {
				continue
			}

			if findLearnedRoute(routes.Routes, prefix) != nil {
				return
			}
		}
	}
}

func waitForNoLearnedRoute(ctx context.Context, t *testing.T, cl *client.Client, prefix string, timeout time.Duration) {
	t.Helper()

	deadline := time.After(timeout)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for learned route %s to be removed", prefix)
		case <-ticker.C:
			routes, err := cl.GetBGPLearnedRoutes(ctx)
			if err != nil {
				continue
			}

			if findLearnedRoute(routes.Routes, prefix) == nil {
				return
			}
		}
	}
}

func waitForAdvertisedRouteInPool(ctx context.Context, t *testing.T, cl *client.Client, pool string, timeout time.Duration) client.BGPAdvertisedRoute {
	t.Helper()

	deadline := time.After(timeout)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for advertised route in pool %s", pool)

			return client.BGPAdvertisedRoute{}
		case <-ticker.C:
			routes, err := cl.GetBGPAdvertisedRoutes(ctx)
			if err != nil {
				continue
			}

			for _, r := range routes.Routes {
				if routeInPool(r.Prefix, pool) {
					return r
				}
			}
		}
	}
}

func routeInPool(prefix, pool string) bool {
	_, poolNet, err := net.ParseCIDR(pool)
	if err != nil {
		return false
	}

	ip, _, err := net.ParseCIDR(prefix)
	if err != nil {
		ip = net.ParseIP(prefix)
		if ip == nil {
			return false
		}
	}

	return poolNet.Contains(ip)
}

func findPeerByAddress(peers []client.BGPPeer, addr string) *client.BGPPeer {
	for i := range peers {
		if peers[i].Address == addr {
			return &peers[i]
		}
	}

	return nil
}

func findPeerIDByAddress(ctx context.Context, t *testing.T, cl *client.Client, addr string) int {
	t.Helper()

	peers, err := cl.ListBGPPeers(ctx, &client.ListParams{Page: 1, PerPage: 100})
	if err != nil {
		t.Fatalf("list peers: %v", err)
	}

	peer := findPeerByAddress(peers.Items, addr)
	if peer == nil {
		t.Fatalf("peer %s not found", addr)
	}

	return peer.ID
}

func findLearnedRoute(routes []client.BGPLearnedRoute, prefix string) *client.BGPLearnedRoute {
	for i := range routes {
		if routes[i].Prefix == prefix {
			return &routes[i]
		}
	}

	return nil
}
