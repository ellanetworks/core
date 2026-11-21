// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func receiveInto(socks *gtpuSockets) <-chan GTPUDatagram {
	ch := make(chan GTPUDatagram, 16)

	socks.setReceiver(func(d GTPUDatagram) {
		d.Payload = bytes.Clone(d.Payload)
		ch <- d
	})

	return ch
}

func awaitDatagram(t *testing.T, ch <-chan GTPUDatagram) GTPUDatagram {
	t.Helper()

	select {
	case d := <-ch:
		return d
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a GTP-U datagram")
	}

	return GTPUDatagram{}
}

func TestGTPUSocketsDeliverUnfragmentedDatagram(t *testing.T) {
	local := netip.MustParseAddr("127.0.0.1")

	socks := senderOrSkip(t, local)
	got := receiveInto(socks)

	peer, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("bind a peer: %v", err)
	}

	defer func() { _ = peer.Close() }()

	pdu := endMarkerPDU(0x42)
	if _, err := peer.WriteToUDP(pdu, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: gtpuPort}); err != nil {
		t.Fatalf("write: %v", err)
	}

	d := awaitDatagram(t, got)

	if d.Reassembled {
		t.Error("an unfragmented datagram was reported as reassembled")
	}

	if d.IfIndex != 1 {
		t.Errorf("ifindex = %d, want 1 (lo)", d.IfIndex)
	}

	if d.Local != netip.AddrPortFrom(local, gtpuPort) {
		t.Errorf("local = %s, want %s:%d", d.Local, local, gtpuPort)
	}

	if want := peer.LocalAddr().(*net.UDPAddr).AddrPort(); d.Peer != want {
		t.Errorf("peer = %s, want %s", d.Peer, want)
	}

	if !bytes.Equal(d.Payload, pdu) {
		t.Errorf("payload = %x, want %x", d.Payload, pdu)
	}
}

func TestGTPUSocketsSendCarriesTrafficClass(t *testing.T) {
	local := netip.MustParseAddr("127.0.0.1")
	peerAddr := netip.MustParseAddr("127.0.0.6")

	ln, err := net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.AddrPortFrom(peerAddr, gtpuPort)))
	if err != nil {
		t.Skipf("cannot bind %s:%d: %v", peerAddr, gtpuPort, err)
	}

	defer func() { _ = ln.Close() }()

	raw, err := ln.SyscallConn()
	if err != nil {
		t.Fatalf("syscall conn: %v", err)
	}

	if err := raw.Control(func(fd uintptr) {
		_ = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_RECVTOS, 1)
	}); err != nil {
		t.Fatalf("control: %v", err)
	}

	socks := senderOrSkip(t, local)

	if err := socks.sendTo(local, peerAddr, 0xb8, []byte("payload"), ""); err != nil {
		t.Fatalf("send: %v", err)
	}

	_ = ln.SetReadDeadline(time.Now().Add(3 * time.Second))

	buf := make([]byte, 64)
	oob := make([]byte, 64)

	n, oobn, flags, from, err := ln.ReadMsgUDPAddrPort(buf, oob)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(buf[:n]) != "payload" || flags != 0 || from.Port() != gtpuPort {
		t.Errorf("received %q flags %#x from %s, want \"payload\" from port %d", buf[:n], flags, from, gtpuPort)
	}

	msgs, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		t.Fatalf("parse control: %v", err)
	}

	for _, m := range msgs {
		if m.Header.Level == unix.IPPROTO_IP && m.Header.Type == unix.IP_TOS && len(m.Data) >= 1 {
			if m.Data[0] != 0xb8 {
				t.Errorf("TOS = %#x, want 0xb8", m.Data[0])
			}

			return
		}
	}

	t.Error("no TOS control message on the received datagram")
}

const (
	fragTestDev  = "ellgtpa"
	fragTestPeer = "ellgtpb"
	fragTestMTU  = 1280
)

var (
	fragTestLocal = netip.MustParseAddr("10.99.0.1")
	fragTestRadio = netip.MustParseAddr("10.99.0.9")
)

