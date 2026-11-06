// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/sctp"
)

func sctpPair(t *testing.T) (*sctpTransport, *sctp.SCTPConn) {
	t.Helper()

	requireSCTP(t)

	var lc sctp.ListenConfig

	ln, err := lc.Listen(context.Background(), &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: loopback1.AsSlice()}}})
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = ln.Close() }()

	accepted := make(chan *sctp.SCTPConn, 1)

	go func() {
		c, err := ln.Accept()
		if err != nil {
			close(accepted)
			return
		}

		accepted <- c
	}()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	port := ln.Addr().(*sctp.SCTPAddr).Port

	tr, err := dialSCTP(ctx, nil, []netip.Addr{loopback1}, uint16(port), nil)
	if err != nil {
		t.Fatal(err)
	}

	remote, ok := <-accepted
	if !ok {
		t.Fatal("accept failed")
	}

	t.Cleanup(func() {
		_ = tr.abort()
		_ = remote.Abort()
	})

	return tr.(*sctpTransport), remote
}

func TestSCTPBriefStallDoesNotAbortTheAssociation(t *testing.T) {
	tr, remote := sctpPair(t)
	tr.setWriteTimeout(10 * time.Second)

	b := marshalled(t, appRequest(1, 1, "ella.example.org", OctetString(AVPUserName, 0, 0, make([]byte, 4*1024))))

	const (
		writers   = 8
		perWriter = 200
	)

	errs := make(chan error, writers)

	var wg sync.WaitGroup

	for range writers {
		wg.Go(func() {
			for range perWriter {
				if err := tr.writeMessage(b); err != nil {
					errs <- err
					return
				}
			}
		})
	}

	time.Sleep(500 * time.Millisecond)

	buf := make([]byte, maxMessageSize)

	for i := range writers * perWriter {
		if _, _, err := remote.ReadMsg(buf); err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("write during a brief stall = %v", err)
	}
}

func TestSCTPStalledPeerAbortsAfterWriteTimeout(t *testing.T) {
	tr, _ := sctpPair(t)
	tr.setWriteTimeout(200 * time.Millisecond)

	b := marshalled(t, appRequest(1, 1, "ella.example.org", OctetString(AVPUserName, 0, 0, make([]byte, 16*1024))))

	deadline := time.Now().Add(testTimeout)

	var err error

	for err == nil && time.Now().Before(deadline) {
		err = tr.writeMessage(b)
	}

	if !errors.Is(err, net.ErrClosed) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("write to a stalled peer = %v", err)
	}

	if err := tr.writeMessage(b); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("write after the timeout = %v", err)
	}
}
