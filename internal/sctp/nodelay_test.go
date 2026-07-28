// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"context"
	"testing"
	"time"
	"unsafe"

	"go.uber.org/zap"
)

func nodelayValue(t *testing.T, conn *SCTPConn) int32 {
	t.Helper()

	var v int32

	optlen := uint32(unsafe.Sizeof(v))

	err := conn.controlFd(func(fd int) error {
		return getsockopt(fd, SCTPNoDelay, uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&optlen)))
	})
	if err != nil {
		t.Fatalf("getsockopt SCTP_NODELAY: %v", err)
	}

	return v
}

// An accepted association inherits SCTP_NODELAY from the listener.
func TestServer_AcceptedConnHasNoDelay(t *testing.T) {
	skipIfNoSCTP(t)

	const port = 29407

	accepted := make(chan *SCTPConn, 1)

	srv := NewServer(Config{
		PPID:   testPPID,
		Name:   "TEST",
		Logger: zap.NewNop(),
	}, Callbacks{
		Dispatch: func(_ context.Context, conn *SCTPConn, _ []byte) {
			select {
			case accepted <- conn:
			default:
			}
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

	if _, err := client.WriteMsg([]byte("probe"), &SndRcvInfo{PPID: PPIDWireOrder(testPPID), Stream: 0}); err != nil {
		t.Fatalf("WriteMsg: %v", err)
	}

	select {
	case conn := <-accepted:
		if got := nodelayValue(t, conn); got == 0 {
			t.Errorf("accepted conn SCTP_NODELAY = %d, want non-zero", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no message dispatched within 5s")
	}
}
