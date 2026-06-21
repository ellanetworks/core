// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestServer_DispatchesMatchingPPID verifies the shared Server delivers a
// message whose PPID matches the configured one, discards a message with a
// different PPID, and shuts down cleanly.
func TestServer_DispatchesMatchingPPID(t *testing.T) {
	skipIfNoSCTP(t)

	const (
		port = 29401
		ppid = uint32(18) // S1AP
	)

	got := make(chan []byte, 2)

	srv := NewServer(Config{
		PPID:   ppid,
		Name:   "TEST",
		Logger: zap.NewNop(),
	}, Callbacks{
		Dispatch: func(_ context.Context, _ *SCTPConn, msg []byte) {
			got <- msg
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := srv.ListenAndServe(ctx, "127.0.0.1", port, ""); err != nil {
		t.Fatalf("ListenAndServe: %v", err)
	}

	fd, err := connectLoopback(port)
	if err != nil {
		t.Fatalf("connectLoopback: %v", err)
	}

	client := NewSCTPConn(fd)
	defer func() { _ = client.Close() }()

	// The PPID travels big-endian on the wire (TS 36.412 §7), which the server
	// converts back with PPIDWireOrder before comparing.
	// A message on a non-S1AP PPID must be discarded by the server.
	if _, err := client.WriteMsg([]byte("wrong-ppid"), &SndRcvInfo{PPID: PPIDWireOrder(NGAPPPID), Stream: 0}); err != nil {
		t.Fatalf("WriteMsg wrong PPID: %v", err)
	}

	want := []byte("matching-ppid")
	if _, err := client.WriteMsg(want, &SndRcvInfo{PPID: PPIDWireOrder(ppid), Stream: 0}); err != nil {
		t.Fatalf("WriteMsg matching PPID: %v", err)
	}

	select {
	case msg := <-got:
		if string(msg) != string(want) {
			t.Errorf("dispatched %q, want %q (wrong-PPID message not discarded?)", msg, want)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timed out waiting for dispatched message")
	}

	// Shut down while the client is still connected. Cancelling the serve
	// context lets the accept loop exit, and closing the server-side
	// association unblocks the read loop, so the serve goroutine exits promptly.
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	srv.Shutdown(shutdownCtx)
}
