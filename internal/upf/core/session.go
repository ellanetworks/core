// Copyright 2024 Ella Networks
package core

import (
	"maps"
	"net"
	"sync"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

type Session struct {
	mu   sync.RWMutex
	SEID uint64
	pdrs map[uint32]SPDRInfo
	fars map[uint32]ebpf.FarInfo
	qers map[uint32]ebpf.QerInfo
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
	Ipv4      net.IP
	Ipv6      net.IP
	Allocated bool
}

func (s *Session) NewFar(id uint32, farInfo ebpf.FarInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.fars[id] = farInfo
}

func (s *Session) UpdateFar(id uint32, farInfo ebpf.FarInfo) {
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

func (s *Session) NewQer(id uint32, qerInfo ebpf.QerInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.qers[id] = qerInfo
}

func (s *Session) UpdateQer(id uint32, qerInfo ebpf.QerInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.qers[id] = qerInfo
}

func (s *Session) GetQer(id uint32) ebpf.QerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.qers[id]
}

func (s *Session) RemoveQer(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.qers, id)
}

func (s *Session) PutPDR(id uint32, info SPDRInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pdrs[id] = info
}

func (s *Session) GetPDR(id uint16) SPDRInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.pdrs[uint32(id)]
}

func (s *Session) HasPDR(id uint32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.pdrs[id]

	return ok
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

// ListQERs returns a snapshot copy of the QER map.
func (s *Session) ListQERs() map[uint32]ebpf.QerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := make(map[uint32]ebpf.QerInfo, len(s.qers))
	maps.Copy(c, s.qers)

	return c
}
