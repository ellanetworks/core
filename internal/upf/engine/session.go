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
	// opMu serializes a whole control-plane operation on this session — modify,
	// delete, and the reconciler's filter propagation — so their compound
	// read-modify-apply sequences never interleave. It is the outermost lock:
	// never acquired while holding conn.mu or filterMu, and only one is held at a
	// time. mu still guards individual field access underneath it.
	opMu    sync.Mutex
	deleted bool // guarded by opMu

	mu           sync.RWMutex
	SEID         uint64
	policyID     string
	pdrs         map[uint32]SPDRInfo
	fars         map[uint32]ebpf.FarInfo
	qers         map[uint32]ebpf.QerInfo
	urrs         map[uint32]struct{}
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
		urrs: map[uint32]struct{}{},
	}
}

type SPDRInfo struct {
	PdrID     uint32
	PdrInfo   ebpf.PdrInfo
	TeID      uint32
	UEIP      netip.Addr
	Allocated bool
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

func (s *Session) RemoveFar(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.fars, id)
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

func (s *Session) HasPDR(id uint32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.pdrs[id]

	return ok
}

// LookupPDR returns the PDR and whether it exists.
func (s *Session) LookupPDR(id uint32) (SPDRInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.pdrs[id]

	return info, ok
}

func (s *Session) RemovePDR(id uint32) SPDRInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	sPdrInfo := s.pdrs[id]
	delete(s.pdrs, id)

	return sPdrInfo
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

// NewQer stores a QER by ID so that future PDR creation can look it up.
func (s *Session) NewQer(id uint32, qerInfo ebpf.QerInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.qers[id] = qerInfo
}

// GetQer returns the QER with the given ID.
func (s *Session) GetQer(id uint32) ebpf.QerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.qers[id]
}

// PutQer updates a QER in the session.
func (s *Session) PutQer(id uint32, qerInfo ebpf.QerInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.qers[id] = qerInfo
}

// RemoveQer removes a QER from the session.
func (s *Session) RemoveQer(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.qers, id)
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

// addURR records a usage reporting rule the session now holds.
func (s *Session) addURR(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.urrs[id] = struct{}{}
}

func (s *Session) removeURR(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.urrs, id)
}

// URRs returns a snapshot copy of the session's usage reporting rule IDs.
func (s *Session) URRs() map[uint32]struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return maps.Clone(s.urrs)
}

// held is what the UPF holds for the session, the left-hand side of a
// reconciliation.
func (s *Session) held() heldSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return heldSession{
		pdrs:         maps.Clone(s.pdrs),
		urrs:         maps.Clone(s.urrs),
		framedRoutes: append([]netip.Prefix(nil), s.framedRoutes...),
		ueIPv4:       s.ueIPv4,
		ueIPv6:       s.ueIPv6,
	}
}

// putRules installs the forwarding and QoS rules the session is to have, along
// with the UE source addresses its uplink PDRs authorise.
func (s *Session) putRules(fars map[uint32]ebpf.FarInfo, qers map[uint32]ebpf.QerInfo, ueIPv4, ueIPv6 netip.Addr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.fars = maps.Clone(fars)
	s.qers = maps.Clone(qers)
	s.ueIPv4 = ueIPv4
	s.ueIPv6 = ueIPv6
}

// sessionSnapshot is a session's whole rule state, so a failed apply can put
// back exactly what the datapath is unwound to.
type sessionSnapshot struct {
	pdrs         map[uint32]SPDRInfo
	fars         map[uint32]ebpf.FarInfo
	qers         map[uint32]ebpf.QerInfo
	urrs         map[uint32]struct{}
	framedRoutes []netip.Prefix
	ueIPv4       netip.Addr
	ueIPv6       netip.Addr
	policyID     string
}

func (s *Session) snapshot() sessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return sessionSnapshot{
		pdrs:         maps.Clone(s.pdrs),
		fars:         maps.Clone(s.fars),
		qers:         maps.Clone(s.qers),
		urrs:         maps.Clone(s.urrs),
		framedRoutes: append([]netip.Prefix(nil), s.framedRoutes...),
		ueIPv4:       s.ueIPv4,
		ueIPv6:       s.ueIPv6,
		policyID:     s.policyID,
	}
}

func (s *Session) restore(snap sessionSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pdrs = snap.pdrs
	s.fars = snap.fars
	s.qers = snap.qers
	s.urrs = snap.urrs
	s.framedRoutes = snap.framedRoutes
	s.ueIPv4 = snap.ueIPv4
	s.ueIPv6 = snap.ueIPv6
	s.policyID = snap.policyID
}
