// Copyright 2024 Ella Networks
package core

import (
	"net"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

type Session struct {
	SEID uint64
	PDRs map[uint32]SPDRInfo
	FARs map[uint32]ebpf.FarInfo
	QERs map[uint32]ebpf.QerInfo
}

func NewSession(seid uint64) *Session {
	return &Session{
		SEID: seid,
		PDRs: map[uint32]SPDRInfo{},
		FARs: map[uint32]ebpf.FarInfo{},
		QERs: map[uint32]ebpf.QerInfo{},
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
	s.FARs[id] = farInfo
}

func (s *Session) UpdateFar(id uint32, farInfo ebpf.FarInfo) {
	s.FARs[id] = farInfo
}

func (s *Session) GetFar(id uint32) ebpf.FarInfo {
	return s.FARs[id]
}

func (s *Session) RemoveFar(id uint32) {
	delete(s.FARs, id)
}

func (s *Session) NewQer(id uint32, qerInfo ebpf.QerInfo) {
	s.QERs[id] = qerInfo
}

func (s *Session) UpdateQer(id uint32, qerInfo ebpf.QerInfo) {
	s.QERs[id] = qerInfo
}

func (s *Session) GetQer(id uint32) ebpf.QerInfo {
	return s.QERs[id]
}

func (s *Session) RemoveQer(id uint32) {
	delete(s.QERs, id)
}

func (s *Session) PutPDR(id uint32, info SPDRInfo) {
	s.PDRs[id] = info
}

func (s *Session) GetPDR(id uint16) SPDRInfo {
	return s.PDRs[uint32(id)]
}

func (s *Session) RemovePDR(id uint32) SPDRInfo {
	sPdrInfo := s.PDRs[id]
	delete(s.PDRs, id)

	return sPdrInfo
}
