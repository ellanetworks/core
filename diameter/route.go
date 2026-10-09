// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

type Route struct {
	Realm       string
	Application uint32
	Priority    int
}

type routeKey struct {
	realm string
	app   uint32
}

type routeEntry struct {
	peer     *peer
	priority int
	implicit bool
}

type RouteStatus struct {
	Realm       string
	Application uint32
	Peers       []RoutePeerStatus
}

type RoutePeerStatus struct {
	ID       string
	Host     string
	Priority int
	State    PeerState
	Implicit bool
}

func (n *Node) rebuildRoutesLocked() {
	routes := make(map[routeKey][]routeEntry)

	for _, p := range n.byID {
		for _, r := range p.cfg.Routes {
			key := routeKey{realm: strings.ToLower(r.Realm), app: r.Application}
			routes[key] = append(routes[key], routeEntry{peer: p, priority: r.Priority})
		}
	}

	for _, entries := range routes {
		slices.SortFunc(entries, func(a, b routeEntry) int {
			return cmp.Or(cmp.Compare(a.priority, b.priority), strings.Compare(a.peer.id, b.peer.id))
		})
	}

	n.routes = routes
	n.pruneRotationLocked()
}

func (n *Node) implicitEntriesLocked(key routeKey) []routeEntry {
	var entries []routeEntry

	for _, p := range n.byHost {
		if p.cfg != nil || p.removed || p.open == nil || !strings.EqualFold(p.lastRealm, key.realm) || !p.open.supports(key.app) {
			continue
		}

		entries = append(entries, routeEntry{peer: p, implicit: true})
	}

	slices.SortFunc(entries, func(a, b routeEntry) int { return strings.Compare(a.peer.host, b.peer.host) })

	return entries
}

func (n *Node) routeEntriesLocked(key routeKey, implicit bool) []routeEntry {
	if key.realm == "" {
		return nil
	}

	entries := slices.Clone(n.routes[key])
	if implicit {
		entries = append(entries, n.implicitEntriesLocked(key)...)
	}

	return entries
}

func (n *Node) implicitKeysLocked() map[routeKey]bool {
	keys := make(map[routeKey]bool)

	for _, p := range n.byHost {
		if p.cfg != nil || p.removed || p.open == nil {
			continue
		}

		for app := range p.open.common {
			keys[routeKey{realm: strings.ToLower(p.lastRealm), app: app}] = true
		}
	}

	return keys
}

func (n *Node) pruneRotationLocked() {
	if len(n.rotation) == 0 {
		return
	}

	implicit := n.implicitKeysLocked()

	for key := range n.rotation {
		if _, static := n.routes[key]; !static && !implicit[key] {
			delete(n.rotation, key)
		}
	}
}

func (n *Node) orderedLocked(key routeKey, entries []routeEntry) []*peer {
	turn := int(n.rotation[key])
	n.rotation[key]++

	out := make([]*peer, 0, len(entries))

	for start := 0; start < len(entries); {
		end := start
		for end < len(entries) && entries[end].implicit == entries[start].implicit && entries[end].priority == entries[start].priority {
			end++
		}

		group := entries[start:end]
		for i := range group {
			out = append(out, group[(turn+i)%len(group)].peer)
		}

		start = end
	}

	return out
}

func (n *Node) Routes() []RouteStatus {
	n.mu.Lock()
	defer n.mu.Unlock()

	keys := n.implicitKeysLocked()
	for key := range n.routes {
		keys[key] = true
	}

	statuses := make([]RouteStatus, 0, len(keys))

	for key := range keys {
		s := RouteStatus{Realm: key.realm, Application: key.app}

		for _, e := range n.routeEntriesLocked(key, true) {
			s.Peers = append(s.Peers, RoutePeerStatus{
				ID:       e.peer.id,
				Host:     e.peer.lastHost,
				Priority: e.priority,
				State:    e.peer.state(),
				Implicit: e.implicit,
			})
		}

		statuses = append(statuses, s)
	}

	slices.SortFunc(statuses, func(a, b RouteStatus) int {
		return cmp.Or(strings.Compare(a.Realm, b.Realm), cmp.Compare(a.Application, b.Application))
	})

	return statuses
}

func validateRoutes(p Peer) error {
	seen := make(map[routeKey]bool)

	for _, r := range p.Routes {
		key := routeKey{realm: strings.ToLower(r.Realm), app: r.Application}

		switch {
		case !isDiameterIdentity(r.Realm):
			return fmt.Errorf("diameter: peer %q has a route with an invalid realm %q", p.ID, r.Realm)
		case seen[key]:
			return fmt.Errorf("diameter: peer %q lists the route for %s and application %d twice", p.ID, r.Realm, r.Application)
		case !slices.ContainsFunc(p.Applications, func(a Application) bool { return a.ID == r.Application || a.ID == RelayApplicationID }):
			return fmt.Errorf("diameter: peer %q routes application %d it does not advertise", p.ID, r.Application)
		}

		seen[key] = true
	}

	return nil
}
