// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"
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
