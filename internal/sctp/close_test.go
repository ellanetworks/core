// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"net"
	"syscall"
	"testing"
	"unsafe"
)

// sctpPRSupported is the SCTP_PR_SUPPORTED socket option (Linux uapi sctp.h).
const sctpPRSupported = 113

// connectLoopbackNoPRSCTP opens a blocking SCTP socket connected to
// 127.0.0.1:port with PR-SCTP negotiation disabled, so the kernel cannot
// silently abandon a policy-flagged send from the remote side before it
// reaches this socket. The caller owns the returned fd.
func connectLoopbackNoPRSCTP(port int) (int, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, syscall.IPPROTO_SCTP)
	if err != nil {
		return -1, err
	}

	// struct sctp_assoc_value{assoc_id, assoc_value}, zeroed: value 0 disables.
	var assocValue [2]uint32
	if err := setsockopt(fd, sctpPRSupported, uintptr(unsafe.Pointer(&assocValue[0])), unsafe.Sizeof(assocValue)); err != nil {
		_ = syscall.Close(fd)
		return -1, err
	}

	sa := &syscall.SockaddrInet4{Port: port}
	copy(sa.Addr[:], net.ParseIP("127.0.0.1").To4())

	if err := syscall.Connect(fd, sa); err != nil {
		_ = syscall.Close(fd)
		return -1, err
	}

	return fd, nil
}

// TestClose_PeerObservesOnlyShutdown verifies the peer's first read after
// Close is the zero-length end-of-association marker, not data.
func TestClose_PeerObservesOnlyShutdown(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29310

	ln := newTestListener(t, port)

	connCh := make(chan *SCTPConn, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			connCh <- nil
			return
		}

		connCh <- conn
	}()

	peerFd, err := connectLoopbackNoPRSCTP(port)
	if err != nil {
		t.Fatalf("connect with PR-SCTP disabled: %v", err)
	}

	t.Cleanup(func() { _ = syscall.Close(peerFd) })

	tv := syscall.Timeval{Sec: 5}
	if err := syscall.SetsockoptTimeval(peerFd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
		t.Fatalf("set receive timeout: %v", err)
	}

	conn := <-connCh
	if conn == nil {
		t.Fatal("Accept failed")
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	buf := make([]byte, 256)
	oob := make([]byte, 256)

	n, oobn, _, _, err := syscall.Recvmsg(peerFd, buf, oob, 0)
	if err != nil {
		t.Fatalf("peer read after Close: %v", err)
	}

	if n != 0 {
		t.Fatalf("peer read after Close: got %d data bytes (%x), want zero-length end-of-association read", n, buf[:n])
	}

	if oobn != 0 {
		t.Fatalf("peer read after Close: got %d ancillary bytes, want zero-length end-of-association read", oobn)
	}
}
