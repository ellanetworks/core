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
	framedRoutes []netip.Prefix
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
