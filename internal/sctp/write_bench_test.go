// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"fmt"
	"net"
	"syscall"
	"testing"
)

// The write queue adds a payload copy, a channel send and a goroutine wakeup per
// message; the sendmsg syscall itself is unchanged, only moved off the caller.
// These measure that added cost at NGAP-representative sizes.
func BenchmarkWriteQueueOverhead(b *testing.B) {
	for _, size := range []int{256, 1024, 8192, 65536} {
		payload := make([]byte, size)
		info := &SndRcvInfo{PPID: PPIDWireOrder(testPPID)}

		b.Run(fmt.Sprintf("copy+enqueue/%dB", size), func(b *testing.B) {
			ch := make(chan queuedWrite, writeQueueDepth)
			done := make(chan struct{})

			go func() {
				defer close(done)

				for range ch {
				}
			}()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				qw := queuedWrite{b: append([]byte(nil), payload...)}
				cp := *info
				qw.info = &cp

				ch <- qw
			}

			b.StopTimer()
			close(ch)
			<-done
		})

		b.Run(fmt.Sprintf("handoff_only/%dB", size), func(b *testing.B) {
			ch := make(chan queuedWrite, writeQueueDepth)
			done := make(chan struct{})

			go func() {
				defer close(done)

				for range ch {
				}
			}()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ch <- queuedWrite{b: payload, info: info}
			}

			b.StopTimer()
			close(ch)
			<-done
		})
	}
}

// The sendmsg the queue defers, measured on a real association with a draining
// peer, so the added cost above can be read as a fraction of it.
func BenchmarkWriteMsgSyncSyscall(b *testing.B) {
	const port = 29441

	netAddr, err := net.ResolveIPAddr("ip", "127.0.0.1")
	if err != nil {
		b.Fatal(err)
	}

	cfg := socketConfig{
		InitMsg: InitMsg{NumOstreams: 2, MaxInstreams: 5, MaxAttempts: 2, MaxInitTimeout: 2},
		Control: func(_, _ string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, 4<<20)
			})
		},
	}

	ln, err := cfg.Listen("sctp", &SCTPAddr{IPAddrs: []net.IPAddr{*netAddr}, Port: port})
	if err != nil {
		b.Skipf("listen: %v", err)
	}

	defer func() { _ = ln.Close() }()

	accepts := make(chan *SCTPConn, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			accepts <- nil
			return
		}

		accepts <- conn
	}()

	clientFd, err := connectLoopback(port)
	if err != nil {
		b.Fatalf("connect: %v", err)
	}

	defer func() { _ = syscall.Close(clientFd) }()

	conn := <-accepts
	if conn == nil {
		b.Fatal("accept failed")
	}

	defer func() { _ = conn.Close() }()

	go func() {
		buf := make([]byte, 1<<16)
		for {
			if _, err := syscall.Read(clientFd, buf); err != nil {
				return
			}
		}
	}()

	payload := make([]byte, 1024)
	info := &SndRcvInfo{PPID: PPIDWireOrder(testPPID)}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if _, err := conn.writeMsgSync(payload, info); err != nil {
			b.Fatalf("writeMsgSync: %v", err)
		}
	}
}
