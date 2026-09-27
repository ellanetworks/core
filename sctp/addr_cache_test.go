// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Repeated LocalAddr/RemoteAddr calls return the cached resolution.
func TestSCTPConn_AddrAccessorsCached(t *testing.T) {
	skipIfNoSCTP(t)

	srv := NewServer(Config{
		PPID: testPPID,
		Name: "TEST",
	}, Callbacks{
		Dispatch: func(_ context.Context, _ *SCTPConn, _ []byte) {},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := testListen("127.0.0.1")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	srv.Serve(ctx, ln)

	fd, err := connectLoopback(ln.laddr.Port)
	if err != nil {
		t.Fatalf("connectLoopback: %v", err)
	}

	client := newSCTPConn(fd)

	defer func() { _ = client.Close() }()

	deadline := time.Now().Add(5 * time.Second)

	var remote1 any

	for remote1 == nil && time.Now().Before(deadline) {
		remote1 = client.RemoteAddr()
	}

	if remote1 == nil {
		t.Fatal("RemoteAddr = nil after association establishment")
	}

	if got := remote1.(*SCTPAddr).String(); !strings.Contains(got, "127.0.0.1") {
		t.Errorf("RemoteAddr = %q, want loopback address", got)
	}

	if remote2 := client.RemoteAddr(); remote2 != remote1 {
		t.Errorf("second RemoteAddr returned a fresh resolution, want the cached one")
	}

	local1 := client.LocalAddr()
	if local1 == nil {
		t.Fatal("LocalAddr = nil for established association")
	}

	if local2 := client.LocalAddr(); local2 != local1 {
		t.Errorf("second LocalAddr returned a fresh resolution, want the cached one")
	}
}
