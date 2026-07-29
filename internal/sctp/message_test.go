// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"context"
	"syscall"
	"testing"
	"time"

	"go.uber.org/zap"
)

// sendOneMessage sends size bytes as a single SCTP message on the given PPID.
func sendOneMessage(t *testing.T, fd int, size int) {
	t.Helper()

	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 1<<20); err != nil {
		t.Fatalf("set peer send buffer: %v", err)
	}

	info := SndRcvInfo{PPID: PPIDWireOrder(testPPID)}
	cmsgBuf := toBuf(&info)
	hdr := &syscall.Cmsghdr{Level: syscall.IPPROTO_SCTP, Type: SCTPCMsgSndRcv}
	hdr.SetLen(syscall.CmsgLen(len(cmsgBuf)))

	if _, err := syscall.SendmsgN(fd, make([]byte, size), append(toBuf(hdr), cmsgBuf...), nil, 0); err != nil {
		t.Fatalf("peer sendmsg %d bytes: %v", size, err)
	}
}

// serverCollecting starts a server recording the size of every dispatched message.
func serverCollecting(t *testing.T, port int, dispatch func(msg []byte)) (int, chan struct{}) {
	t.Helper()

	skipIfNoSCTP(t)

	disconnected := make(chan struct{})

	srv := NewServer(Config{PPID: testPPID, Name: "TEST", Logger: zap.NewNop()}, Callbacks{
		Dispatch: func(_ context.Context, _ *SCTPConn, msg []byte) { dispatch(msg) },
		OnDisconnect: func(_ *SCTPConn) {
			select {
			case <-disconnected:
			default:
				close(disconnected)
			}
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := srv.ListenAndServe(ctx, "127.0.0.1", port, ""); err != nil {
		t.Fatalf("ListenAndServe: %v", err)
	}

	t.Cleanup(func() {
		sctx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()

		srv.Shutdown(sctx)
	})

	fd, err := connectLoopback(port)
	if err != nil {
		t.Fatalf("connectLoopback: %v", err)
	}

	t.Cleanup(func() { _ = syscall.Close(fd) })

	return fd, disconnected
}

// A message the kernel splits across deliveries (it does so under receive-buffer
// pressure) must be reassembled and dispatched once, not as several PDUs.
func TestServer_LargeMessageDispatchedOnce(t *testing.T) {
	const size = 100 * 1024

	sizes := make(chan int, 16)

	fd, _ := serverCollecting(t, 29421, func(msg []byte) { sizes <- len(msg) })

	sendOneMessage(t, fd, size)

	select {
	case n := <-sizes:
		if n != size {
			t.Fatalf("dispatched %d bytes, want the whole %d-byte message", n, size)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("message never dispatched")
	}

	select {
	case n := <-sizes:
		t.Fatalf("message was split: a second dispatch of %d bytes", n)
	case <-time.After(500 * time.Millisecond):
	}
}

// A message larger than the read buffer must abort the association rather than
// dispatch a fragment as if it were whole.
func TestServer_OversizedMessageAbortsAssociation(t *testing.T) {
	sizes := make(chan int, 16)

	fd, disconnected := serverCollecting(t, 29422, func(msg []byte) { sizes <- len(msg) })

	sendOneMessage(t, fd, int(readBufSize)+64*1024)

	select {
	case <-disconnected:
	case <-time.After(10 * time.Second):
		t.Fatal("oversized message did not abort the association")
	}

	select {
	case n := <-sizes:
		t.Fatalf("a fragment of %d bytes was dispatched", n)
	default:
	}
}

// A panic in message handling must be contained to its association.
func TestServer_DispatchPanicDoesNotKillServer(t *testing.T) {
	fd, disconnected := serverCollecting(t, 29423, func(_ []byte) {
		panic("decoder blew up")
	})

	sendOneMessage(t, fd, 64)

	select {
	case <-disconnected:
	case <-time.After(10 * time.Second):
		t.Fatal("panicking dispatch did not close the association")
	}
}
