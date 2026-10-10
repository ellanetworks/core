// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"maps"
	"net/netip"
	"slices"
	"sync"

	"github.com/ellanetworks/core/internal/models"
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
	localSwitch  bool
	classified   bool
	filtered     bool
	written      *ebpf.Classifier
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
	PdrID      uint32
	PdrInfo    ebpf.PdrInfo
	TeID       uint32
	ChooseID   uint8
	UEIP       netip.Addr
	Precedence uint32
	SDF        []models.SDFFilter
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

func (s *Session) SetLocalSwitch(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.localSwitch = on
}

func (s *Session) SetClassified(classified bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := s.classified != classified
	s.classified = classified

	return changed
}

func (s *Session) classifierWritten(c ebpf.Classifier) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.written != nil && slices.Equal(s.written.Rules, c.Rules) && slices.Equal(s.written.Targets, c.Targets)
}

func (s *Session) setWrittenClassifier(c *ebpf.Classifier) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.written = c
}

func (s *Session) SetFiltered(filtered bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := s.filtered != filtered
	s.filtered = filtered

	return changed
}

func (s *Session) pdrFlags(spdrInfo SPDRInfo) uint8 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var flags uint8

	if s.localSwitch {
		flags |= ebpf.PdrFlagLocalSwitch
	}

	if spdrInfo.UEIP.IsValid() && s.classified || !spdrInfo.UEIP.IsValid() && len(spdrInfo.SDF) > 0 {
		flags |= ebpf.PdrFlagSDF
	}

	if !spdrInfo.UEIP.IsValid() && len(spdrInfo.SDF) == 0 && spdrInfo.TeID != 0 && s.tunnelClassifiedLocked(spdrInfo.TeID) {
		flags |= ebpf.PdrFlagFallback
	}

	return flags
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

func (s *Session) LookupPDR(id uint32) (SPDRInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.pdrs[id]

	return info, ok
}

func (s *Session) RemovePDR(id uint32) (SPDRInfo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, ok := s.pdrs[id]
	delete(s.pdrs, id)

	return info, ok
}

func (s *Session) clearPDRs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pdrs = map[uint32]SPDRInfo{}
}

func (s *Session) RemoveFar(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.fars, id)
}

func (s *Session) RemoveQer(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.qers, id)
}

func (s *Session) ListPDRs() map[uint32]SPDRInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := make(map[uint32]SPDRInfo, len(s.pdrs))
	maps.Copy(c, s.pdrs)

	return c
}

func (s *Session) chosenTEID(chooseID uint8) uint32 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.pdrs {
		if p.ChooseID == chooseID && p.TeID != 0 {
			return p.TeID
		}
	}

	return 0
}

func (s *Session) holdsTEID(teid uint32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.pdrs {
		if p.TeID == teid && !p.UEIP.IsValid() {
			return true
		}
	}

	return false
}

func (s *Session) tunnelEntry(teid uint32) (SPDRInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var (
		entry SPDRInfo
		found bool
	)

	for _, p := range s.pdrs {
		if p.TeID != teid || p.UEIP.IsValid() {
			continue
		}

		if !found || tunnelEntryBefore(p, entry) {
			entry, found = p, true
		}
	}

	return entry, found
}

func (s *Session) tunnelClassifiedLocked(teid uint32) bool {
	for _, p := range s.pdrs {
		if p.TeID == teid && !p.UEIP.IsValid() && len(p.SDF) > 0 {
			return true
		}
	}

	return false
}

func tunnelEntryBefore(a, b SPDRInfo) bool {
	if (len(a.SDF) == 0) != (len(b.SDF) == 0) {
		return len(a.SDF) == 0
	}

	return a.PdrID < b.PdrID
}

func (s *Session) findFAR(match func(ebpf.FarInfo) bool) (uint32, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for id, far := range s.fars {
		if match(far) {
			return id, true
		}
	}

	return 0, false
}

func (s *Session) ListFARs() map[uint32]ebpf.FarInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := make(map[uint32]ebpf.FarInfo, len(s.fars))
	maps.Copy(c, s.fars)

	return c
}

func (s *Session) GetQer(id uint32) ebpf.QerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.qers[id]
}

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
