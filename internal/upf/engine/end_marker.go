// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

const (
	gtpuPort = 2152

	gtpuVersion1        = 0x20
	gtpuProtocolTypeGTP = 0x10
	gtpuMsgEndMarker    = 0xfe

	endMarkersPerSwitch = 1
)

type endMarkerTarget struct {
	teid  uint32
	local netip.Addr
	peer  netip.Addr
}

func endMarkerTargetFor(old ebpf.FarInfo, next ebpf.FarInfo) (endMarkerTarget, bool) {
	if old.OuterHeaderCreation == 0 || old.TeID == 0 {
		return endMarkerTarget{}, false
	}

	peer := ebpf.In6AddrToIP(old.RemoteIP).Unmap()
	if !peer.IsValid() || peer.IsUnspecified() {
		return endMarkerTarget{}, false
	}

	if next.OuterHeaderCreation == 0 || next.TeID == 0 {
		return endMarkerTarget{}, false
	}

	if old.TeID == next.TeID && old.RemoteIP == next.RemoteIP {
		return endMarkerTarget{}, false
	}

	return endMarkerTarget{
		teid:  old.TeID,
		local: ebpf.In6AddrToIP(old.LocalIP).Unmap(),
		peer:  peer,
	}, true
}

func appendEndMarkerTarget(targets []endMarkerTarget, target endMarkerTarget) []endMarkerTarget {
	if slices.Contains(targets, target) {
		return targets
	}

	return append(targets, target)
}

func endMarkerPDU(teid uint32) []byte {
	pdu := make([]byte, 8)
	pdu[0] = gtpuVersion1 | gtpuProtocolTypeGTP
	pdu[1] = gtpuMsgEndMarker
	binary.BigEndian.PutUint16(pdu[2:4], 0)
	binary.BigEndian.PutUint32(pdu[4:8], teid)

	return pdu
}

type endMarkerSender interface {
	send(t endMarkerTarget, count int, device string) error
	Close() error
}

func (s *gtpuSockets) send(t endMarkerTarget, count int, device string) error {
	pdu := endMarkerPDU(t.teid)

	for range count {
		if err := s.sendTo(t.local, t.peer, 0, pdu, device); err != nil {
			return fmt.Errorf("send End Marker: %w", err)
		}
	}

	return nil
}

func (conn *SessionEngine) sendEndMarkers(targets []endMarkerTarget) {
	if len(targets) == 0 || conn.endMarkers == nil {
		return
	}

	if q := conn.downlinkQueue(); q != nil {
		if err := conn.WaitForDatapath(); err != nil {
			logger.UpfLog.Warn("failed to wait for the datapath before sending End Markers", zap.Error(err))
		}

		q.Flush()
	}

	device := conn.N3VRFDevice()

	for _, t := range targets {
		if err := conn.endMarkers.send(t, endMarkersPerSwitch, device); err != nil {
			logger.UpfLog.Warn("failed to send GTP-U End Marker",
				logger.TEID(t.teid), zap.String("peer", t.peer.String()), zap.Error(err))

			continue
		}

		logger.UpfLog.Info("sent GTP-U End Marker",
			logger.TEID(t.teid), zap.String("peer", t.peer.String()))
	}
}
