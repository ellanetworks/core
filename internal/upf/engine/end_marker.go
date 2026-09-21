// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/netutil"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

const (
	gtpuPort = 2152

	gtpuVersion1        = 0x20
	gtpuProtocolTypeGTP = 0x10
	gtpuMsgEndMarker    = 0xfe

	endMarkersPerSwitch = 1

	endMarkerReadBuffer = 2048
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

func endMarkerPDU(teid uint32) []byte {
	pdu := make([]byte, 8)
	pdu[0] = gtpuVersion1 | gtpuProtocolTypeGTP
	pdu[1] = gtpuMsgEndMarker
	binary.BigEndian.PutUint16(pdu[2:4], 0)
	binary.BigEndian.PutUint32(pdu[4:8], teid)

	return pdu
}

type endMarkerSockets struct {
	mu     sync.Mutex
	socks  map[netip.Addr]*net.UDPConn
	closed bool
}

func (s *endMarkerSockets) get(local netip.Addr, device string) (*net.UDPConn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, fmt.Errorf("the End Marker sockets are closed")
	}

	if sock, ok := s.socks[local]; ok {
		return sock, nil
	}

	lc := net.ListenConfig{}

	if device != "" {
		lc.Control = netutil.BindToDeviceControl(device)
	}

	conn, err := lc.ListenPacket(context.Background(), "udp", net.UDPAddrFromAddrPort(netip.AddrPortFrom(local, gtpuPort)).String())
	if err != nil {
		return nil, fmt.Errorf("bind %s for GTP-U End Markers: %w", local, err)
	}

	sock, ok := conn.(*net.UDPConn)
	if !ok {
		_ = conn.Close()

		return nil, fmt.Errorf("bind %s for GTP-U End Markers: not a UDP socket", local)
	}

	if err := sock.SetReadBuffer(endMarkerReadBuffer); err != nil {
		logger.UpfLog.Warn("could not shrink the End Marker socket's receive buffer",
			zap.String("local", local.String()), zap.Error(err))
	}

	go discardInbound(sock)

	if s.socks == nil {
		s.socks = make(map[netip.Addr]*net.UDPConn, 2)
	}

	s.socks[local] = sock

	return sock, nil
}

func discardInbound(sock *net.UDPConn) {
	buf := make([]byte, 1)

	for {
		if _, _, err := sock.ReadFromUDP(buf); err != nil {
			return
		}
	}
}

func (s *endMarkerSockets) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true

	var firstErr error

	for local, sock := range s.socks {
		if err := sock.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close End Marker socket %s: %w", local, err)
		}

		delete(s.socks, local)
	}

	return firstErr
}

func (s *endMarkerSockets) send(t endMarkerTarget, count int, device string) error {
	local := t.local
	if !local.IsValid() || local.Is4() != t.peer.Is4() {
		return fmt.Errorf("no local address matching peer %s to source an End Marker from", t.peer)
	}

	sock, err := s.get(local, device)
	if err != nil {
		return err
	}

	pdu := endMarkerPDU(t.teid)
	to := net.UDPAddrFromAddrPort(netip.AddrPortFrom(t.peer, gtpuPort))

	for range count {
		if _, err := sock.WriteToUDP(pdu, to); err != nil {
			return fmt.Errorf("send End Marker to %s: %w", to, err)
		}
	}

	return nil
}

func (conn *SessionEngine) sendEndMarkers(targets []endMarkerTarget) {
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
