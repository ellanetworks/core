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
				if err := tr.writeMessage(context.Background(), b); err != nil {
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
		err = tr.writeMessage(context.Background(), b)
	}

	if !errors.Is(err, net.ErrClosed) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("write to a stalled peer = %v", err)
	}

	if err := tr.writeMessage(context.Background(), b); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("write after the timeout = %v", err)
	}
}

func TestSCTPCallerDeadlineEndsWaitWithoutAborting(t *testing.T) {
	tr, remote := sctpPair(t)
	tr.setWriteTimeout(testTimeout)

	b := marshalled(t, appRequest(1, 1, "ella.example.org", OctetString(AVPUserName, 0, 0, make([]byte, 16*1024))))

	sent := 0
	deadline := time.Now().Add(testTimeout)

	var err error

	for err == nil && time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		err = tr.writeMessage(ctx, b)

		cancel()

		if err == nil {
			sent++
		}
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("write with an expired caller deadline = %v", err)
	}

	if tr.closed.Load() {
		t.Fatal("caller deadline aborted the association")
	}

	drained := make(chan error, 1)

	go func() {
		buf := make([]byte, maxMessageSize)

		for range sent + 1 {
			if _, _, err := remote.ReadMsg(buf); err != nil {
				drained <- err
				return
			}
		}

		drained <- nil
	}()

	if err := tr.writeMessage(context.Background(), b); err != nil {
		t.Fatalf("write after the peer resumed reading = %v", err)
	}

	if err := <-drained; err != nil {
		t.Fatalf("read: %v", err)
	}
}
