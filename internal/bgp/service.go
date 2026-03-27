package bgp

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/kernel"
	api "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gobgp "github.com/osrg/gobgp/v4/pkg/server"
	"go.uber.org/zap"
)

const defaultListenPort int32 = 179

// bgpRouteMetric is the fixed kernel route metric for all BGP-learned routes.
const bgpRouteMetric = 200

// MaxLearnedRoutesPerPeer is the maximum number of routes that can be learned
// from a single BGP peer. Routes beyond this limit are silently dropped.
// This prevents a misconfigured or malicious peer from flooding the kernel
// routing table.
const MaxLearnedRoutesPerPeer = 1000

// ownedPath tracks an advertised prefix and its owner.
type ownedPath struct {
	ip    net.IP
	owner string
}

// learnedRoute tracks a BGP-learned route installed in the kernel.
type learnedRoute struct {
	prefix  net.IPNet
	gateway net.IP
	peer    string // peer address that advertised this route
}

// BGPService manages the embedded BGP speaker.
//
// Lock hierarchy (acquire in this order to avoid deadlocks):
//
//	mu          — protects server, running, advertising, settings, peers, paths
//	learnedMu   — protects learnedRoutes
//
// Never acquire mu while holding learnedMu. Code that needs both must acquire
// mu first, then learnedMu. handleSinglePath snapshots mu-protected fields
// under mu.RLock, releases it, then acquires learnedMu alone.
type BGPService struct {
	server      *gobgp.BgpServer
	mu          sync.RWMutex
	running     bool
	advertising bool // false when NAT is enabled — suppresses route announcements
	settings    BGPSettings
	peers       []BGPPeer
	paths       map[string]ownedPath // keyed by IP string e.g. "10.45.0.3"
	n6Addr      net.IP
	logger      *zap.Logger
	listenPort  int32

	// Route learning dependencies (optional — nil means route learning is disabled)
	kernel      kernel.Kernel
	importStore ImportPrefixStore
	filter      *RouteFilter

	// learnedMu protects learnedRoutes. Acquired after mu when both are needed.
	learnedMu     sync.RWMutex
	learnedRoutes map[string]learnedRoute // keyed by prefix string e.g. "0.0.0.0/0"
	cancelWatch   context.CancelFunc
}

// Option configures the BGP service.
type Option func(*BGPService)

// WithKernel sets the kernel interface for installing learned routes.
func WithKernel(k kernel.Kernel) Option {
	return func(b *BGPService) { b.kernel = k }
}

// WithImportPrefixStore sets the store for loading per-peer import prefix lists.
func WithImportPrefixStore(s ImportPrefixStore) Option {
	return func(b *BGPService) { b.importStore = s }
}

// WithRouteFilter sets the safety rejection filter for learned routes.
func WithRouteFilter(f *RouteFilter) Option {
	return func(b *BGPService) { b.filter = f }
}

// UpdateFilter replaces the safety rejection filter and re-evaluates all
// learned routes against the new filter. Routes that now match a reject
// prefix are immediately removed from the kernel.
func (b *BGPService) UpdateFilter(f *RouteFilter) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.filter = f

	if !b.running || !b.routeLearningEnabled() {
		return
	}

	b.learnedMu.Lock()
	defer b.learnedMu.Unlock()

	removed := 0

	for prefixStr, lr := range b.learnedRoutes {
		dst := lr.prefix

		if f.overlapsAny(&dst) {
			err := b.kernel.DeleteRoute(&dst, lr.gateway, bgpRouteMetric, kernel.N6)
			if err != nil {
				b.logger.Warn("failed to remove route rejected by updated filter",
					zap.String("prefix", prefixStr), zap.Error(err))
			}

			delete(b.learnedRoutes, prefixStr)

			removed++
		}
	}

	if removed > 0 {
		b.logger.Info("removed learned routes after filter update",
			zap.Int("removed", removed), zap.Int("remaining", len(b.learnedRoutes)))
	}
}

