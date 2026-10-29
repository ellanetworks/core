// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"maps"
	"net"
	"net/netip"
	"slices"
	"sync"

	"github.com/ellanetworks/core/internal/radioctx"
)

type radioAddresses struct {
	mu     sync.RWMutex
	byAddr map[netip.Addr]string
}

func (r *radioAddresses) recordSwitched(ctx context.Context, prev, next dataPlane) {
	id, ok := radioctx.From(ctx)
	if !ok {
		return
	}

	var switched []AnchorBinding

	if next.AN.bound() && !next.AN.equal(prev.AN) {
		switched = append(switched, next.AN)
	}

	for _, b := range next.Bearers {
		if !b.AN.bound() {
			continue
		}

		if i := slices.IndexFunc(prev.Bearers, func(p bearerLeg) bool { return p.Slot == b.Slot }); i >= 0 && b.AN.equal(prev.Bearers[i].AN) {
			continue
		}

		switched = append(switched, b.AN)
	}

	if len(switched) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.byAddr == nil {
		r.byAddr = make(map[netip.Addr]string)
	}

	for _, an := range switched {
		for _, ip := range []net.IP{an.IPv4, an.IPv6} {
			if addr, ok := netip.AddrFromSlice(ip); ok {
				r.byAddr[addr.Unmap()] = id
			}
		}
	}
}

func (s *SMF) RadioIDsByAddress() map[netip.Addr]string {
	s.radios.mu.RLock()
	defer s.radios.mu.RUnlock()

	return maps.Clone(s.radios.byAddr)
}