func requireRoot(t *testing.T) {
	t.Helper()

	if os.Geteuid() == 0 {
		return
	}

	const msg = "creating veth pairs requires root"
	if os.Getenv("EBPF_REQUIRE_PRIVILEGED") != "" {
		t.Fatal(msg)
	}

	t.Skip(msg)
}

func runIP(t *testing.T, args ...string) {
	t.Helper()

	if out, err := exec.CommandContext(context.Background(), "ip", args...).CombinedOutput(); err != nil {
		t.Fatalf("ip %v: %v: %s", args, err, out)
	}
}

func setupFragmentationLink(t *testing.T) (dev, peer *net.Interface) {
	t.Helper()

	requireRoot(t)

	_ = exec.CommandContext(context.Background(), "ip", "link", "del", fragTestDev).Run()

	runIP(t, "link", "add", fragTestDev, "type", "veth", "peer", "name", fragTestPeer)
	t.Cleanup(func() { _ = exec.CommandContext(context.Background(), "ip", "link", "del", fragTestDev).Run() })

	for _, d := range []string{fragTestDev, fragTestPeer} {
		runIP(t, "link", "set", d, "mtu", "1280", "up")
	}

	runIP(t, "addr", "add", fragTestLocal.String()+"/24", "dev", fragTestDev)

	peerIface, err := net.InterfaceByName(fragTestPeer)
	if err != nil {
		t.Fatalf("lookup %s: %v", fragTestPeer, err)
	}

	runIP(t, "neigh", "add", fragTestRadio.String(), "dev", fragTestDev, "lladdr", peerIface.HardwareAddr.String(), "nud", "permanent")

	devIface, err := net.InterfaceByName(fragTestDev)
	if err != nil {
		t.Fatalf("lookup %s: %v", fragTestDev, err)
	}

	return devIface, peerIface
}

func packetSocket(t *testing.T, ifindex int) int {
	t.Helper()

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons16(unix.ETH_P_ALL)))
	if err != nil {
		t.Fatalf("AF_PACKET socket: %v", err)
	}

	t.Cleanup(func() { _ = unix.Close(fd) })

	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Protocol: htons16(unix.ETH_P_ALL), Ifindex: ifindex}); err != nil {
		t.Fatalf("bind AF_PACKET: %v", err)
	}

	tv := unix.NsecToTimeval(int64(200 * time.Millisecond))
	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv)

	return fd
}

func htons16(v uint16) uint16 { return v<<8 | v>>8 }

func ipv4Checksum(h []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(h); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(h[i:]))
	}

	for sum > 0xffff {
		sum = sum&0xffff + sum>>16
	}

	return ^uint16(sum)
}

func ipv4Fragment(src, dst netip.Addr, id uint16, offset int, more bool, payload []byte) []byte {
	pkt := make([]byte, 20+len(payload))
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(len(pkt)))
	binary.BigEndian.PutUint16(pkt[4:6], id)

	frag := uint16(offset / 8)
	if more {
		frag |= 0x2000
	}

	binary.BigEndian.PutUint16(pkt[6:8], frag)
	pkt[8] = 64
	pkt[9] = 17
	s, d := src.As4(), dst.As4()
	copy(pkt[12:16], s[:])
	copy(pkt[16:20], d[:])
	binary.BigEndian.PutUint16(pkt[10:12], ipv4Checksum(pkt[:20]))
	copy(pkt[20:], payload)

	return pkt
}