// New creates a BGPService. Does not start the speaker.
func New(n6Addr net.IP, logger *zap.Logger, opts ...Option) *BGPService {
	b := &BGPService{
		n6Addr:        n6Addr,
		logger:        logger,
		listenPort:    defaultListenPort,
		paths:         make(map[string]ownedPath),
		learnedRoutes: make(map[string]learnedRoute),
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// SetListenPort overrides the BGP listen port. Use -1 in tests to disable TCP listening.
func (b *BGPService) SetListenPort(port int32) {
	b.listenPort = port
}

// InjectLearnedRouteForTest adds a learned route to the in-memory map without
// actually installing it in the kernel. Used by tests to set up state before
// testing reconfiguration and cleanup methods.
func (b *BGPService) InjectLearnedRouteForTest(prefix net.IPNet, gateway net.IP, peer string) {
	b.learnedMu.Lock()
	defer b.learnedMu.Unlock()

	b.learnedRoutes[prefix.String()] = learnedRoute{
		prefix:  prefix,
		gateway: gateway,
		peer:    peer,
	}
}

// Start initializes the GoBGP server with the given settings and peers,
// and announces /32 routes for all currently allocated IPs.
// allocatedIPs maps IP strings (e.g. "10.45.0.1") to subscriber IMSIs.
// When advertising is false (NAT enabled), the BGP speaker starts but does not
// announce subscriber /32 routes.
func (b *BGPService) Start(ctx context.Context, settings BGPSettings, peers []BGPPeer, allocatedIPs map[string]string, advertising bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return fmt.Errorf("BGP service is already running")
	}

	b.advertising = advertising

	return b.startLocked(ctx, settings, peers, allocatedIPs)
}

// resolveListenPort returns the TCP port for the BGP speaker.
// If a test override is set (listenPort != defaultListenPort), it takes precedence.
// Otherwise, the port is parsed from settings.ListenAddress (e.g. ":179", "0.0.0.0:1179").
func (b *BGPService) resolveListenPort(settings BGPSettings) int32 {
	if b.listenPort != defaultListenPort {
		return b.listenPort
	}

	if settings.ListenAddress != "" {
		_, portStr, err := net.SplitHostPort(settings.ListenAddress)
		if err == nil {
			port, err := strconv.ParseInt(portStr, 10, 32)
			if err == nil {
				return int32(port)
			}
		}
	}

	return defaultListenPort
}

// routeLearningEnabled returns true if the service has all dependencies
// needed to learn routes from peers and install them in the kernel.
func (b *BGPService) routeLearningEnabled() bool {
	return b.kernel != nil && b.importStore != nil && b.filter != nil
}

// startLocked starts the GoBGP server. Must be called with mu held.
// If allocatedIPs is nil, existing paths from the in-memory map are replayed.
func (b *BGPService) startLocked(ctx context.Context, settings BGPSettings, peers []BGPPeer, allocatedIPs map[string]string) error {
	routerID := settings.RouterID
	if routerID == "" {
		routerID = b.n6Addr.String()
	}

	listenPort := b.resolveListenPort(settings)

	// Clean stale BGP routes from a prior crash before starting the speaker.
	if b.routeLearningEnabled() {
		b.cleanStaleRoutes()
	}

	s := gobgp.NewBgpServer(gobgp.GrpcListenAddress(""))
	go s.Serve()

	err := s.StartBgp(ctx, &api.StartBgpRequest{
		Global: &api.Global{
			Asn:        uint32(settings.LocalAS),
			RouterId:   routerID,
			ListenPort: listenPort,
		},
	})
	if err != nil {
		s.Stop()

		return fmt.Errorf("failed to start BGP speaker: %w", err)
	}

	for _, peer := range peers {
		if err := b.addPeer(ctx, s, peer); err != nil {
			b.logger.Warn("failed to add BGP peer",
				zap.String("address", peer.Address), zap.Error(err))
		}
	}

	if allocatedIPs != nil {
		if b.advertising {
			// Fresh start with advertising: populate paths and announce.
			b.paths = make(map[string]ownedPath, len(allocatedIPs))

			for ipStr, subscriber := range allocatedIPs {
				ip := net.ParseIP(ipStr)
				if ip == nil {
					continue
				}

				if err := b.announcePath(s, ip); err != nil {
					b.logger.Warn("failed to announce initial BGP route",
						zap.String("ip", ipStr), zap.Error(err))
				}

				b.paths[ipStr] = ownedPath{ip: ip, owner: subscriber}
			}
		} else {
			// Fresh start without advertising: paths map stays empty.
			b.paths = make(map[string]ownedPath)
			b.logger.Info("Route advertisement suppressed (NAT is enabled)")
		}
	} else if b.advertising {
		// Replay from in-memory paths map (after reconfiguration restart).
		for _, op := range b.paths {
			if err := b.announcePath(s, op.ip); err != nil {
				b.logger.Warn("failed to re-announce route after restart",
					zap.String("ip", op.ip.String()), zap.Error(err))
			}
		}
	}

	b.server = s
	b.running = true
	b.settings = settings
	b.peers = peers

	// Start watching best paths for route learning.
	if b.routeLearningEnabled() {
		watchCtx, cancel := context.WithCancel(context.Background())

		if err := b.startWatchBestPaths(watchCtx, s); err != nil {
			cancel()

			_ = s.StopBgp(ctx, &api.StopBgpRequest{})

			return fmt.Errorf("failed to start best-path watcher: %w", err)
		}

		b.cancelWatch = cancel
	}

	b.logger.Info("BGP service started",
		zap.Int("localAS", settings.LocalAS),
		zap.String("routerID", routerID),
		zap.Int("peers", len(peers)),
		zap.Int("routes", len(b.paths)),
	)

	return nil
}

// stopLocked shuts down the GoBGP server. Must be called with mu held.
func (b *BGPService) stopLocked() error {
	if !b.running {
		return nil
	}

	// Stop the best-path watcher before shutting down GoBGP.
	if b.cancelWatch != nil {
		b.cancelWatch()
		b.cancelWatch = nil
	}

	// Remove all learned routes from the kernel.
	b.removeAllLearnedRoutes()

	ctx := context.Background()

	err := b.server.StopBgp(ctx, &api.StopBgpRequest{})
	if err != nil {
		return fmt.Errorf("failed to stop BGP speaker: %w", err)
	}

	b.server = nil
	b.running = false
	b.logger.Info("BGP service stopped")

	return nil
}

// Stop gracefully shuts down the BGP speaker.
func (b *BGPService) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.stopLocked()
}

