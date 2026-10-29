// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"net/netip"
	"sync"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

type Peers struct {
	mu        sync.RWMutex
	ids       map[netip.Addr]uint16
	addrs     []netip.Addr
	exhausted bool
}

func NewPeers() *Peers {
	return &Peers{
		ids:   make(map[netip.Addr]uint16),
		addrs: []netip.Addr{{}},
	}
}

func (p *Peers) ID(addr netip.Addr) uint16 {
	if !addr.IsValid() || addr.IsUnspecified() {
		return 0
	}

	addr = addr.Unmap()

	p.mu.RLock()
	id, ok := p.ids[addr]
	p.mu.RUnlock()

	if ok {
		return id
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if id, ok := p.ids[addr]; ok {
		return id
	}

	if len(p.addrs) >= ebpf.MaxPeers {
		if !p.exhausted {
			p.exhausted = true

			logger.UpfLog.Warn("GTP-U peer table is full; new radios are not counted per radio",
				zap.Int("capacity", ebpf.MaxPeers-1))
		}

		return 0
	}

	id = uint16(len(p.addrs))
	p.addrs = append(p.addrs, addr)
	p.ids[addr] = id

	return id
}

func (p *Peers) All() map[uint16]netip.Addr {
	p.mu.RLock()
	defer p.mu.RUnlock()

	out := make(map[uint16]netip.Addr, len(p.addrs)-1)

	for id, addr := range p.addrs[1:] {
		out[uint16(id+1)] = addr
	}

	return out
}
