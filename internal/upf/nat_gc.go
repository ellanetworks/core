// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"time"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

// NAT conntrack timeout classes, evaluated on the UE-side entry.
// TCP established satisfies RFC 5382 REQ-5 (>= 2 h 4 min); UDP replied
// satisfies RFC 4787 REQ-5 (>= 2 min, 5 min recommended); an unreplied UDP
// flow gets a short timeout so probe traffic cannot pin entries; ICMP
// follows RFC 5508 REQ-2.
const (
	natTCPEstablishedTimeout = 7440 * time.Second
	natTCPTransitoryTimeout  = 120 * time.Second
	natTCPClosingTimeout     = 10 * time.Second
	natUDPRepliedTimeout     = 300 * time.Second
	natUDPUnrepliedTimeout   = 30 * time.Second
	natICMPTimeout           = 60 * time.Second

	// Covers the window between a pair's two inserts and gives the uplink
	// repair path time to re-create an evicted partner.
	natOrphanGrace = 60 * time.Second
)

const (
	natStateEstablished = 1
	natStateClosing     = 2

	protoICMP = 1
	protoTCP  = 6
	protoUDP  = 17
)

func natEntryTimeout(proto uint16, state, replied uint8) time.Duration {
	switch proto {
	case protoTCP:
		switch state {
		case natStateClosing:
			return natTCPClosingTimeout
		case natStateEstablished:
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
// (decided on the authoritative UE-side entry, both keys together) and
// NAT-side orphans whose partner is missing or points elsewhere, once past
// the grace period.
func natExpiredKeys(snapshot map[ebpf.N3N6EntrypointFiveTuple]ebpf.N3N6EntrypointNatEntry, nowNs uint64) []ebpf.N3N6EntrypointFiveTuple {
	toDelete := make(map[ebpf.N3N6EntrypointFiveTuple]struct{})

	expired := func(refreshTs uint64, timeout time.Duration) bool {
		return refreshTs+uint64(timeout.Nanoseconds()) < nowNs
	}

	for key, entry := range snapshot {
		if entry.UeSide != 0 {
			if expired(entry.RefreshTs, natEntryTimeout(key.Proto, entry.State, entry.Replied)) {
				toDelete[key] = struct{}{}
				toDelete[entry.Src] = struct{}{}
			}

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