// Announce adds a /32 route for the given IP to the BGP RIB.
// It is a no-op if the service is not running or not advertising (NAT enabled).
func (b *BGPService) Announce(ip net.IP, owner string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running || !b.advertising {
		return nil
	}

	if err := b.announcePath(b.server, ip); err != nil {
		return err
	}

	b.paths[ip.String()] = ownedPath{ip: ip, owner: owner}

	return nil
}

// Withdraw removes a /32 route for the given IP from the BGP RIB.
// It is a no-op if the service is not running or not advertising (NAT enabled).
func (b *BGPService) Withdraw(ip net.IP) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running || !b.advertising {
		return nil
	}

	if err := b.withdrawPath(b.server, ip); err != nil {
		return err
	}

	delete(b.paths, ip.String())

	return nil
}

// Reconfigure applies new settings/peers. If the local AS, router ID, or listen
// address changed, a full restart is required. On failure, the previous working
// config is restored (reverter pattern).
func (b *BGPService) Reconfigure(ctx context.Context, settings BGPSettings, peers []BGPPeer) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return nil
	}

	needsRestart := settings.LocalAS != b.settings.LocalAS ||
		settings.RouterID != b.settings.RouterID ||
		settings.ListenAddress != b.settings.ListenAddress

	if needsRestart {
		oldSettings := b.settings
		oldPeers := b.peers

		if err := b.stopLocked(); err != nil {
			return fmt.Errorf("failed to stop BGP for reconfigure: %w", err)
		}

		// Try starting with new settings (replays paths from in-memory map).
		err := b.startLocked(ctx, settings, peers, nil)
		if err != nil {
			// Rollback: restore previous working config.
			rollbackErr := b.startLocked(ctx, oldSettings, oldPeers, nil)
			if rollbackErr != nil {
				return fmt.Errorf("reconfigure failed: %w; rollback also failed: %w", err, rollbackErr)
			}

			return fmt.Errorf("reconfigure failed, rolled back to previous config: %w", err)
		}
	} else {
		// Hot reconfigure: add/remove peers as needed
		b.reconcilePeers(ctx, peers)
		b.settings = settings
		b.peers = peers

		// Re-evaluate learned routes against the (possibly changed) import prefix lists.
		if b.routeLearningEnabled() {
			b.reEvaluateLearnedRoutes(ctx, peers)
			b.replayGlobalRIB(ctx, peers)
		}
	}

	return nil
}

