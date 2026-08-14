// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"maps"
	"net/netip"
	"sync"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

type Session struct {
	opMu    sync.Mutex
	deleted bool // guarded by opMu

	mu           sync.RWMutex
	SEID         uint64
	imsi         string
	policyID     string
	pdrs         map[uint32]SPDRInfo
	fars         map[uint32]ebpf.FarInfo
	qers         map[uint32]ebpf.QerInfo
	framedRoutes []netip.Prefix
	ueIPv4       netip.Addr
	ueIPv6       netip.Addr
}

func NewSession(seid uint64) *Session {
	return &Session{
		SEID: seid,
		pdrs: map[uint32]SPDRInfo{},
		fars: map[uint32]ebpf.FarInfo{},
		qers: map[uint32]ebpf.QerInfo{},
	}
}

type SPDRInfo struct {
	PdrID   uint32
	PdrInfo ebpf.PdrInfo
	TeID    uint32
	UEIP    netip.Addr
}

func (s *Session) PolicyID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.policyID
}

func (s *Session) SetPolicyID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.policyID = id
}

func (s *Session) IMSI() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.imsi
}

func (s *Session) SetIMSI(imsi string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.imsi = imsi
}

func (s *Session) PutFar(id uint32, farInfo ebpf.FarInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.fars[id] = farInfo
}

func (s *Session) GetFar(id uint32) ebpf.FarInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.fars[id]
}

func (s *Session) PutPDR(id uint32, info SPDRInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pdrs[id] = info
}

func (s *Session) GetPDR(id uint32) SPDRInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.pdrs[id]
}

// LookupPDR returns the PDR and whether it exists.
func (s *Session) LookupPDR(id uint32) (SPDRInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.pdrs[id]

	return info, ok
}

// ListPDRs returns a snapshot copy of the PDR map.
func (s *Session) ListPDRs() map[uint32]SPDRInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := make(map[uint32]SPDRInfo, len(s.pdrs))
	maps.Copy(c, s.pdrs)

	return c
}

// ListFARs returns a snapshot copy of the FAR map.
func (s *Session) ListFARs() map[uint32]ebpf.FarInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := make(map[uint32]ebpf.FarInfo, len(s.fars))
	maps.Copy(c, s.fars)

	return c
}

// GetQer returns the QER with the given ID.
func (s *Session) GetQer(id uint32) ebpf.QerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.qers[id]
}

// PutQer stores a QER by ID so that PDR creation can look it up.
func (s *Session) PutQer(id uint32, qerInfo ebpf.QerInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.qers[id] = qerInfo
}

// SetUEAddresses records the UE source addresses (v4 /32, v6 /64 base) for uplink
// validation; fixed for the session lifetime, so set once at establishment.
func (s *Session) SetUEAddresses(v4, v6 netip.Addr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ueIPv4 = v4
	s.ueIPv6 = v6
}

func (s *Session) UEAddresses() (netip.Addr, netip.Addr) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.ueIPv4, s.ueIPv6
}

// SetFramedRoutes records the session's framed-route prefixes so they can be
// removed from the datapath when the session is deleted.
func (s *Session) SetFramedRoutes(prefixes []netip.Prefix) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.framedRoutes = prefixes
}

// FramedRoutes returns a snapshot copy of the session's framed-route prefixes.
func (s *Session) FramedRoutes() []netip.Prefix {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return append([]netip.Prefix(nil), s.framedRoutes...)
}

// ListQERs returns a snapshot copy of the QER map.
func (s *Session) ListQERs() map[uint32]ebpf.QerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := make(map[uint32]ebpf.QerInfo, len(s.qers))
	maps.Copy(c, s.qers)

	return c
}

// snapshot copies the rule maps so a failed modification can restore them.
func (s *Session) snapshot() (pdrs map[uint32]SPDRInfo, fars map[uint32]ebpf.FarInfo, qers map[uint32]ebpf.QerInfo) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return maps.Clone(s.pdrs), maps.Clone(s.fars), maps.Clone(s.qers)
}

func (s *Session) restore(pdrs map[uint32]SPDRInfo, fars map[uint32]ebpf.FarInfo, qers map[uint32]ebpf.QerInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pdrs = pdrs
	s.fars = fars
	s.qers = qers
}
