// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"context"
	"testing"
	"time"
)

func TestWriteMsgUnordered(t *testing.T) {
	skipIfNoSCTP(t)

	srv := NewServer(Config{PPID: testPPID, Name: "TEST"}, Callbacks{
		Dispatch: func(_ context.Context, conn *SCTPConn, msg []byte) {
			if _, err := conn.WriteMsg(msg, &SndRcvInfo{PPID: PPIDWireOrder(testPPID), Flags: SCTPUnordered}); err != nil {
				t.Errorf("unordered write: %v", err)
			}
		},
	})

	ctx, cancel := context.WithCancel(context.Background())

	ln, err := testListen("127.0.0.1")
	if err != nil {
		cancel()
		t.Fatalf("Listen: %v", err)
	}

	srv.Serve(ctx, ln)

	t.Cleanup(func() {
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		srv.Shutdown(shutdownCtx)
	})

	conn := dialLoopback(t, ln.laddr.Port)

	if _, err := conn.WriteMsg([]byte("unordered"), &SndRcvInfo{PPID: PPIDWireOrder(testPPID)}); err != nil {
		t.Fatalf("WriteMsg: %v", err)
	}

	buf := make([]byte, 64)

	n, info, err := conn.ReadMsg(buf)
	if err != nil {
		t.Fatalf("ReadMsg: %v", err)
	}

	if string(buf[:n]) != "unordered" {
		t.Fatalf("payload = %q", buf[:n])
	}

	if info.Flags&SCTPUnordered == 0 {
		t.Fatalf("received flags = 0x%x, want SCTPUnordered set", info.Flags)
	}
}
