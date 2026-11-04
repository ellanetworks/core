// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestDialerAndPrimaryPeerAddr(t *testing.T) {
	var lc ListenConfig

	ln, err := lc.Listen(context.Background(), &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.IPv4(127, 0, 0, 1)}}})
	if err != nil {
		t.Skipf("SCTP not available: %v", err)
	}

	defer func() { _ = ln.Close() }()

	accepted := make(chan *SCTPConn, 1)

	go func() {
		c, err := ln.Accept()
		if err == nil {
			accepted <- c
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	d := Dialer{
		LocalAddr: &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.IPv4(127, 0, 0, 2)}}},
		InitMsg:   InitMsg{NumOstreams: 2, MaxInstreams: 2, MaxAttempts: 4, MaxInitTimeout: 3000},
	}

	client, err := d.Dial(ctx, ln.Addr().(*SCTPAddr))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	defer func() { _ = client.Close() }()

	var server *SCTPConn

	select {
	case server = <-accepted:
	case <-ctx.Done():
		t.Fatal("no association accepted")
	}

	defer func() { _ = server.Close() }()

	if err := server.PrepareAccepted(nil); err != nil {
		t.Fatalf("PrepareAccepted: %v", err)
	}

	primary, err := server.PrimaryPeerAddr()
	if err != nil || primary != netip.MustParseAddr("127.0.0.2") {
		t.Fatalf("PrimaryPeerAddr = %v, %v", primary, err)
	}

	if client.writeCh == nil || server.writeCh == nil {
		t.Fatal("queued writer not started")
	}

	if _, err := client.WriteMsg([]byte("ping"), &SndRcvInfo{PPID: PPIDWireOrder(46)}); err != nil {
		t.Fatalf("WriteMsg: %v", err)
	}

	buf := make([]byte, MaxMessageSize)

	n, info, err := server.ReadMsg(buf)
	if err != nil || string(buf[:n]) != "ping" || PPIDWireOrder(info.PPID) != 46 {
		t.Fatalf("ReadMsg = %q, %+v, %v", buf[:n], info, err)
	}
}

func TestParseSockaddrStorage(t *testing.T) {
	v4 := make([]byte, sockaddrStorageSize)
	v4[0] = 2
	copy(v4[4:], []byte{10, 1, 2, 3})

	if a, err := parseSockaddrStorage(v4); err != nil || a != netip.MustParseAddr("10.1.2.3") {
		t.Fatalf("v4 = %v, %v", a, err)
	}

	v6 := make([]byte, sockaddrStorageSize)
	v6[0] = 10
	copy(v6[8:], netip.MustParseAddr("::ffff:10.1.2.3").AsSlice())

	if a, err := parseSockaddrStorage(v6); err != nil || a != netip.MustParseAddr("10.1.2.3") {
		t.Fatalf("mapped v6 = %v, %v", a, err)
	}

	if _, err := parseSockaddrStorage(make([]byte, sockaddrStorageSize)); err == nil {
		t.Fatal("unknown family accepted")
	}
}

func TestDialerControlBindsTheSocketBeforeConnecting(t *testing.T) {
	var lc ListenConfig

	ln, err := lc.Listen(context.Background(), &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.IPv4(127, 0, 0, 1)}}})
	if err != nil {
		t.Skipf("SCTP not available: %v", err)
	}

	defer func() { _ = ln.Close() }()

	go func() {
		if c, err := ln.Accept(); err == nil {
			_ = c.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	raddr, ok := ln.Addr().(*SCTPAddr)
	if !ok {
		t.Fatalf("listener address is %T", ln.Addr())
	}

	var gotNetwork, gotAddress string

	d := Dialer{
		InitMsg: InitMsg{NumOstreams: 2, MaxInstreams: 2, MaxAttempts: 4, MaxInitTimeout: 3000},
		Control: func(network, address string, c syscall.RawConn) error {
			gotNetwork, gotAddress = network, address

			var setErr error

			if err := c.Control(func(fd uintptr) {
				setErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, "lo")
			}); err != nil {
				return err
			}

			return setErr
		},
	}

	client, err := d.Dial(ctx, raddr)
	if errors.Is(err, syscall.EPERM) {
		t.Skipf("SO_BINDTODEVICE requires CAP_NET_RAW: %v", err)
	}

	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	defer func() { _ = client.Close() }()

	if gotNetwork != "sctp" || gotAddress != raddr.String() {
		t.Fatalf("Control called with %q, %q; want sctp, %q", gotNetwork, gotAddress, raddr.String())
	}

	var (
		device [syscall.IFNAMSIZ]byte
		size   = uint32(len(device))
	)

	if err := client.controlFd(func(fd int) error {
		_, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT, uintptr(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE,
			uintptr(unsafe.Pointer(&device[0])), uintptr(unsafe.Pointer(&size)), 0)
		if errno != 0 {
			return errno
		}

		return nil
	}); err != nil {
		t.Fatalf("getsockopt SO_BINDTODEVICE: %v", err)
	}

	if got := strings.TrimRight(string(device[:size]), "\x00"); got != "lo" {
		t.Fatalf("association bound to %q, want lo", got)
	}
}

func TestDialerControlErrorFailsTheDial(t *testing.T) {
	want := errors.New("control refused")

	d := Dialer{Control: func(string, string, syscall.RawConn) error { return want }}

	_, err := d.Dial(context.Background(), &SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.IPv4(127, 0, 0, 1)}}, Port: 9})
	if errors.Is(err, syscall.EPROTONOSUPPORT) || errors.Is(err, syscall.ESOCKTNOSUPPORT) {
		t.Skipf("SCTP not available: %v", err)
	}

	if !errors.Is(err, want) {
		t.Fatalf("Dial error = %v, want %v", err, want)
	}
}
