package bgp

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/kernel"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gobgp "github.com/osrg/gobgp/v4/pkg/server"
	"go.uber.org/zap"
)

// startWatchBestPaths registers a WatchEvent callback that installs and removes
// kernel routes as GoBGP's best-path set changes. Must be called with mu held.
func (b *BGPService) startWatchBestPaths(ctx context.Context, s *gobgp.BgpServer) {
	err := s.WatchEvent(ctx, gobgp.WatchEventMessageCallbacks{
		OnBestPath: func(paths []*apiutil.Path, _ time.Time) {
			b.handleBestPaths(ctx, paths)
		},
	}, gobgp.WatchBestPath(true))
	if err != nil {
		b.logger.Warn("failed to start best-path watcher", zap.Error(err))
	}
}

// handleBestPaths processes a batch of best-path updates from GoBGP.
func (b *BGPService) handleBestPaths(ctx context.Context, paths []*apiutil.Path) {
	for _, path := range paths {
		b.handleSinglePath(ctx, path)
	}
}

// handleSinglePath processes a single best-path event: either installing,
// updating, or removing a kernel route.
func (b *BGPService) handleSinglePath(ctx context.Context, path *apiutil.Path) {
	// Extract prefix from NLRI.
	prefix := extractPrefix(path)
	if prefix == nil {
		return
	}

	prefixStr := prefix.String()

	// Extract next-hop.
	nextHop := extractNextHop(path)

	if path.Withdrawal {
		b.handleWithdrawal(prefixStr)

		return
	}

	if !nextHop.IsValid() {
		b.logger.Debug("skipping path with no next-hop", zap.String("prefix", prefixStr))

		return
	}

	nextHopIP := nextHop.As4()
	gwIP := net.IP(nextHopIP[:])

	peerAddr := path.PeerAddress.String()

	// Skip locally-originated routes (our own subscriber /32s).
	b.mu.RLock()
	_, isOwnedPath := b.paths[prefix.IP.String()]
	running := b.running
	peers := b.peers
	b.mu.RUnlock()

	if !running {
		return
	}

	if isOwnedPath {
		return
	}

	// Skip routes with our own next-hop (routing loop prevention).
	if gwIP.Equal(b.n6Addr) {
		b.logger.Debug("skipping route with own next-hop",
			zap.String("prefix", prefixStr), zap.String("nextHop", gwIP.String()))

		return
	}

	// Safety rejection: check against hard-coded reject prefixes.
	if b.filter.overlapsAny(prefix) {
		b.logger.Warn("rejecting BGP route (safety filter)",
			zap.String("prefix", prefixStr), zap.String("peer", peerAddr))

		return
	}

	// Per-peer prefix list filtering.
	peerID := findPeerID(peers, peerAddr)
	if peerID == 0 {
		b.logger.Debug("skipping route from unknown peer", zap.String("peer", peerAddr))

		return
	}

	entries, err := b.loadImportPrefixes(ctx, peerID)
	if err != nil {
		b.logger.Warn("failed to load import prefixes",
			zap.Int("peerID", peerID), zap.Error(err))

		return
	}

	if len(entries) == 0 || !matchesPrefixList(prefix, entries) {
		return
	}

	// Install or update the route in the kernel.
	err = b.kernel.ReplaceRoute(prefix, gwIP, bgpRouteMetric, kernel.N6)
	if err != nil {
		b.logger.Warn("failed to install BGP route",
			zap.String("prefix", prefixStr), zap.Error(err))

		return
	}

	b.learnedMu.Lock()
	b.learnedRoutes[prefixStr] = learnedRoute{
		prefix:  *prefix,
		gateway: gwIP,
		peer:    peerAddr,
	}
	b.learnedMu.Unlock()

	b.logger.Info("installed BGP route",
		zap.String("prefix", prefixStr),
		zap.String("nextHop", gwIP.String()),
		zap.String("peer", peerAddr),
	)
}

// handleWithdrawal removes a previously learned route from the kernel.
func (b *BGPService) handleWithdrawal(prefixStr string) {
	b.learnedMu.Lock()
	lr, exists := b.learnedRoutes[prefixStr]

	if !exists {
		b.learnedMu.Unlock()

		return
	}

	delete(b.learnedRoutes, prefixStr)
	b.learnedMu.Unlock()

	dst := lr.prefix

	err := b.kernel.DeleteRoute(&dst, lr.gateway, bgpRouteMetric, kernel.N6)
	if err != nil {
		b.logger.Warn("failed to remove withdrawn BGP route",
			zap.String("prefix", prefixStr), zap.Error(err))

		return
	}

	b.logger.Info("removed BGP route (withdrawn)",
		zap.String("prefix", prefixStr), zap.String("peer", lr.peer))
}

