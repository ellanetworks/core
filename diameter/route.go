// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
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
}

type route struct {
	entries []routeEntry
	next    atomic.Uint32
}

type RouteStatus struct {
	Realm       string
	Application uint32
	Peers       []RoutePeerStatus
}

type RoutePeerStatus struct {
	ID       string
	Priority int
	State    PeerState
}

func (n *Node) rebuildRoutesLocked() {
	routes := make(map[routeKey]*route)

	for _, p := range n.byID {
		for _, r := range p.cfg.Routes {
			key := routeKey{realm: strings.ToLower(r.Realm), app: r.Application}

			rt, ok := routes[key]
			if !ok {
				rt = &route{}
				routes[key] = rt
			}

			rt.entries = append(rt.entries, routeEntry{peer: p, priority: r.Priority})
		}
	}

	for _, rt := range routes {
		slices.SortFunc(rt.entries, func(a, b routeEntry) int {
			return cmp.Or(cmp.Compare(a.priority, b.priority), strings.Compare(a.peer.id, b.peer.id))
		})
	}

	n.routes = routes
}

func (rt *route) ordered() []*peer {
	out := make([]*peer, 0, len(rt.entries))
	turn := int(rt.next.Add(1) - 1)

	for start := 0; start < len(rt.entries); {
		end := start
		for end < len(rt.entries) && rt.entries[end].priority == rt.entries[start].priority {
			end++
		}

		group := rt.entries[start:end]
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

	statuses := make([]RouteStatus, 0, len(n.routes))

	for key, rt := range n.routes {
		s := RouteStatus{Realm: key.realm, Application: key.app}

		for _, e := range rt.entries {
			s.Peers = append(s.Peers, RoutePeerStatus{ID: e.peer.id, Priority: e.priority, State: e.peer.state()})
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
