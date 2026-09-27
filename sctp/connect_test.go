// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"context"
	"testing"
	"time"
)

func TestOnConnectBeforeFirstMessage(t *testing.T) {
	skipIfNoSCTP(t)

	events := make(chan string, 4)

	srv := NewServer(Config{PPID: testPPID, Name: "TEST"}, Callbacks{
		OnConnect: func(conn *SCTPConn) {
			if conn.RemoteAddr() == nil {
				t.Error("OnConnect got a conn without a remote address")
			}

			events <- "connect"
		},
		Dispatch:     func(context.Context, *SCTPConn, []byte) { events <- "dispatch" },
		OnDisconnect: func(*SCTPConn) { events <- "disconnect" },
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

	next := func() string {
		select {
		case e := <-events:
			return e
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for a server callback")
		}

		return ""
	}

	if e := next(); e != "connect" {
		t.Fatalf("first callback = %q, want connect before any message", e)
	}

	if _, err := conn.WriteMsg([]byte("hello"), &SndRcvInfo{PPID: PPIDWireOrder(testPPID)}); err != nil {
		t.Fatalf("WriteMsg: %v", err)
	}

	if e := next(); e != "dispatch" {
		t.Fatalf("second callback = %q, want dispatch", e)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if e := next(); e != "disconnect" {
		t.Fatalf("third callback = %q, want disconnect", e)
	}
}
