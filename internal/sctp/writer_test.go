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

// A peer that keeps the association up but stops reading must not block a sender.
func TestWriter_WedgedPeerFailsAssociation(t *testing.T) {
	srv, serverConn, disconnected, client := serverWithAcceptedConn(t, 29412)

	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		srv.Shutdown(sctx)
	}()

	// Shrink both buffers so the write side wedges within a few messages.
	_ = client.controlFd(func(fd int) error {
		return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 2048)
	})
	_ = serverConn.controlFd(func(fd int) error {
		return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 2048)
	})

	payload := make([]byte, 1024)
	deadline := time.Now().Add(20 * time.Second)
	sawQueueFull := false

	for i := 0; i < writeQueueDepth*8 && time.Now().Before(deadline); i++ {
		start := time.Now()

		_, err := serverConn.WriteMsg(payload, &SndRcvInfo{PPID: PPIDWireOrder(testPPID), Stream: 0})

		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("WriteMsg blocked the caller for %v; the queue must be non-blocking", elapsed)
		}

		if errors.Is(err, ErrWriteQueueFull) {
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
		t.Log("association failed via write deadline, not queue overflow")
	}
}

func TestWriter_CloseReleasesServeGoroutine(t *testing.T) {
	srv, serverConn, disconnected, _ := serverWithAcceptedConn(t, 29413)

	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		srv.Shutdown(sctx)
	}()

	_ = serverConn.Close()

	select {
	case <-disconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("OnDisconnect never ran; the serve goroutine is stuck waiting on the writer")
	}
}

func TestWriter_StartAfterCloseDoesNotOrphan(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29414

	ln := newTestListener(t, port)
	conn := acceptOne(t, ln, port)

	_ = conn.Close()

	conn.startWriter(zap.NewNop())

	done := make(chan struct{})

	go func() {
		conn.awaitWriter()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("awaitWriter blocked on a writer started after Close")
	}
}

// A send that stays parked past the write deadline must fail the association
// even when the queue has room.
func TestWriter_WriteDeadlineFailsAssociation(t *testing.T) {
	original := writeTimeout
	writeTimeout = 250 * time.Millisecond

	defer func() { writeTimeout = original }()

	srv, serverConn, disconnected, client := serverWithAcceptedConn(t, 29415)

	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		srv.Shutdown(sctx)
	}()

	_ = client.controlFd(func(fd int) error {
		return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 2048)
	})
	_ = serverConn.controlFd(func(fd int) error {
		return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 2048)
	})

	// Stay well under writeQueueDepth so the deadline, not queue overflow, is
	// what fails the association.
	payload := make([]byte, 4096)
	for i := 0; i < 16; i++ {
		if _, err := serverConn.WriteMsg(payload, &SndRcvInfo{PPID: PPIDWireOrder(testPPID), Stream: 0}); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	select {
	case <-disconnected:
	case <-time.After(10 * time.Second):
		t.Fatal("a send parked past the write deadline did not fail the association")
	}
}
