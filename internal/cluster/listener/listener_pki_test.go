// Copyright 2026 Ella Networks

package listener_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
	"github.com/ellanetworks/core/internal/pki"
)

// TestListener_BootstrapALPN_NoClientCert verifies a client without a
// leaf can complete the handshake on the bootstrap ALPN and the handler
// runs.
func TestListener_BootstrapALPN_NoClientCert(t *testing.T) {
	p := testutil.GenTestPKI(t, []int{1})

	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	bundle := p.Bundle()
	leaf := p.Nodes[1].TLSCert
	ln := listener.New(listener.Config{
		BindAddress:      addr,
		AdvertiseAddress: addr,
		NodeID:           1,
		TrustBundle:      func() *pki.TrustBundle { return bundle },
		Leaf:             func() *tls.Certificate { return &leaf },
		Revoked:          func(*big.Int) bool { return false },
	})

	defer ln.Stop()

	var wg sync.WaitGroup

	wg.Add(1)

	ln.Register(listener.ALPNPKIBootstrap, func(conn net.Conn) {
		defer wg.Done()
		defer func() { _ = conn.Close() }()

		// Must have no peer cert on this ALPN.
		tc := conn.(*tls.Conn)
		if len(tc.ConnectionState().PeerCertificates) != 0 {
			t.Errorf("bootstrap handler saw %d peer certs, want 0", len(tc.ConnectionState().PeerCertificates))
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ln.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Build a raw TLS client that trusts the server (via InsecureSkipVerify
	// plus our own VerifyConnection) and presents no client cert at all.
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 2 * time.Second},
		Config: &tls.Config{
			MinVersion:         tls.VersionTLS13,
			NextProtos:         []string{listener.ALPNPKIBootstrap},
			InsecureSkipVerify: true, // test-only: we're not verifying server here.
		},
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.Fatalf("bootstrap dial: %v", err)
	}

	_ = conn.Close()

	waitWithTimeout(t, &wg, 2*time.Second)
}

// TestListener_CloseByPeerSerial closes a tracked connection after its
// peer's leaf is "revoked".
func TestListener_CloseByPeerSerial(t *testing.T) {
	p := testutil.GenTestPKI(t, []int{1, 2})

	ln1, addr1 := newTestListener(t, p, 1)
	defer ln1.Stop()

	handlerReady := make(chan struct{})
	closedCh := make(chan struct{})

	ln1.Register(listener.ALPNRaft, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		close(handlerReady)

		buf := make([]byte, 16)
		// Read will return an error when CloseByPeerSerial fires.
		_, _ = conn.Read(buf)

		close(closedCh)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ln1.Start(ctx); err != nil {
		t.Fatal(err)
	}

	ln2, _ := newTestListener(t, p, 2)
	defer ln2.Stop()

	conn, err := ln2.Dial(ctx, addr1, 1, listener.ALPNRaft, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	defer func() { _ = conn.Close() }()

	<-handlerReady

	// Node 2's leaf serial is the one we use to close.
	leafCert := p.Nodes[2].TLSCert
	parsed := parsedCert(t, leafCert)

	closed := ln1.CloseByPeerSerial(parsed.SerialNumber)
	if closed == 0 {
		t.Fatal("CloseByPeerSerial should have closed at least one conn")
	}

	select {
	case <-closedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("server-side handler did not observe close")
	}
}

func waitWithTimeout(t *testing.T, wg *sync.WaitGroup, d time.Duration) {
	t.Helper()

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(d):
		t.Fatal("wg did not complete in time")
	}
}
