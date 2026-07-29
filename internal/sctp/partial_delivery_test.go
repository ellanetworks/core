// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"bytes"
	"net"
	"syscall"
	"testing"
	"time"
)

// newSmallBufferListener shrinks SO_RCVBUF before bind. An association's rwnd is
// derived from the listening socket when it is created (net/sctp/associola.c
// sctp_association_init), so shrinking it after accept would be too late.
func newSmallBufferListener(t *testing.T, port, rcvbuf int) *sctpListener {
	t.Helper()

	netAddr, err := net.ResolveIPAddr("ip", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	cfg := socketConfig{
		InitMsg: InitMsg{NumOstreams: 2, MaxInstreams: 5, MaxAttempts: 2, MaxInitTimeout: 2},
		Control: func(_, _ string, c syscall.RawConn) error {
			var serr error

			if err := c.Control(func(fd uintptr) {
				serr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, rcvbuf)
			}); err != nil {
				return err
			}

			return serr
		},
	}

	ln, err := cfg.Listen("sctp", &SCTPAddr{IPAddrs: []net.IPAddr{*netAddr}, Port: port})
	if err != nil {
		t.Fatalf("listen :%d: %v", port, err)
	}

	t.Cleanup(func() { _ = ln.Close() })

	return ln
}

// splitPair returns the two ends of one association whose receive buffer is small
// enough that the kernel splits a large message across deliveries. The sender is a
// raw fd so writes are not routed through the write queue.
func splitPair(t *testing.T, port, rcvbuf int) (server *SCTPConn, clientFd int) {
	t.Helper()

	skipIfNoSCTP(t)

	ln := newSmallBufferListener(t, port, rcvbuf)

	type accepted struct {
		conn *SCTPConn
		err  error
	}

	accepts := make(chan accepted, 1)

	go func() {
		conn, err := ln.Accept()
		accepts <- accepted{conn: conn, err: err}
	}()

	fd, err := connectLoopback(port)
	if err != nil {
		t.Fatalf("connectLoopback: %v", err)
	}

	t.Cleanup(func() { _ = syscall.Close(fd) })

	select {
	case a := <-accepts:
		if a.err != nil {
			t.Fatalf("accept: %v", a.err)
		}

		server = a.conn
	case <-time.After(10 * time.Second):
		t.Fatal("no association accepted")
	}

	t.Cleanup(func() { _ = server.Close() })

	if err := server.subscribeEvents(sctpEventDataIO); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := server.setReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatalf("setReadDeadline: %v", err)
	}

	return server, fd
}

const splitMsgSize = 96 * 1024

func splitPayload() []byte {
	payload := make([]byte, splitMsgSize)
	for i := range payload {
		payload[i] = byte(i % 251)
	}

	return payload
}

// A message the kernel splits across deliveries must come back from readMsg whole
// and byte-exact. This drives the real kernel path rather than a modelled reader.
func TestReadMsg_KernelSplitMessageIsReassembled(t *testing.T) {
	server, clientFd := splitPair(t, 29421, 4096)
	payload := splitPayload()

	go func() {
		for sent := 0; sent < len(payload); {
			n, err := syscall.Write(clientFd, payload[sent:])
			if err != nil {
				return
			}

			sent += n
		}
	}()

	buf := make([]byte, 256*1024)

	n, _, notification, err := server.readMsg(buf)
	if err != nil {
		t.Fatalf("readMsg: %v", err)
	}

	if notification != nil {
		t.Fatalf("expected payload, got notification %T", notification)
	}

	if n != splitMsgSize {
		t.Fatalf("expected %d bytes, got %d", splitMsgSize, n)
	}

	if !bytes.Equal(buf[:n], payload) {
		t.Fatal("reassembled message does not match what was sent")
	}
}

// Guards the test above against passing vacuously: it only proves reassembly if
// the kernel really delivers the message in pieces on this host.
func TestReadMsgOnce_KernelSplitsLargeMessage(t *testing.T) {
	server, clientFd := splitPair(t, 29422, 4096)
	payload := splitPayload()

	go func() {
		for sent := 0; sent < len(payload); {
			n, err := syscall.Write(clientFd, payload[sent:])
			if err != nil {
				return
			}

			sent += n
		}
	}()

	buf := make([]byte, 256*1024)

	d, err := server.readMsgOnce(buf)
	if err != nil {
		t.Fatalf("readMsgOnce: %v", err)
	}

	t.Logf("first delivery: n=%d eor=%v of %d bytes", d.n, d.eor, splitMsgSize)

	if d.eor {
		t.Fatalf("kernel completed a %d-byte message in one delivery with a %d-byte receive buffer; "+
			"the reassembly path is not being exercised", splitMsgSize, 4096)
	}

	total := d.n

	for !d.eor {
		d, err = server.readMsgOnce(buf[total:])
		if err != nil {
			t.Fatalf("readMsgOnce (continuation): %v", err)
		}

		total += d.n
	}

	if total != splitMsgSize {
		t.Fatalf("fragments summed to %d, expected %d", total, splitMsgSize)
	}
}
