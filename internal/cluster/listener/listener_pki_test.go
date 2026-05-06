// Copyright 2026 Ella Networks

package listener_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
	"github.com/ellanetworks/core/internal/pki"
)

// TestListener_BootstrapALPN_NoClientCert verifies a client without a
// leaf can complete the handshake on the bootstrap ALPN and the
// handler runs.
func TestListener_BootstrapALPN_NoClientCert(t *testing.T) {
	p := testutil.GenTestPKI(t, []int{1})

	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	ln := listener.New(listener.Config{
		BindAddress:      addr,
		AdvertiseAddress: addr,
		NodeID:           1,
		Pin:              p.PinFunc(),
		Leaf:             p.LeafFunc(1),
	})

	defer ln.Stop()

	var wg sync.WaitGroup

	wg.Add(1)

	ln.Register(listener.ALPNPKIBootstrap, func(conn net.Conn) {
		defer wg.Done()
		defer func() { _ = conn.Close() }()

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

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 2 * time.Second},
		Config: &tls.Config{
			MinVersion:         tls.VersionTLS13,
			NextProtos:         []string{listener.ALPNPKIBootstrap},
			InsecureSkipVerify: true, // test-only
		},
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.Fatalf("bootstrap dial: %v", err)
	}

	_ = conn.Close()

	waitWithTimeout(t, &wg, 2*time.Second)
}

// TestListener_CloseByPeerFingerprint closes a tracked connection
// after its peer has been removed from the keyring.
func TestListener_CloseByPeerFingerprint(t *testing.T) {
	p := testutil.GenTestPKI(t, []int{1, 2})

	ln1, addr1 := newTestListener(t, p, 1)
	defer ln1.Stop()

	handlerReady := make(chan struct{})
	closedCh := make(chan struct{})

	ln1.Register(listener.ALPNRaft, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		close(handlerReady)

		buf := make([]byte, 16)
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

	fp := pki.Fingerprint(p.Nodes[2].Cert)

	closed := ln1.CloseByPeerFingerprint(fp)
	if closed == 0 {
		t.Fatal("CloseByPeerFingerprint should have closed at least one conn")
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