func TestGTPUSocketsReportKernelReassembledDatagram(t *testing.T) {
	dev, peer := setupFragmentationLink(t)

	socks := &gtpuSockets{}

	t.Cleanup(func() { _ = socks.Close() })

	got := receiveInto(socks)

	if _, err := socks.get(fragTestLocal, ""); err != nil {
		t.Fatalf("bind: %v", err)
	}

	gtp := make([]byte, 2000)
	gtp[0], gtp[1] = 0x30, 0xff
	binary.BigEndian.PutUint16(gtp[2:4], uint16(len(gtp)-8))
	binary.BigEndian.PutUint32(gtp[4:8], 0x1234)

	for i := 8; i < len(gtp); i++ {
		gtp[i] = byte(i)
	}

	udp := make([]byte, 8+len(gtp))
	binary.BigEndian.PutUint16(udp[0:2], gtpuPort)
	binary.BigEndian.PutUint16(udp[2:4], gtpuPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
	copy(udp[8:], gtp)

	const split = 1200

	fd := packetSocket(t, peer.Index)
	sa := &unix.SockaddrLinklayer{Protocol: htons16(unix.ETH_P_IP), Ifindex: peer.Index}

	for _, frag := range [][]byte{
		ipv4Fragment(fragTestRadio, fragTestLocal, 0x77, split, false, udp[split:]),
		ipv4Fragment(fragTestRadio, fragTestLocal, 0x77, 0, true, udp[:split]),
	} {
		frame := make([]byte, 14+len(frag))
		copy(frame[0:6], dev.HardwareAddr)
		copy(frame[6:12], peer.HardwareAddr)
		binary.BigEndian.PutUint16(frame[12:14], unix.ETH_P_IP)
		copy(frame[14:], frag)

		if err := unix.Sendto(fd, frame, 0, sa); err != nil {
			t.Fatalf("inject fragment: %v", err)
		}
	}

	d := awaitDatagram(t, got)

	if !d.Reassembled {
		t.Error("a datagram the kernel reassembled was not reported as such")
	}

	if d.IfIndex != dev.Index {
		t.Errorf("ifindex = %d, want %d (%s)", d.IfIndex, dev.Index, fragTestDev)
	}

	if d.Peer != netip.AddrPortFrom(fragTestRadio, gtpuPort) {
		t.Errorf("peer = %s, want %s:%d", d.Peer, fragTestRadio, gtpuPort)
	}

	if !bytes.Equal(d.Payload, gtp) {
		t.Errorf("payload is %d bytes and differs from the %d-byte G-PDU sent", len(d.Payload), len(gtp))
	}
}

func TestGTPUSocketsFragmentOversizedSend(t *testing.T) {
	_, peer := setupFragmentationLink(t)

	socks := &gtpuSockets{}

	t.Cleanup(func() { _ = socks.Close() })

	capFD := packetSocket(t, peer.Index)

	payload := make([]byte, 2000)
	for i := range payload {
		payload[i] = byte(i)
	}

	if err := socks.sendTo(fragTestLocal, fragTestRadio, 0x28, payload, ""); err != nil {
		t.Fatalf("send: %v", err)
	}

	var (
		reassembled = make([]byte, 8+len(payload))
		received    int
		buf         = make([]byte, 4096)
	)

	deadline := time.Now().Add(2 * time.Second)
	for received < len(reassembled) && time.Now().Before(deadline) {
		n, _, err := unix.Recvfrom(capFD, buf, 0)
		if err != nil || n < 14+20 || binary.BigEndian.Uint16(buf[12:14]) != unix.ETH_P_IP {
			continue
		}

		ip := buf[14:n]
		if !bytes.Equal(ip[16:20], fragTestRadio.AsSlice()) || ip[9] != 17 {
			continue
		}

		if len(ip) > fragTestMTU {
			t.Fatalf("a %d-byte packet left a %d-byte MTU link", len(ip), fragTestMTU)
		}

		if ip[1] != 0x28 {
			t.Errorf("fragment TOS = %#x, want 0x28", ip[1])
		}

		flags := binary.BigEndian.Uint16(ip[6:8])
		if flags&0x4000 != 0 {
			t.Error("a fragment carries DF")
		}

		offset := int(flags&0x1fff) * 8
		data := ip[20:binary.BigEndian.Uint16(ip[2:4])]
		copy(reassembled[offset:], data)
		received += len(data)
	}

	if received != len(reassembled) {
		t.Fatalf("captured %d of %d bytes of the fragmented datagram", received, len(reassembled))
	}

	if !bytes.Equal(reassembled[8:], payload) {
		t.Error("the fragments do not reassemble into the payload sent")
	}
}