// GetStatus returns the live peer session states of the BGP speaker.
func (b *BGPService) GetStatus(ctx context.Context) (*BGPStatus, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.running {
		return &BGPStatus{}, nil
	}

	peerStatuses, err := b.listPeerStatusesLocked(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer statuses: %w", err)
	}

	return &BGPStatus{
		Peers: peerStatuses,
	}, nil
}

// GetRoutes returns the currently advertised routes, sorted by prefix.
func (b *BGPService) GetRoutes() ([]BGPRoute, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.running {
		return nil, nil
	}

	routes, err := b.listRoutesLocked()
	if err != nil {
		return nil, err
	}

	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Prefix < routes[j].Prefix
	})

	return routes, nil
}

// GetLearnedRoutes returns the currently installed BGP-learned routes, sorted by prefix.
func (b *BGPService) GetLearnedRoutes() []LearnedRoute {
	b.learnedMu.RLock()
	defer b.learnedMu.RUnlock()

	routes := make([]LearnedRoute, 0, len(b.learnedRoutes))

	for _, lr := range b.learnedRoutes {
		routes = append(routes, LearnedRoute{
			Prefix:  lr.prefix.String(),
			NextHop: lr.gateway.String(),
			Peer:    lr.peer,
		})
	}

	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Prefix < routes[j].Prefix
	})

	return routes
}

// CountLearnedRoutesByPeer returns the number of learned routes from a specific peer address.
func (b *BGPService) CountLearnedRoutesByPeer(peerAddr string) int {
	b.learnedMu.RLock()
	defer b.learnedMu.RUnlock()

	count := 0

	for _, lr := range b.learnedRoutes {
		if lr.peer == peerAddr {
			count++
		}
	}

	return count
}

// LearnedRouteCountsByPeer returns a map from peer address to learned route count.
// More efficient than calling CountLearnedRoutesByPeer per peer.
func (b *BGPService) LearnedRouteCountsByPeer() map[string]int {
	b.learnedMu.RLock()
	defer b.learnedMu.RUnlock()

	counts := make(map[string]int, len(b.learnedRoutes))

	for _, lr := range b.learnedRoutes {
		counts[lr.peer]++
	}

	return counts
}

// GetEffectiveRouterID resolves an empty router ID to the N6 address default.
func (b *BGPService) GetEffectiveRouterID(configuredRouterID string) string {
	if configuredRouterID != "" {
		return configuredRouterID
	}

	return b.n6Addr.String()
}

// IsRunning returns true if the BGP speaker is currently active.
func (b *BGPService) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.running
}

// IsAdvertising returns true if the BGP speaker is running and advertising
// subscriber routes. Returns false when NAT is enabled.
func (b *BGPService) IsAdvertising() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.running && b.advertising
}

// SetAdvertising toggles whether the BGP speaker advertises subscriber /32
// routes. When enabling, allocatedIPs (from the database) is used to rebuild
// the paths map and announce all routes. When disabling, all paths are
// withdrawn and the map is cleared. It is a no-op if the service is not running.
func (b *BGPService) SetAdvertising(advertising bool, allocatedIPs map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running || b.advertising == advertising {
		return
	}

	b.advertising = advertising

	if advertising {
		b.paths = make(map[string]ownedPath, len(allocatedIPs))

		for ipStr, subscriber := range allocatedIPs {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}

			if err := b.announcePath(b.server, ip); err != nil {
				b.logger.Warn("failed to announce route after enabling advertising",
					zap.String("ip", ipStr), zap.Error(err))
			}

			b.paths[ipStr] = ownedPath{ip: ip, owner: subscriber}
		}

		b.logger.Info("BGP route advertising enabled", zap.Int("routes", len(b.paths)))
	} else {
		for _, op := range b.paths {
			if err := b.withdrawPath(b.server, op.ip); err != nil {
				b.logger.Warn("failed to withdraw route after disabling advertising",
					zap.String("ip", op.ip.String()), zap.Error(err))
			}
		}

		b.paths = make(map[string]ownedPath)

		b.logger.Info("BGP route advertising suppressed (NAT enabled)")
	}
}

