// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"time"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

// NAT conntrack timeout classes, evaluated on the UE-side entry. Entries are
// reaped by a sweep on natGCInterval, so an entry outlives its class by up to
// one interval.
// TCP established satisfies RFC 5382 REQ-5 (>= 2 h 4 min); half-closed keeps
// a connection alive while one side still sends; UDP replied satisfies RFC
// 4787 REQ-5 (>= 2 min, 5 min recommended); an unreplied UDP flow gets a
// short timeout so probe traffic cannot pin entries; ICMP follows RFC 5508
// REQ-2.
const (
	natTCPEstablishedTimeout = 7440 * time.Second
	natTCPTransitoryTimeout  = 120 * time.Second
	natTCPHalfClosedTimeout  = 120 * time.Second
	natTCPClosedTimeout      = 10 * time.Second
	natUDPRepliedTimeout     = 300 * time.Second
	natUDPUnrepliedTimeout   = 30 * time.Second
	natICMPTimeout           = 60 * time.Second

	// Covers the window between a pair's two inserts and gives the repair
	// paths time to re-create an evicted partner.
	natOrphanGrace = 60 * time.Second
)

const (
	natStateEstablished = 1

	natClosedUE     = 0x1
	natClosedRemote = 0x2
	natClosedBoth   = natClosedUE | natClosedRemote

	protoICMP = 1
	protoTCP  = 6
	protoUDP  = 17
)

func natEntryTimeout(proto uint16, state, replied, closed uint8) time.Duration {
	switch proto {
	case protoTCP:
		switch {
		case closed == natClosedBoth:
			return natTCPClosedTimeout
		case closed != 0:
			return natTCPHalfClosedTimeout
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

// natExpiredKeys returns the nat_ct keys to delete: expired connections
// (decided on the authoritative UE-side entry, both keys together) and, when
// complete is set, NAT-side orphans whose partner is missing or points
// elsewhere. A partial snapshot cannot distinguish an orphan from an entry
// the scan missed, so orphan reaping is skipped and left to the next sweep.
func natExpiredKeys(snapshot map[ebpf.N3N6EntrypointFiveTuple]ebpf.N3N6EntrypointNatEntry, nowNs uint64, complete bool) []ebpf.N3N6EntrypointFiveTuple {
	toDelete := make(map[ebpf.N3N6EntrypointFiveTuple]struct{})

	expired := func(refreshTs uint64, timeout time.Duration) bool {
		return refreshTs+uint64(timeout.Nanoseconds()) < nowNs
	}

	for key, entry := range snapshot {
		if entry.UeSide != 0 {
			if expired(entry.RefreshTs, natEntryTimeout(key.Proto, entry.State, entry.Replied, entry.Closed)) {
				toDelete[key] = struct{}{}
				toDelete[entry.Src] = struct{}{}
			}

			continue
		}

		if !complete {
			continue
		}

		partner, ok := snapshot[entry.Src]
		if (!ok || partner.Src != key) && expired(entry.RefreshTs, natOrphanGrace) {
			toDelete[key] = struct{}{}
		}
	}

	keys := make([]ebpf.N3N6EntrypointFiveTuple, 0, len(toDelete))
	for key := range toDelete {
		keys = append(keys, key)
	}

	return keys
}
