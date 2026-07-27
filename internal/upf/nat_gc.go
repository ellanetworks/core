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

// natExpiredKeys returns the nat_ct keys to delete; a partial snapshot cannot
// distinguish an orphan from a missed entry, so orphans are reaped only when
// complete.
func natExpiredKeys(snapshot map[ebpf.N3N6EntrypointFiveTuple]ebpf.N3N6EntrypointNatEntry, nowNs uint64, complete bool) []ebpf.N3N6EntrypointFiveTuple {
	toDelete := make(map[ebpf.N3N6EntrypointFiveTuple]struct{})

	expired := func(refreshTs uint64, timeout time.Duration) bool {
		return refreshTs+uint64(timeout.Nanoseconds()) < nowNs
	}

	for key, entry := range snapshot {
		if entry.UeSide != 0 {
			if expired(entry.RefreshTs, natEntryTimeout(key.Proto, entry.State, entry.Replied, entry.Closed)) {
				toDelete[key] = struct{}{}

				// The NAT tuple may have been re-reserved by
				// another subscriber since; only the partner
				// still pointing back belongs to this pair.
				if partner, ok := snapshot[entry.Peer]; ok && partner.Peer == key {
					toDelete[entry.Peer] = struct{}{}
				}
			}

			continue
		}

		if !complete {
			continue
		}

		partner, ok := snapshot[entry.Peer]
		if (!ok || partner.Peer != key) && expired(entry.RefreshTs, natOrphanGrace) {
			toDelete[key] = struct{}{}
		}
	}

	keys := make([]ebpf.N3N6EntrypointFiveTuple, 0, len(toDelete))
	for key := range toDelete {
		keys = append(keys, key)
	}

	return keys
}
