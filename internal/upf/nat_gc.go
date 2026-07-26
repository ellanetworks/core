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
//
// RFC 5382 §5 places FIN_WAIT_1, FIN_WAIT_2 and CLOSE_WAIT in the established
// phase — after one FIN the peer may still send indefinitely — so a
// half-closed connection keeps the established timeout. REQ-5 sets the floors:
// established >= 2 h 4 min, partially open >= 4 min. ICMP follows RFC 5508
// REQ-2, UDP with a reply RFC 4787 REQ-5 (>= 2 min, 5 min recommended).
//
// Two classes fall below their floor deliberately, matching netfilter:
//   - Both directions closed: only final-ACK retransmissions can still
//     arrive, and RFC 5382 §5 contemplates a NAT reaping such a session
//     early. Holding a mapping for 4 min after every close would multiply
//     the entries a connection-cycling subscriber occupies.
//   - UDP with no reply yet: 4787 REQ-5 exempts only well-known destination
//     ports, but scan and probe traffic would otherwise pin an entry per
//     destination for the full 5 minutes.
const (
	natTCPEstablishedTimeout = 7440 * time.Second
	natTCPTransitoryTimeout  = 240 * time.Second
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

				// The NAT tuple may have been re-reserved by
				// another subscriber since; only the partner
				// still pointing back belongs to this pair.
				if partner, ok := snapshot[entry.Src]; ok && partner.Src == key {
					toDelete[entry.Src] = struct{}{}
				}
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
