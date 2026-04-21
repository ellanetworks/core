// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
	hraft "github.com/hashicorp/raft"
)

func newTestStreamLayer(t *testing.T, pki *testutil.PKI, nodeID int) (*raftStreamLayer, *listener.Listener, string) {
	t.Helper()

	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	ln := listener.New(listener.Config{
		BindAddress:      addr,
		AdvertiseAddress: addr,
		NodeID:           nodeID,
		TrustBundle:      pki.BundleFunc(),

		Leaf: pki.LeafFunc(nodeID),

		Revoked: func(*big.Int) bool { return false },
	})

	sl, err := newRaftStreamLayer(ln, addr)
	if err != nil {
		t.Fatalf("newRaftStreamLayer for node %d: %v", nodeID, err)
	}

	return sl, ln, addr
}

func TestRaftStreamLayer_Addr(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1})

	sl, _, addr := newTestStreamLayer(t, pki, 1)

	if got := sl.Addr().String(); got != addr {
		t.Fatalf("Addr() = %q, want %q", got, addr)
	}
}

func TestRaftStreamLayer_CloseUnblocksAccept(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1})

	sl, _, _ := newTestStreamLayer(t, pki, 1)

	done := make(chan struct{})

	go func() {
		_, err := sl.Accept()
		if err == nil {
			t.Error("Accept after Close should return error")
		}

		close(done)
	}()

	time.Sleep(10 * time.Millisecond)

	if err := sl.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Accept did not unblock after Close")
	}
}

func TestRaftStreamLayer_CloseIsIdempotent(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1})

	sl, _, _ := newTestStreamLayer(t, pki, 1)

	if err := sl.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	if err := sl.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestRaftStreamLayer_DialAndAccept(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	sl1, ln1, addr1 := newTestStreamLayer(t, pki, 1)
	sl2, ln2, _ := newTestStreamLayer(t, pki, 2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ln1.Start(ctx); err != nil {
		t.Fatalf("start listener 1: %v", err)
	}

	if err := ln2.Start(ctx); err != nil {
		t.Fatalf("start listener 2: %v", err)
	}

	// Node 2 dials node 1 via the stream layer.
	conn, err := sl2.Dial(hraft.ServerAddress(addr1), 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	defer func() { _ = conn.Close() }()

	// Node 1 accepts the connection.
	accepted, err := sl1.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	defer func() { _ = accepted.Close() }()

	// Write from dialer, read from acceptor.
	if _, err := conn.Write([]byte("raft-data")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	buf := make([]byte, 32)

	n, err := accepted.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if string(buf[:n]) != "raft-data" {
		t.Fatalf("expected %q, got %q", "raft-data", string(buf[:n]))
	}

	_ = sl1.Close()
	_ = sl2.Close()

	ln1.Stop()
	ln2.Stop()
}

func TestRaftStreamLayer_DialBadAddress(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1})

	sl, ln, _ := newTestStreamLayer(t, pki, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ln.Start(ctx); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	_, err := sl.Dial("127.0.0.1:1", 500*time.Millisecond)
	if err == nil {
		t.Fatal("Dial to unreachable address should return error")
	}

	_ = sl.Close()

	ln.Stop()
}
