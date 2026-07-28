// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"bytes"
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"go.uber.org/zap"
)

// serverWithAcceptedConn starts a Server on port, returns the accepted
// server-side conn (captured from Dispatch after the client sends one trigger
// message) and a channel closed when that conn disconnects.
func serverWithAcceptedConn(t *testing.T, port int) (server *Server, accepted *SCTPConn, disconnected chan struct{}, client *SCTPConn) {
	t.Helper()

	skipIfNoSCTP(t)

	acceptedCh := make(chan *SCTPConn, 1)
	disconnected = make(chan struct{})

	srv := NewServer(Config{PPID: testPPID, Name: "TEST", Logger: zap.NewNop()}, Callbacks{
		Dispatch: func(_ context.Context, conn *SCTPConn, _ []byte) {
			select {
			case acceptedCh <- conn:
			default:
			}
		},
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

	fd, err := connectLoopback(port)
	if err != nil {
		t.Fatalf("connectLoopback: %v", err)
	}

	t.Cleanup(func() { _ = syscall.Close(fd) })

	client = NewSCTPConn(fd)
	if _, err := client.WriteMsg([]byte("trigger"), &SndRcvInfo{PPID: PPIDWireOrder(testPPID)}); err != nil {
		t.Fatalf("client trigger write: %v", err)
	}

	select {
	case accepted = <-acceptedCh:
	case <-time.After(8 * time.Second):
		t.Fatal("no message dispatched; never captured the accepted conn")
	}

	return srv, accepted, disconnected, client
}

// TestWriter_DeliversInOrder verifies the per-association writer queue delivers
// every enqueued PDU to the peer in FIFO order.
func TestWriter_DeliversInOrder(t *testing.T) {
	srv, serverConn, _, client := serverWithAcceptedConn(t, 29411)

	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()

	const n = 50

	for i := 0; i < n; i++ {
		if _, err := serverConn.WriteMsg([]byte{byte(i)}, &SndRcvInfo{PPID: PPIDWireOrder(testPPID), Stream: 0}); err != nil {
			t.Fatalf("server WriteMsg %d: %v", i, err)
		}
	}

	buf := make([]byte, 128)

	for i := 0; i < n; i++ {
		if err := client.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatalf("SetReadDeadline: %v", err)
		}

		var msg []byte

		for {
			read, _, notification, err := client.ReadMsg(buf)
			if err != nil {
				t.Fatalf("client ReadMsg %d: %v", i, err)
			}

			if notification != nil {
				continue
			}

			msg = buf[:read]

			break
		}

		if !bytes.Equal(msg, []byte{byte(i)}) {
			t.Fatalf("message %d out of order: got %v", i, msg)
		}
	}
}

// TestWriter_WedgedPeerFailsAssociation verifies the F15 fix: a peer that keeps
// the association up but stops reading does not block a sender — the bounded
// queue fills, the association is failed, and disconnect cleanup runs.
func TestWriter_WedgedPeerFailsAssociation(t *testing.T) {
	srv, serverConn, disconnected, client := serverWithAcceptedConn(t, 29412)

	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()

	// Shrink both buffers so a non-reading client wedges the write side after a
	// few messages rather than after megabytes.
	_ = client.controlFd(func(fd int) error {
		return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 2048)
	})
	_ = serverConn.controlFd(func(fd int) error {
		return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 2048)
	})

	// The client never reads. Enqueue a burst larger than the queue depth; the
	// caller must never block, and the queue must overflow (or the write deadline
	// fire) and fail the association.
	payload := make([]byte, 1024)
	deadline := time.Now().Add(20 * time.Second)
	sawQueueFull := false

	for i := 0; i < writeQueueDepth*8 && time.Now().Before(deadline); i++ {
		start := time.Now()

		_, err := serverConn.WriteMsg(payload, &SndRcvInfo{PPID: PPIDWireOrder(testPPID), Stream: 0})

		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("WriteMsg blocked the caller for %v; the queue must be non-blocking", elapsed)
		}

		if errors.Is(err, errWriteQueueFull) {
			sawQueueFull = true
			break
		}
	}

	select {
	case <-disconnected:
	case <-time.After(10 * time.Second):
		t.Fatal("wedged peer did not fail the association")
	}

	if !sawQueueFull {
		t.Log("association failed via write deadline rather than queue overflow (acceptable)")
	}
}
