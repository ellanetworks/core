// Copyright 2026 Ella Networks

package listener_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
)

// freePort returns an available TCP port on localhost.
func freePort(t *testing.T) int {
	t.Helper()

	lc := net.ListenConfig{}

	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}

	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	return port
}

func newTestListener(t *testing.T, pki *testutil.PKI, nodeID int) (*listener.Listener, string) {
	t.Helper()

	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	ln := listener.New(listener.Config{
		BindAddress:      addr,
		AdvertiseAddress: addr,
		NodeID:           nodeID,
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[nodeID].TLSCert,
	})

	return ln, addr
}

func TestListener_RoundtripRaft(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	ln1, addr1 := newTestListener(t, pki, 1)

	var received sync.WaitGroup

	received.Add(1)

	ln1.Register(listener.ALPNRaft, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		buf := make([]byte, 64)
		n, _ := conn.Read(buf)

		if string(buf[:n]) != "raft-ping" {
			t.Errorf("expected raft-ping, got %q", string(buf[:n]))
		}

		_, _ = conn.Write([]byte("raft-pong"))

		received.Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ln1.Start(ctx); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	// Node 2 dials node 1 with ALPN Raft.
	ln2, _ := newTestListener(t, pki, 2)
	defer ln2.Stop()

	conn, err := ln2.Dial(ctx, addr1, 1, listener.ALPNRaft, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	defer func() { _ = conn.Close() }()

	if _, err := conn.Write([]byte("raft-ping")); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, 64)

	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(buf[:n]) != "raft-pong" {
		t.Fatalf("expected raft-pong, got %q", string(buf[:n]))
	}

	received.Wait()
	ln1.Stop()
}

func TestListener_RoundtripHTTP(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	ln1, addr1 := newTestListener(t, pki, 1)

	ln1.Register(listener.ALPNHTTP, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		buf := make([]byte, 64)
		n, _ := conn.Read(buf)

		if string(buf[:n]) != "http-hello" {
			t.Errorf("expected http-hello, got %q", string(buf[:n]))
		}

		_, _ = conn.Write([]byte("http-world"))
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ln1.Start(ctx); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	ln2, _ := newTestListener(t, pki, 2)
	defer ln2.Stop()

	conn, err := ln2.Dial(ctx, addr1, 1, listener.ALPNHTTP, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	defer func() { _ = conn.Close() }()

	_, _ = conn.Write([]byte("http-hello"))

	buf := make([]byte, 64)

	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(buf[:n]) != "http-world" {
		t.Fatalf("expected http-world, got %q", string(buf[:n]))
	}

	ln1.Stop()
}

func TestListener_ALPNDispatch(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	ln1, addr1 := newTestListener(t, pki, 1)

	var raftHit, httpHit sync.WaitGroup

	raftHit.Add(1)
	httpHit.Add(1)

	ln1.Register(listener.ALPNRaft, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		_, _ = conn.Write([]byte("raft"))

		raftHit.Done()
	})

	ln1.Register(listener.ALPNHTTP, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		_, _ = conn.Write([]byte("http"))

		httpHit.Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ln1.Start(ctx); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	ln2, _ := newTestListener(t, pki, 2)
	defer ln2.Stop()

	// Dial with Raft ALPN.
	connR, err := ln2.Dial(ctx, addr1, 1, listener.ALPNRaft, 2*time.Second)
	if err != nil {
		t.Fatalf("dial raft: %v", err)
	}

	buf := make([]byte, 16)

	n, _ := connR.Read(buf)
	if string(buf[:n]) != "raft" {
		t.Fatalf("expected raft handler, got %q", string(buf[:n]))
	}

	_ = connR.Close()

	// Dial with HTTP ALPN.
	connH, err := ln2.Dial(ctx, addr1, 1, listener.ALPNHTTP, 2*time.Second)
	if err != nil {
		t.Fatalf("dial http: %v", err)
	}

	n, _ = connH.Read(buf)
	if string(buf[:n]) != "http" {
		t.Fatalf("expected http handler, got %q", string(buf[:n]))
	}

	_ = connH.Close()

	raftHit.Wait()
	httpHit.Wait()

	ln1.Stop()
}

func TestListener_WrongCA_Rejected(t *testing.T) {
	pki1 := testutil.GenTestPKI(t, []int{1})
	pki2 := testutil.GenTestPKI(t, []int{2})

	// Node 1 trusts pki1's CA.
	ln1, addr1 := newTestListener(t, pki1, 1)

	ln1.Register(listener.ALPNRaft, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		_, _ = io.ReadAll(conn)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ln1.Start(ctx); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	// Node 2 uses pki2's CA — different from pki1.
	ln2 := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           2,
		CAPool:           pki2.CAPool,
		LeafCert:         pki2.Nodes[2].TLSCert,
	})
	defer ln2.Stop()

	_, err := ln2.Dial(ctx, addr1, 1, listener.ALPNRaft, 2*time.Second)
	if err == nil {
		t.Fatal("expected dial to fail with wrong CA")
	}

	ln1.Stop()
}

func TestListener_PeerNodeID(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	ln1, addr1 := newTestListener(t, pki, 1)

	var gotNodeID int

	var wg sync.WaitGroup

	wg.Add(1)

	ln1.Register(listener.ALPNRaft, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		defer wg.Done()

		// The accepted conn should reveal node 2's identity.
		nodeID, err := listener.PeerNodeID(conn.(*tls.Conn))
		if err != nil {
			t.Errorf("PeerNodeID: %v", err)
			return
		}

		gotNodeID = nodeID
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ln1.Start(ctx); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	ln2, _ := newTestListener(t, pki, 2)
	defer ln2.Stop()

	conn, err := ln2.Dial(ctx, addr1, 1, listener.ALPNRaft, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	_ = conn.Close()

	wg.Wait()

	if gotNodeID != 2 {
		t.Fatalf("expected peer node-id 2, got %d", gotNodeID)
	}

	ln1.Stop()
}

// TestListener_Dial_ExpectedPeerMismatch covers the case where a caller
// dials an address expecting node X but reaches a peer whose cluster
// leaf resolves to node Y. The chain and CN shape still verify, but Dial
// must tear down the connection to prevent acting on data from the wrong
// peer (e.g. an offboarded node's key answering on a current peer's IP).
func TestListener_Dial_ExpectedPeerMismatch(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	ln1, addr1 := newTestListener(t, pki, 1)

	ln1.Register(listener.ALPNRaft, func(conn net.Conn) {
		_ = conn.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ln1.Start(ctx); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	defer ln1.Stop()

	ln2, _ := newTestListener(t, pki, 2)
	defer ln2.Stop()

	// ln1 answers as node-id 1; dialing with expectedPeerID=7 must be rejected.
	_, err := ln2.Dial(ctx, addr1, 7, listener.ALPNRaft, 2*time.Second)
	if err == nil {
		t.Fatal("expected dial to fail when peer CN does not match expectedPeerID")
	}

	if !strings.Contains(err.Error(), "expected peer node-id 7") {
		t.Fatalf("expected mismatch error referencing expectedPeerID, got: %v", err)
	}
}

// TestListener_DialAnyPeer accepts any valid cluster peer without pinning
// a specific node-id. Used by discovery; verifies chain + CN shape are
// still enforced but the identity check is relaxed.
func TestListener_DialAnyPeer(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	ln1, addr1 := newTestListener(t, pki, 1)

	ln1.Register(listener.ALPNRaft, func(conn net.Conn) {
		_ = conn.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ln1.Start(ctx); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	defer ln1.Stop()

	ln2, _ := newTestListener(t, pki, 2)
	defer ln2.Stop()

	conn, err := ln2.DialAnyPeer(ctx, addr1, listener.ALPNRaft, 2*time.Second)
	if err != nil {
		t.Fatalf("DialAnyPeer: %v", err)
	}

	_ = conn.Close()
}

func TestListener_Stop_Completes(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1})

	ln, _ := newTestListener(t, pki, 1)

	ln.Register(listener.ALPNRaft, func(conn net.Conn) {
		_ = conn.Close()
	})

	if err := ln.Start(context.Background()); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	done := make(chan struct{})

	go func() {
		ln.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return")
	}
}

func TestListener_AdvertiseAddress(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1})

	ln := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:9999",
		AdvertiseAddress: "10.0.0.1:7000",
		NodeID:           1,
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[1].TLSCert,
	})

	if ln.AdvertiseAddress() != "10.0.0.1:7000" {
		t.Fatalf("expected 10.0.0.1:7000, got %s", ln.AdvertiseAddress())
	}
}

func TestListener_UnknownALPN_Closed(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	ln1, addr1 := newTestListener(t, pki, 1)

	// Only register Raft — HTTP is unregistered.
	ln1.Register(listener.ALPNRaft, func(conn net.Conn) {
		_ = conn.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ln1.Start(ctx); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	ln2, _ := newTestListener(t, pki, 2)
	defer ln2.Stop()

	// Dial with HTTP ALPN which is not registered on ln1.
	conn, err := ln2.Dial(ctx, addr1, 1, listener.ALPNHTTP, 2*time.Second)
	if err != nil {
		// Server may reject during handshake since ALPNHTTP is still
		// in NextProtos — the conn is accepted but dispatch closes it.
		// Either a dial error or an immediate read error is acceptable.
		if !strings.Contains(err.Error(), "ALPN") {
			t.Logf("dial error (acceptable): %v", err)
		}

		ln1.Stop()

		return
	}

	defer func() { _ = conn.Close() }()

	// The server should close the connection since there's no handler.
	buf := make([]byte, 1)

	_, err = conn.Read(buf)
	if err == nil {
		t.Fatal("expected read error on unhandled ALPN connection")
	}

	ln1.Stop()
}