// cleanStaleRoutes removes leftover metric-200 routes from a prior crash.
// Called before the BGP speaker starts so we begin with a clean slate.
func (b *BGPService) cleanStaleRoutes() {
	stale, err := b.kernel.ListRoutesByPriority(bgpRouteMetric, kernel.N6)
	if err != nil {
		b.logger.Warn("failed to list stale BGP routes", zap.Error(err))

		return
	}

	for i := range stale {
		dst := stale[i]

		// We don't know the original gateway, but DeleteRoute with a nil
		// gateway will match on destination + priority + interface.
		err := b.kernel.DeleteRoute(&dst, nil, bgpRouteMetric, kernel.N6)
		if err != nil {
			b.logger.Warn("failed to remove stale BGP route",
				zap.String("prefix", dst.String()), zap.Error(err))
		}
	}

	if len(stale) > 0 {
		b.logger.Info("cleaned stale BGP routes from prior run",
			zap.Int("count", len(stale)))
	}
}

// removeAllLearnedRoutes removes all learned routes from the kernel and
// clears the in-memory map. Called during Stop().
func (b *BGPService) removeAllLearnedRoutes() {
	b.learnedMu.Lock()
	defer b.learnedMu.Unlock()

	for prefixStr, lr := range b.learnedRoutes {
		dst := lr.prefix

		err := b.kernel.DeleteRoute(&dst, lr.gateway, bgpRouteMetric, kernel.N6)
		if err != nil {
			b.logger.Warn("failed to remove learned BGP route on stop",
				zap.String("prefix", prefixStr), zap.Error(err))
		}
	}

	if len(b.learnedRoutes) > 0 {
		b.logger.Info("removed all BGP-learned routes",
			zap.Int("count", len(b.learnedRoutes)))
	}

	b.learnedRoutes = make(map[string]learnedRoute)
}

// loadImportPrefixes fetches and parses the import prefix list for a peer.
func (b *BGPService) loadImportPrefixes(ctx context.Context, peerID int) ([]ImportPrefix, error) {
	rawEntries, err := b.importStore.ListImportPrefixes(ctx, peerID)
	if err != nil {
		return nil, err
	}

	entries := make([]ImportPrefix, 0, len(rawEntries))

	for _, raw := range rawEntries {
		_, network, err := net.ParseCIDR(raw.Prefix)
		if err != nil {
			b.logger.Warn("skipping invalid import prefix",
				zap.String("prefix", raw.Prefix), zap.Error(err))

			continue
		}

		entries = append(entries, ImportPrefix{
			Prefix:    network,
			MaxLength: raw.MaxLength,
		})
	}

	return entries, nil
}

// findPeerID returns the DB ID for the peer with the given address, or 0 if not found.
func findPeerID(peers []BGPPeer, address string) int {
	for _, p := range peers {
		if p.Address == address {
			return p.ID
		}
	}

	return 0
}

// extractPrefix returns the route destination from a path's NLRI.
func extractPrefix(path *apiutil.Path) *net.IPNet {
	if path.Nlri == nil {
		return nil
	}

	ipPrefix, ok := path.Nlri.(*bgppacket.IPAddrPrefix)
	if !ok {
		return nil
	}

	prefix := ipPrefix.Prefix
	addr := prefix.Addr()
	bits := prefix.Bits()

	ip := addr.As4()

	return &net.IPNet{
		IP:   net.IP(ip[:]),
		Mask: net.CIDRMask(bits, 32),
	}
}

// extractNextHop returns the next-hop address from a path's attributes.
func extractNextHop(path *apiutil.Path) netip.Addr {
	for _, attr := range path.Attrs {
		if nh, ok := attr.(*bgppacket.PathAttributeNextHop); ok {
			return nh.Value
		}

		if mp, ok := attr.(*bgppacket.PathAttributeMpReachNLRI); ok {
			return mp.Nexthop
		}
	}

	return netip.Addr{}
}
