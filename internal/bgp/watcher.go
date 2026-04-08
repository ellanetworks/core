package bgp

import (
	"context"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/kernel"
	api "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	"go.uber.org/zap"
)

// ribPollInterval is the interval between RIB polling cycles.
const ribPollInterval = 3 * time.Second

// startRIBPoller periodically syncs kernel routes with the GoBGP global RIB.
// It replaces the event-driven WatchEvent approach, which has an unavoidable
// deadlock on shutdown (see https://github.com/osrg/gobgp/issues/3366).
// It exits when ctx is cancelled.
func (b *BGPService) startRIBPoller(ctx context.Context) {
	ticker := time.NewTicker(ribPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.syncRoutes(ctx)
		}
	}
}

// syncRoutes performs a full reconciliation of kernel routes against the GoBGP
// global RIB. Routes present in the RIB that pass all filters are installed;
// previously learned routes no longer in the RIB are removed.
func (b *BGPService) syncRoutes(ctx context.Context) {
	// TryRLock so we never block a concurrent stopLocked (which holds mu
	// write-lock and waits for the poller to exit). If mu is write-locked we
	// simply skip this cycle; the next tick will retry.
	if !b.mu.TryRLock() {
		return
	}

	if !b.running || !b.routeLearningEnabled() {
		b.mu.RUnlock()

		return
	}

	server := b.server
	peers := b.peers
	filter := b.filter
	ownedPaths := b.paths
	n6Addr := b.n6Addr
	b.mu.RUnlock()

	prefixCache := b.preloadImportPrefixes(ctx, peers)

	if ctx.Err() != nil {
		return
	}

	type ribEntry struct {
		prefix  netip.Prefix
		nextHop netip.Addr
		peer    string
	}

	var ribRoutes []ribEntry

	err := server.ListPath(apiutil.ListPathRequest{
		TableType: api.TableType_TABLE_TYPE_GLOBAL,
		Family:    bgppacket.RF_IPv4_UC,
	}, func(_ bgppacket.NLRI, paths []*apiutil.Path) {
		for _, path := range paths {
			if path.Withdrawal {
				continue
			}

			prefix, ok := extractPrefix(path)
			if !ok {
				continue
			}

			if _, owned := ownedPaths[prefix.Addr().String()]; owned {
				continue
			}

			nextHop := extractNextHop(path)
			if !nextHop.IsValid() || nextHop == n6Addr {
				continue
			}

			if filter.overlapsAny(prefix) {
				continue
			}

			peerAddr := path.PeerAddress.String()
			peerID := findPeerID(peers, peerAddr)

			if peerID == 0 {
				continue
			}

			entries := prefixCache[peerID]
			if len(entries) == 0 || !matchesPrefixList(prefix, entries) {
				continue
			}

			ribRoutes = append(ribRoutes, ribEntry{
				prefix:  prefix,
				nextHop: nextHop,
				peer:    peerAddr,
			})
		}
	})
	if err != nil {
		b.logger.Warn("failed to list global RIB for route sync", zap.Error(err))

		return
	}

	if ctx.Err() != nil {
		return
	}

	ribSet := make(map[string]ribEntry, len(ribRoutes))
	for _, r := range ribRoutes {
		ribSet[r.prefix.String()] = r
	}

	b.learnedMu.Lock()
	defer b.learnedMu.Unlock()

	// Remove routes no longer in the RIB.
	for prefixStr, lr := range b.learnedRoutes {
		if _, inRIB := ribSet[prefixStr]; !inRIB {
			err := b.kernel.DeleteRoute(lr.prefix, lr.gateway, bgpRouteMetric, kernel.N6)
			if err != nil {
				b.logger.Warn("failed to remove withdrawn BGP route",
					zap.String("prefix", prefixStr), zap.Error(err))
			}

			delete(b.learnedRoutes, prefixStr)

			b.logger.Info("removed BGP route (withdrawn)",
				zap.String("prefix", prefixStr), zap.String("peer", lr.peer))
		}
	}

	// Update routes whose gateway or peer changed.
	for prefixStr, lr := range b.learnedRoutes {
		r, ok := ribSet[prefixStr]
		if !ok {
			continue
		}

		if lr.gateway == r.nextHop && lr.peer == r.peer {
			continue
		}

		err := b.kernel.ReplaceRoute(r.prefix, r.nextHop, bgpRouteMetric, kernel.N6)
		if err != nil {
			b.logger.Warn("failed to update BGP route",
				zap.String("prefix", prefixStr), zap.Error(err))

			continue
		}

		b.learnedRoutes[prefixStr] = learnedRoute{
			prefix:  r.prefix,
			gateway: r.nextHop,
			peer:    r.peer,
		}

		b.logger.Info("updated BGP route",
			zap.String("prefix", prefixStr),
			zap.String("nextHop", r.nextHop.String()),
			zap.String("peer", r.peer),
		)
	}

	// Install new routes, respecting per-peer limits.
	peerCounts := make(map[string]int)
	for _, lr := range b.learnedRoutes {
		peerCounts[lr.peer]++
	}

	for _, r := range ribRoutes {
		prefixStr := r.prefix.String()

		if _, exists := b.learnedRoutes[prefixStr]; exists {
			continue
		}

		if peerCounts[r.peer] >= MaxLearnedRoutesPerPeer {
			continue
		}

		err := b.kernel.ReplaceRoute(r.prefix, r.nextHop, bgpRouteMetric, kernel.N6)
		if err != nil {
			b.logger.Warn("failed to install BGP route",
				zap.String("prefix", prefixStr), zap.Error(err))

			continue
		}

		b.learnedRoutes[prefixStr] = learnedRoute{
			prefix:  r.prefix,
			gateway: r.nextHop,
			peer:    r.peer,
		}

		peerCounts[r.peer]++

		b.logger.Info("installed BGP route",
			zap.String("prefix", prefixStr),
			zap.String("nextHop", r.nextHop.String()),
			zap.String("peer", r.peer),
		)
	}
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

		// We don't know the original gateway, but DeleteRoute with an invalid
		// gateway will match on destination + priority + interface.
		err := b.kernel.DeleteRoute(dst, netip.Addr{}, bgpRouteMetric, kernel.N6)
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

		err := b.kernel.DeleteRoute(dst, lr.gateway, bgpRouteMetric, kernel.N6)
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

// removeLearnedRoutesForPeer removes all learned routes from the kernel
// that were advertised by the given peer address and removes them from
// the in-memory map. Called when a peer is removed during reconciliation.
func (b *BGPService) removeLearnedRoutesForPeer(peerAddr string) {
	b.learnedMu.Lock()
	defer b.learnedMu.Unlock()

	removed := 0

	for prefixStr, lr := range b.learnedRoutes {
		if lr.peer != peerAddr {
			continue
		}

		dst := lr.prefix

		err := b.kernel.DeleteRoute(dst, lr.gateway, bgpRouteMetric, kernel.N6)
		if err != nil {
			b.logger.Warn("failed to remove learned BGP route for deleted peer",
				zap.String("prefix", prefixStr), zap.String("peer", peerAddr), zap.Error(err))
		}

		delete(b.learnedRoutes, prefixStr)

		removed++
	}

	if removed > 0 {
		b.logger.Info("removed BGP-learned routes for deleted peer",
			zap.String("peer", peerAddr), zap.Int("count", removed))
	}
}

// reEvaluateLearnedRoutes checks all currently learned routes against the
// current import prefix lists and safety filter, removing any that no longer
// match. Called after hot reconfiguration to enforce import policy changes.
// Must be called with mu held (for peers access).
func (b *BGPService) reEvaluateLearnedRoutes(ctx context.Context, peers []BGPPeer) {
	// Pre-load import prefixes for all peers outside learnedMu to avoid
	// holding the lock during DB queries.
	prefixCache := b.preloadImportPrefixes(ctx, peers)

	b.learnedMu.Lock()
	defer b.learnedMu.Unlock()

	removed := 0

	for prefixStr, lr := range b.learnedRoutes {
		// Check safety filter first.
		dst := lr.prefix

		if b.filter.overlapsAny(dst) {
			err := b.kernel.DeleteRoute(dst, lr.gateway, bgpRouteMetric, kernel.N6)
			if err != nil {
				b.logger.Warn("failed to remove route rejected by updated filter",
					zap.String("prefix", prefixStr), zap.Error(err))
			}

			delete(b.learnedRoutes, prefixStr)

			removed++

			continue
		}

		// Check per-peer import prefix list.
		peerID := findPeerID(peers, lr.peer)
		entries := prefixCache[peerID]

		if len(entries) == 0 || !matchesPrefixList(dst, entries) {
			err := b.kernel.DeleteRoute(dst, lr.gateway, bgpRouteMetric, kernel.N6)
			if err != nil {
				b.logger.Warn("failed to remove route no longer matching import policy",
					zap.String("prefix", prefixStr), zap.Error(err))
			}

			delete(b.learnedRoutes, prefixStr)

			removed++
		}
	}

	if removed > 0 {
		b.logger.Info("re-evaluated learned routes after reconfigure",
			zap.Int("removed", removed), zap.Int("remaining", len(b.learnedRoutes)))
	}
}

// replayGlobalRIB iterates the GoBGP global RIB and installs any routes that
// now pass the import policy but were not previously learned. This handles the
// case where an import policy is widened (e.g. from "default route only" to
// "permit all"), which reEvaluateLearnedRoutes alone cannot handle because it
// only removes routes.
// Must be called with mu held (for server, peers, filter, paths access).
func (b *BGPService) replayGlobalRIB(ctx context.Context, peers []BGPPeer) {
	prefixCache := b.preloadImportPrefixes(ctx, peers)

	b.learnedMu.Lock()
	defer b.learnedMu.Unlock()

	// Build per-peer counts from existing learned routes for limit enforcement.
	peerCounts := make(map[string]int)

	for _, lr := range b.learnedRoutes {
		peerCounts[lr.peer]++
	}

	installed := 0

	err := b.server.ListPath(apiutil.ListPathRequest{
		TableType: api.TableType_TABLE_TYPE_GLOBAL,
		Family:    bgppacket.RF_IPv4_UC,
	}, func(nlri bgppacket.NLRI, paths []*apiutil.Path) {
		for _, path := range paths {
			if path.Withdrawal {
				continue
			}

			prefix, ok := extractPrefix(path)
			if !ok {
				continue
			}

			prefixStr := prefix.String()

			// Skip if already learned.
			if _, exists := b.learnedRoutes[prefixStr]; exists {
				continue
			}

			// Skip locally-originated routes.
			if _, owned := b.paths[prefix.Addr().String()]; owned {
				continue
			}

			nextHop := extractNextHop(path)
			if !nextHop.IsValid() {
				continue
			}

			// Skip routes with our own next-hop.
			if nextHop == b.n6Addr {
				continue
			}

			// Safety filter.
			if b.filter.overlapsAny(prefix) {
				continue
			}

			peerAddr := path.PeerAddress.String()
			peerID := findPeerID(peers, peerAddr)

			if peerID == 0 {
				continue
			}

			entries := prefixCache[peerID]

			if len(entries) == 0 || !matchesPrefixList(prefix, entries) {
				continue
			}

			if peerCounts[peerAddr] >= MaxLearnedRoutesPerPeer {
				continue
			}

			err := b.kernel.ReplaceRoute(prefix, nextHop, bgpRouteMetric, kernel.N6)
			if err != nil {
				b.logger.Warn("failed to install route during RIB replay",
					zap.String("prefix", prefixStr), zap.Error(err))

				continue
			}

			b.learnedRoutes[prefixStr] = learnedRoute{
				prefix:  prefix,
				gateway: nextHop,
				peer:    peerAddr,
			}

			peerCounts[peerAddr]++
			installed++
		}
	})
	if err != nil {
		b.logger.Warn("failed to list global RIB for replay", zap.Error(err))

		return
	}

	if installed > 0 {
		b.logger.Info("installed routes from RIB replay after policy change",
			zap.Int("installed", installed), zap.Int("total", len(b.learnedRoutes)))
	}
}

// preloadImportPrefixes loads and parses import prefix lists for all peers
// before acquiring learnedMu, so DB queries don't block route processing.
func (b *BGPService) preloadImportPrefixes(ctx context.Context, peers []BGPPeer) map[int][]ImportPrefix {
	cache := make(map[int][]ImportPrefix, len(peers))

	for _, p := range peers {
		entries, err := b.loadImportPrefixes(ctx, p.ID)
		if err != nil {
			b.logger.Warn("failed to preload import prefixes",
				zap.String("peer", p.Address), zap.Error(err))

			continue
		}

		cache[p.ID] = entries
	}

	return cache
}

// loadImportPrefixes fetches and parses the import prefix list for a peer.
func (b *BGPService) loadImportPrefixes(ctx context.Context, peerID int) ([]ImportPrefix, error) {
	rawEntries, err := b.importStore.ListImportPrefixes(ctx, peerID)
	if err != nil {
		return nil, err
	}

	entries := make([]ImportPrefix, 0, len(rawEntries))

	for _, raw := range rawEntries {
		network, err := netip.ParsePrefix(raw.Prefix)
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
func extractPrefix(path *apiutil.Path) (netip.Prefix, bool) {
	if path.Nlri == nil {
		return netip.Prefix{}, false
	}

	ipPrefix, ok := path.Nlri.(*bgppacket.IPAddrPrefix)
	if !ok {
		return netip.Prefix{}, false
	}

	return ipPrefix.Prefix, true
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