// addPeer adds a single peer to the GoBGP server.
func (b *BGPService) addPeer(ctx context.Context, s *gobgp.BgpServer, peer BGPPeer) error {
	p := &api.Peer{
		Conf: &api.PeerConf{
			NeighborAddress: peer.Address,
			PeerAsn:         uint32(peer.RemoteAS),
			Description:     peer.Description,
			AuthPassword:    peer.Password,
		},
		EbgpMultihop: &api.EbgpMultihop{
			Enabled:     true,
			MultihopTtl: 255,
		},
		GracefulRestart: &api.GracefulRestart{
			Enabled:     true,
			RestartTime: 300,
		},
		Timers: &api.Timers{
			Config: &api.TimersConfig{
				HoldTime:                     uint64(peer.HoldTime),
				MinimumAdvertisementInterval: 0,
			},
		},
		AfiSafis: []*api.AfiSafi{
			{
				Config: &api.AfiSafiConfig{
					Family: &api.Family{
						Afi:  api.Family_AFI_IP,
						Safi: api.Family_SAFI_UNICAST,
					},
					Enabled: true,
				},
				MpGracefulRestart: &api.MpGracefulRestart{
					Config: &api.MpGracefulRestartConfig{Enabled: true},
				},
			},
		},
	}

	return s.AddPeer(ctx, &api.AddPeerRequest{Peer: p})
}

func buildPath(ip net.IP, nextHopAddr net.IP) (*apiutil.Path, error) {
	ipv4 := ip.To4()
	if ipv4 == nil {
		return nil, fmt.Errorf("only IPv4 addresses are supported, got: %s", ip.String())
	}

	addr, ok := netip.AddrFromSlice(ipv4)
	if !ok {
		return nil, fmt.Errorf("failed to convert IP to netip.Addr: %s", ip.String())
	}

	prefix := netip.PrefixFrom(addr, 32)

	nlri, err := bgppacket.NewIPAddrPrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create NLRI: %w", err)
	}

	nhAddr, ok := netip.AddrFromSlice(nextHopAddr.To4())
	if !ok {
		return nil, fmt.Errorf("failed to convert next-hop to netip.Addr: %s", nextHopAddr.String())
	}

	origin := bgppacket.NewPathAttributeOrigin(0) // IGP

	nextHop, err := bgppacket.NewPathAttributeNextHop(nhAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create next-hop attribute: %w", err)
	}

	return &apiutil.Path{
		Family: bgppacket.RF_IPv4_UC,
		Nlri:   nlri,
		Attrs:  []bgppacket.PathAttributeInterface{origin, nextHop},
	}, nil
}

// announcePath adds a /32 route to the GoBGP RIB.
func (b *BGPService) announcePath(s *gobgp.BgpServer, ip net.IP) error {
	path, err := buildPath(ip, b.n6Addr)
	if err != nil {
		return err
	}

	_, err = s.AddPath(apiutil.AddPathRequest{
		Paths: []*apiutil.Path{path},
	})

	return err
}

// withdrawPath removes a /32 route from the GoBGP RIB.
func (b *BGPService) withdrawPath(s *gobgp.BgpServer, ip net.IP) error {
	path, err := buildPath(ip, b.n6Addr)
	if err != nil {
		return err
	}

	path.Withdrawal = true

	return s.DeletePath(apiutil.DeletePathRequest{
		Paths: []*apiutil.Path{path},
	})
}

// listPeerStatusesLocked returns the live state of all peers. Must be called with mu held.
func (b *BGPService) listPeerStatusesLocked(ctx context.Context) ([]BGPPeerStatus, error) {
	var statuses []BGPPeerStatus

	err := b.server.ListPeer(ctx, &api.ListPeerRequest{EnableAdvertised: true}, func(peer *api.Peer) {
		ps := BGPPeerStatus{
			Address:  peer.GetConf().GetNeighborAddress(),
			RemoteAS: int(peer.GetConf().GetPeerAsn()),
		}

		state := peer.GetState()
		if state != nil {
			ps.State = sessionStateString(state.GetSessionState())
		}

		timers := peer.GetTimers()
		if timers != nil && timers.GetState() != nil {
			uptime := timers.GetState().GetUptime()
			if uptime != nil && state != nil && state.GetSessionState() == api.PeerState_SESSION_STATE_ESTABLISHED {
				uptimeTime := uptime.AsTime()
				ps.Uptime = time.Since(uptimeTime).Truncate(time.Second).String()
			}
		}

		// Count prefixes sent/received from AfiSafi state
		for _, afiSafi := range peer.GetAfiSafis() {
			afiState := afiSafi.GetState()
			if afiState != nil {
				ps.PrefixesSent += int(afiState.GetAdvertised())
				ps.PrefixesReceived += int(afiState.GetReceived())
			}
		}

		statuses = append(statuses, ps)
	})
	if err != nil {
		return nil, err
	}

	return statuses, nil
}

