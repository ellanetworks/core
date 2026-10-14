// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package enginetest

import (
	"fmt"
	"net/netip"
	"sync"

	"github.com/cilium/ebpf"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
)

type urrKey struct {
	seid uint64
	id   uint32
}

type Maps struct {
	mu       sync.Mutex
	uplink   map[uint32]upfebpf.PdrInfo
	downlink map[netip.Addr]upfebpf.PdrInfo
	framed   map[netip.Prefix]netip.Addr
	urrs     map[urrKey]struct{}
	sdf      map[uint32]upfebpf.SdfFilterList
	notified map[upfebpf.DataNotification]struct{}
}

func NewMaps() *Maps {
	return &Maps{
		uplink:   map[uint32]upfebpf.PdrInfo{},
		downlink: map[netip.Addr]upfebpf.PdrInfo{},
		framed:   map[netip.Prefix]netip.Addr{},
		urrs:     map[urrKey]struct{}{},
		sdf:      map[uint32]upfebpf.SdfFilterList{},
		notified: map[upfebpf.DataNotification]struct{}{},
	}
}

func (m *Maps) PutPdrUplink(teid uint32, pdrInfo upfebpf.PdrInfo) error {
	if _, err := upfebpf.ToN3N6EntrypointPdrInfo(pdrInfo); err != nil {
		return fmt.Errorf("build uplink PDR: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.uplink[teid] = pdrInfo

	return nil
}

func (m *Maps) PutPdrDownlink(addr netip.Addr, pdrInfo upfebpf.PdrInfo) error {
	if _, err := upfebpf.ToN3N6EntrypointPdrInfo(pdrInfo); err != nil {
		return fmt.Errorf("build downlink PDR: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.downlink[addr] = pdrInfo

	return nil
}

func (m *Maps) DeletePdrUplink(teid uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.uplink[teid]; !ok {
		return ebpf.ErrKeyNotExist
	}

	delete(m.uplink, teid)

	return nil
}

func (m *Maps) DeletePdrDownlink(addr netip.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.downlink[addr]; !ok {
		return ebpf.ErrKeyNotExist
	}

	delete(m.downlink, addr)

	return nil
}

func (m *Maps) PutFramedDownlink(prefix netip.Prefix, ueAddr netip.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.framed[prefix.Masked()] = ueAddr

	return nil
}

func (m *Maps) DeleteFramedDownlink(prefix netip.Prefix) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	prefix = prefix.Masked()

	if _, ok := m.framed[prefix]; !ok {
		return ebpf.ErrKeyNotExist
	}

	delete(m.framed, prefix)

	return nil
}

func (m *Maps) NewUrr(seid uint64, id uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.urrs[urrKey{seid, id}] = struct{}{}

	return nil
}

func (m *Maps) DeleteUrr(seid uint64, id uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.urrs, urrKey{seid, id})

	return nil
}

func (m *Maps) PutSdfFilterList(index uint32, list upfebpf.SdfFilterList) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sdf[index] = list

	return nil
}

func (m *Maps) DeleteSdfFilterList(index uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sdf, index)

	return nil
}

func (m *Maps) MarkNotified(d upfebpf.DataNotification) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.notified[d] = struct{}{}
}

func (m *Maps) IsAlreadyNotified(d upfebpf.DataNotification) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.notified[d]

	return ok
}

func (m *Maps) ClearNotified(seid uint64, pdrid uint16) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for d := range m.notified {
		if d.LocalSEID == seid && d.PdrID == pdrid {
			delete(m.notified, d)
		}
	}
}

func (m *Maps) ClearNotifiedForSEID(seid uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for d := range m.notified {
		if d.LocalSEID == seid {
			delete(m.notified, d)
		}
	}
}

func (m *Maps) UplinkPDR(teid uint32) (upfebpf.PdrInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.uplink[teid]

	return p, ok
}

func (m *Maps) DownlinkPDR(ueIP netip.Addr) (upfebpf.PdrInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.downlink[ueIP]

	return p, ok
}

func (m *Maps) FramedRoute(prefix netip.Prefix) (netip.Addr, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.framed[prefix.Masked()]

	return a, ok
}

func (m *Maps) SDFList(index uint32) (upfebpf.SdfFilterList, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	l, ok := m.sdf[index]

	return l, ok
}

func (m *Maps) Counts() (uplink, downlink, framed, urrs int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.uplink), len(m.downlink), len(m.framed), len(m.urrs)
}
