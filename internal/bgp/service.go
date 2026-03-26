package bgp

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"

	api "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gobgp "github.com/osrg/gobgp/v4/pkg/server"
	"go.uber.org/zap"
)

const defaultListenPort int32 = 179

// ownedPath tracks an advertised prefix and its owner.
type ownedPath struct {
	ip    net.IP
	owner string
}

// BGPService manages the embedded BGP speaker.
type BGPService struct {
	server     *gobgp.BgpServer
	mu         sync.RWMutex
	running    bool
	settings   BGPSettings
	peers      []BGPPeer
	paths      map[string]ownedPath // keyed by IP string e.g. "10.45.0.3"
	n6Addr     net.IP
	logger     *zap.Logger
	listenPort int32
}

// New creates a BGPService. Does not start the speaker.
func New(n6Addr net.IP, logger *zap.Logger) *BGPService {
	return &BGPService{
		n6Addr:     n6Addr,
		logger:     logger,
		listenPort: defaultListenPort,
		paths:      make(map[string]ownedPath),
	}
}

// SetListenPort overrides the BGP listen port. Use -1 in tests to disable TCP listening.
func (b *BGPService) SetListenPort(port int32) {
	b.listenPort = port
}

// Start initializes the GoBGP server with the given settings and peers,
// and announces /32 routes for all currently allocated IPs.
func (b *BGPService) Start(ctx context.Context, settings BGPSettings, peers []BGPPeer, allocatedIPs []net.IP) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return fmt.Errorf("BGP service is already running")
	}

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

// startLocked starts the GoBGP server. Must be called with mu held.
// If allocatedIPs is nil, existing paths from the in-memory map are replayed.
func (b *BGPService) startLocked(ctx context.Context, settings BGPSettings, peers []BGPPeer, allocatedIPs []net.IP) error {
	routerID := settings.RouterID
	if routerID == "" {
		routerID = b.n6Addr.String()
	}

	listenPort := b.resolveListenPort(settings)

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
		// Fresh start: announce from DB and populate the paths map.
		b.paths = make(map[string]ownedPath, len(allocatedIPs))

		for _, ip := range allocatedIPs {
			if err := b.announcePath(s, ip); err != nil {
				b.logger.Warn("failed to announce initial BGP route",
					zap.String("ip", ip.String()), zap.Error(err))
			}

			b.paths[ip.String()] = ownedPath{ip: ip, owner: ""}
		}
	} else {
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

// Announce adds a /32 route for the given IP to the BGP RIB, tagged with the given owner.
func (b *BGPService) Announce(ip net.IP, owner string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return nil
	}

	if err := b.announcePath(b.server, ip); err != nil {
		return err
	}

	b.paths[ip.String()] = ownedPath{ip: ip, owner: owner}

	return nil
}

// Withdraw removes a /32 route for the given IP from the BGP RIB.
func (b *BGPService) Withdraw(ip net.IP) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
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

// GetRoutes returns the currently advertised routes.
func (b *BGPService) GetRoutes() ([]BGPRoute, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.running {
		return nil, nil
	}

	return b.listRoutesLocked()
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

	err := b.server.ListPeer(ctx, &api.ListPeerRequest{}, func(peer *api.Peer) {
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

		// Count prefixes sent from AfiSafi state
		for _, afiSafi := range peer.GetAfiSafis() {
			afiState := afiSafi.GetState()
			if afiState != nil {
				ps.PrefixesSent += int(afiState.GetAdvertised())
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