// listRoutesLocked returns all advertised routes from the global RIB. Must be called with mu held.
func (b *BGPService) listRoutesLocked() ([]BGPRoute, error) {
	var routes []BGPRoute

	err := b.server.ListPath(apiutil.ListPathRequest{
		TableType: api.TableType_TABLE_TYPE_GLOBAL,
		Family:    bgppacket.RF_IPv4_UC,
	}, func(nlri bgppacket.NLRI, paths []*apiutil.Path) {
		for _, path := range paths {
			if path.Withdrawal {
				continue
			}

			route := BGPRoute{
				Prefix: nlri.String(),
			}

			// Extract next-hop from path attributes
			for _, attr := range path.Attrs {
				if nh, ok := attr.(*bgppacket.PathAttributeNextHop); ok {
					route.NextHop = nh.Value.String()

					break
				}
			}

			// Only include locally-originated routes (those we announced).
			// The global RIB may also contain routes learned from peers,
			// which are not relevant to the advertised-routes view.
			ip, _, _ := net.ParseCIDR(route.Prefix)
			if ip == nil {
				continue
			}

			owned, ok := b.paths[ip.String()]
			if !ok {
				continue
			}

			route.Subscriber = owned.owner
			routes = append(routes, route)
		}
	})
	if err != nil {
		return nil, err
	}

	return routes, nil
}

// peerPropertiesMatch returns true if two peers have the same configuration
// (ignoring the ID field which is a DB artifact).
// Note: Address is intentionally not compared here because reconcilePeers uses
// the address as the map key. An address change appears as a remove + add,
// which correctly triggers learned route cleanup for the old address.
func peerPropertiesMatch(a, b BGPPeer) bool {
	return a.RemoteAS == b.RemoteAS &&
		a.HoldTime == b.HoldTime &&
		a.Password == b.Password &&
		a.Description == b.Description
}

// reconcilePeers adds/removes/updates peers to match the desired set. Must be called with mu held.
func (b *BGPService) reconcilePeers(ctx context.Context, desired []BGPPeer) {
	desiredMap := make(map[string]BGPPeer)
	for _, p := range desired {
		desiredMap[p.Address] = p
	}

	currentMap := make(map[string]BGPPeer)
	for _, p := range b.peers {
		currentMap[p.Address] = p
	}

	// Remove peers no longer in the desired set
	for addr := range currentMap {
		if _, ok := desiredMap[addr]; !ok {
			err := b.server.DeletePeer(ctx, &api.DeletePeerRequest{
				Address: addr,
			})
			if err != nil {
				b.logger.Warn("failed to delete BGP peer", zap.String("address", addr), zap.Error(err))
			}

			b.removeLearnedRoutesForPeer(addr)
		}
	}

	for addr, peer := range desiredMap {
		current, exists := currentMap[addr]
		if !exists {
			// Add new peer
			if err := b.addPeer(ctx, b.server, peer); err != nil {
				b.logger.Warn("failed to add BGP peer", zap.String("address", addr), zap.Error(err))
			}
		} else if !peerPropertiesMatch(current, peer) {
			// Properties changed: delete and re-add
			err := b.server.DeletePeer(ctx, &api.DeletePeerRequest{Address: addr})
			if err != nil {
				b.logger.Warn("failed to delete BGP peer for update", zap.String("address", addr), zap.Error(err))

				continue
			}

			if err := b.addPeer(ctx, b.server, peer); err != nil {
				b.logger.Warn("failed to re-add BGP peer after update", zap.String("address", addr), zap.Error(err))
			}
		}
	}
}

func sessionStateString(state api.PeerState_SessionState) string {
	switch state {
	case api.PeerState_SESSION_STATE_ESTABLISHED:
		return "established"
	case api.PeerState_SESSION_STATE_ACTIVE:
		return "active"
	case api.PeerState_SESSION_STATE_CONNECT:
		return "connect"
	case api.PeerState_SESSION_STATE_IDLE:
		return "idle"
	case api.PeerState_SESSION_STATE_OPENSENT:
		return "opensent"
	case api.PeerState_SESSION_STATE_OPENCONFIRM:
		return "openconfirm"
	default:
		return "unknown"
	}
}
