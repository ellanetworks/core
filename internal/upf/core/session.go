// Copyright 2024 Ella Networks
package core

import (
	"net"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

type Session struct {
	LocalSEID  uint64
	RemoteSEID uint64
	PDRs       map[uint32]SPDRInfo
	FARs       map[uint32]SFarInfo
	QERs       map[uint32]SQerInfo
}

func NewSession(localSEID uint64, remoteSEID uint64) *Session {
	return &Session{
		LocalSEID:  localSEID,
		RemoteSEID: remoteSEID,
		PDRs:       map[uint32]SPDRInfo{},
		FARs:       map[uint32]SFarInfo{},
		QERs:       map[uint32]SQerInfo{},
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

type SFarInfo struct {
	FarInfo  ebpf.FarInfo
	GlobalID uint32
}

type SQerInfo struct {
	QerInfo  ebpf.QerInfo
	GlobalID uint32
}

func (s *Session) NewFar(id uint32, internalID uint32, farInfo ebpf.FarInfo) {
	s.FARs[id] = SFarInfo{
		FarInfo:  farInfo,
		GlobalID: internalID,
	}
}

func (s *Session) UpdateFar(id uint32, farInfo ebpf.FarInfo) {
	sFarInfo := s.FARs[id]
	sFarInfo.FarInfo = farInfo
	s.FARs[id] = sFarInfo
}

func (s *Session) GetFar(id uint32) SFarInfo {
	return s.FARs[id]
}

func (s *Session) RemoveFar(id uint32) SFarInfo {
	sFarInfo := s.FARs[id]
	delete(s.FARs, id)
	return sFarInfo
}

func (s *Session) NewQer(id uint32, internalID uint32, qerInfo ebpf.QerInfo) {
	s.QERs[id] = SQerInfo{
		QerInfo:  qerInfo,
		GlobalID: internalID,
	}
}

func (s *Session) UpdateQer(id uint32, qerInfo ebpf.QerInfo) {
	sQerInfo := s.QERs[id]
	sQerInfo.QerInfo = qerInfo
	s.QERs[id] = sQerInfo
}

func (s *Session) GetQer(id uint32) SQerInfo {
	return s.QERs[id]
}

func (s *Session) RemoveQer(id uint32) SQerInfo {
	sQerInfo := s.QERs[id]
	delete(s.QERs, id)
	return sQerInfo
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
