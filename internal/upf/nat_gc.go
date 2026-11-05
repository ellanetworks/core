// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"time"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

// NAT conntrack timeout classes, evaluated on the UE-side entry. Floors:
// RFC 5382 REQ-5 (established >= 2 h 4 min, partially open >= 4 min; §5 keeps
// a half-closed connection established), RFC 5508 REQ-2 (ICMP), RFC 4787
// REQ-5 (UDP with a reply). Closed TCP and unreplied UDP sit below their
// floor to bound the entries a connection-cycling subscriber holds.
const (
	natTCPEstablishedTimeout = 7440 * time.Second
	natTCPTransitoryTimeout  = 240 * time.Second
	natTCPClosedTimeout      = 10 * time.Second
	natUDPRepliedTimeout     = 300 * time.Second
	natUDPUnrepliedTimeout   = 30 * time.Second
	natICMPTimeout           = 60 * time.Second

	// Covers the window between a pair's two inserts.
	natOrphanGrace = 60 * time.Second
)

const (
	natStateEstablished = 1

	natClosed = 0x1

	protoICMP = 1
	protoTCP  = 6
	protoUDP  = 17
)

func natEntryTimeout(proto uint16, state, replied, closed uint8) time.Duration {
	switch proto {
	case protoTCP:
		switch {
		case closed != 0:
			return natTCPClosedTimeout
		case state == natStateEstablished:
			return natTCPEstablishedTimeout
		default:
			return natTCPTransitoryTimeout
		}
	case protoUDP:
		if replied != 0 {
			return natUDPRepliedTimeout
		}

		return natUDPUnrepliedTimeout
	case protoICMP:
		return natICMPTimeout
	default:
		return ConnTrackTimeout
	}
}

type natLookup func(ebpf.N3N6EntrypointFiveTuple) (ebpf.N3N6EntrypointNatEntry, bool)

func natExpiredKeys(keys []ebpf.N3N6EntrypointFiveTuple, values []ebpf.N3N6EntrypointNatEntry, lookup natLookup, nowNs uint64) []ebpf.N3N6EntrypointFiveTuple {
	var toDelete []ebpf.N3N6EntrypointFiveTuple

	seen := make(map[ebpf.N3N6EntrypointFiveTuple]struct{})

	add := func(k ebpf.N3N6EntrypointFiveTuple) {
		if _, ok := seen[k]; ok {
			return
		}

		seen[k] = struct{}{}
		toDelete = append(toDelete, k)
	}

	expired := func(refreshTs uint64, timeout time.Duration) bool {
		return refreshTs+uint64(timeout.Nanoseconds()) < nowNs
	}

	for i, key := range keys {
		entry := values[i]

		if entry.UeSide != 0 {
			if expired(entry.RefreshTs, natEntryTimeout(key.Proto, entry.State, entry.Replied, entry.Closed)) {
				add(key)

				if partner, ok := lookup(entry.Peer); ok && partner.Peer == key {
					add(entry.Peer)
				}
			}

			continue
		}

		if !expired(entry.RefreshTs, natOrphanGrace) {
			continue
		}

		if partner, ok := lookup(entry.Peer); !ok || partner.Peer != key {
			add(key)
		}
	}

	return toDelete
}

const (
	natGCMinInterval      = 10 * time.Second
	natGCMaxInterval      = 5 * time.Minute
	natGCIntervalRounding = time.Second
)

func nextNATGCInterval(prev time.Duration, deleteRatio float64) time.Duration {
	switch {
	case deleteRatio > 0.25:
		deleteRatio = min(deleteRatio, 0.9)
		return max(time.Duration(float64(prev)*(1-deleteRatio)).Round(natGCIntervalRounding), natGCMinInterval)
	case deleteRatio < 0.05:
		return min(time.Duration(float64(prev)*1.5).Round(natGCIntervalRounding), natGCMaxInterval)
	}

	return prev
}
