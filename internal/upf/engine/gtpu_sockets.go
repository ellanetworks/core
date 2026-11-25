// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/netutil"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

const (
	gtpuReadBuffer = 4 << 20

	gtpuMaxDatagram = 65535

	gtpuControlBuffer = 128
)

type GTPUDatagram struct {
	Local       netip.AddrPort
	Peer        netip.AddrPort
	IfIndex     int
	Reassembled bool
	Payload     []byte
}

type GTPUReceiver func(GTPUDatagram)

type gtpuSockets struct {
	mu       sync.Mutex
	socks    map[netip.Addr]*net.UDPConn
	closed   bool
	receiver atomic.Pointer[GTPUReceiver]
	readers  sync.WaitGroup
}

func (s *gtpuSockets) setReceiver(r GTPUReceiver) {
	if r == nil {
		s.receiver.Store(nil)
		return
	}

	s.receiver.Store(&r)
}

func (s *gtpuSockets) get(local netip.Addr, device string) (*net.UDPConn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, fmt.Errorf("the GTP-U sockets are closed")
	}

	if sock, ok := s.socks[local]; ok {
		return sock, nil
	}

	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			if device != "" {
				if err := netutil.BindToDeviceControl(device)(network, address, c); err != nil {
					return err
				}
			}

			return configureGTPUSocket(c, local.Is4())
		},
	}

	conn, err := lc.ListenPacket(context.Background(), "udp", net.UDPAddrFromAddrPort(netip.AddrPortFrom(local, gtpuPort)).String())
	if err != nil {
		return nil, fmt.Errorf("bind %s for GTP-U: %w", local, err)
	}

	sock, ok := conn.(*net.UDPConn)
	if !ok {
		_ = conn.Close()

		return nil, fmt.Errorf("bind %s for GTP-U: not a UDP socket", local)
	}

	if err := sock.SetReadBuffer(gtpuReadBuffer); err != nil {
		logger.UpfLog.Warn("could not size the GTP-U socket's receive buffer",
			zap.String("local", local.String()), zap.Error(err))
	}

	if s.socks == nil {
		s.socks = make(map[netip.Addr]*net.UDPConn, 2)
	}

	s.socks[local] = sock

	s.readers.Add(1)

	go func() {
		defer s.readers.Done()

		s.serve(sock, netip.AddrPortFrom(local, gtpuPort))
	}()

	return sock, nil
}

func configureGTPUSocket(c syscall.RawConn, v4 bool) error {
	var opErr error

	err := c.Control(func(fd uintptr) {
		if v4 {
			opErr = errors.Join(
				unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DONT),
				unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_PKTINFO, 1),
				unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_RECVFRAGSIZE, 1),
			)

			return
		}

		opErr = errors.Join(
			unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_MTU_DISCOVER, unix.IPV6_PMTUDISC_DONT),
			unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_RECVPKTINFO, 1),
			unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_RECVFRAGSIZE, 1),
		)
	})
	if err != nil {
		return err
	}

	if opErr != nil {
		return fmt.Errorf("configure GTP-U socket: %w", opErr)
	}

	return nil
}

func (s *gtpuSockets) serve(sock *net.UDPConn, local netip.AddrPort) {
	buf := make([]byte, gtpuMaxDatagram)
	oob := make([]byte, gtpuControlBuffer)

	for {
		n, oobn, flags, peer, err := sock.ReadMsgUDPAddrPort(buf, oob)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}

			continue
		}

		receiver := s.receiver.Load()
		if receiver == nil || flags&unix.MSG_TRUNC != 0 {
			continue
		}

		d := GTPUDatagram{
			Local:   local,
			Peer:    netip.AddrPortFrom(peer.Addr().Unmap(), peer.Port()),
			Payload: buf[:n],
		}

		d.IfIndex, d.Reassembled = parseGTPUControl(oob[:oobn])

		(*receiver)(d)
	}
}

func parseGTPUControl(oob []byte) (ifindex int, reassembled bool) {
	msgs, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return 0, false
	}

	for _, m := range msgs {
		switch {
		case m.Header.Level == unix.IPPROTO_IP && m.Header.Type == unix.IP_PKTINFO && len(m.Data) >= unix.SizeofInet4Pktinfo:
			ifindex = int(int32(binary.NativeEndian.Uint32(m.Data[0:4])))
		case m.Header.Level == unix.IPPROTO_IPV6 && m.Header.Type == unix.IPV6_PKTINFO && len(m.Data) >= unix.SizeofInet6Pktinfo:
			ifindex = int(int32(binary.NativeEndian.Uint32(m.Data[16:20])))
		case m.Header.Level == unix.IPPROTO_IP && m.Header.Type == unix.IP_RECVFRAGSIZE,
			m.Header.Level == unix.IPPROTO_IPV6 && m.Header.Type == unix.IPV6_RECVFRAGSIZE:
			reassembled = true
		}
	}

	return ifindex, reassembled
}

func (s *gtpuSockets) sendTo(local netip.Addr, peer netip.Addr, tclass uint8, payload []byte, device string) error {
	if !local.IsValid() || local.Is4() != peer.Is4() {
		return fmt.Errorf("no local address matching peer %s to send GTP-U from", peer)
	}

	sock, err := s.get(local, device)
	if err != nil {
		return err
	}

	var oob []byte
	if tclass != 0 {
		oob = trafficClassControl(local.Is4(), tclass)
	}

	if _, _, err := sock.WriteMsgUDPAddrPort(payload, oob, netip.AddrPortFrom(peer, gtpuPort)); err != nil {
		return fmt.Errorf("send GTP-U to %s: %w", peer, err)
	}

	return nil
}

func trafficClassControl(v4 bool, tclass uint8) []byte {
	oob := make([]byte, unix.CmsgSpace(4))
	h := (*unix.Cmsghdr)(unsafe.Pointer(&oob[0]))

	if v4 {
		h.Level = unix.IPPROTO_IP
		h.Type = unix.IP_TOS
	} else {
		h.Level = unix.IPPROTO_IPV6
		h.Type = unix.IPV6_TCLASS
	}

	h.SetLen(unix.CmsgLen(4))
	binary.NativeEndian.PutUint32(oob[unix.CmsgLen(0):], uint32(tclass))

	return oob
}

func (s *gtpuSockets) Close() error {
	s.mu.Lock()

	s.closed = true

	var firstErr error

	for local, sock := range s.socks {
		if err := sock.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close GTP-U socket %s: %w", local, err)
		}

		delete(s.socks, local)
	}

	s.mu.Unlock()

	s.readers.Wait()

	return firstErr
}

func (conn *SessionEngine) ServeGTPU(receiver GTPUReceiver) error {
	if conn.gtpu == nil {
		return fmt.Errorf("no GTP-U sockets")
	}

	conn.gtpu.setReceiver(receiver)

	if receiver == nil {
		return nil
	}

	device := conn.N3VRFDevice()

	var errs []error

	for _, local := range []netip.Addr{conn.n3AddressIPv4, conn.n3AddressIPv6} {
		if !local.IsValid() {
			continue
		}

		if _, err := conn.gtpu.get(local, device); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (conn *SessionEngine) SendGTPU(local netip.Addr, peer netip.Addr, tclass uint8, payload []byte) error {
	if conn.gtpu == nil {
		return fmt.Errorf("no GTP-U sockets")
	}

	return conn.gtpu.sendTo(local, peer, tclass, payload, conn.N3VRFDevice())
}
