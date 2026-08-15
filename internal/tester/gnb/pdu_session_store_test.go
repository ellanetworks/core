// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"sync"
	"testing"
	"time"
)

func newTestGnodeB() *GnodeB {
	g := &GnodeB{}
	g.cond = sync.NewCond(&g.mu)

	return g
}

// A procedure must see the resources its own signalling established, never the
// ones already in the store: the gNB allocates a fresh downlink TEID on every
// re-establishment, and a scenario that picked up the newer one would tear down
// a tunnel it never built.
func TestAwaitPDUSessionIgnoresOlderResources(t *testing.T) {
	g := newTestGnodeB()

	g.storePDUSession(1, &PDUSessionInformation{PDUSessionID: 1, DLTEID: 3})

	before := g.sessionGeneration()

	go func() {
		time.Sleep(10 * time.Millisecond)
		g.storePDUSession(1, &PDUSessionInformation{PDUSessionID: 1, DLTEID: 7})
	}()

	got, err := g.awaitPDUSession(1, 1, before, time.Second)
	if err != nil {
		t.Fatalf("awaitPDUSession: %v", err)
	}

	if got.DLTEID != 7 {
		t.Errorf("awaited DL TEID %d, want the re-established 7", got.DLTEID)
	}
}

// storePDUSession updates the stored session in place, so what a procedure
// returned must not change underneath its caller.
func TestAwaitPDUSessionReturnsASnapshot(t *testing.T) {
	g := newTestGnodeB()

	g.storePDUSession(1, &PDUSessionInformation{PDUSessionID: 1, DLTEID: 3})

	got, err := g.awaitPDUSession(1, 1, 0, time.Second)
	if err != nil {
		t.Fatalf("awaitPDUSession: %v", err)
	}

	g.storePDUSession(1, &PDUSessionInformation{PDUSessionID: 1, DLTEID: 7})

	if got.DLTEID != 3 {
		t.Errorf("the returned session now reports DL TEID %d, want the 3 it was established with", got.DLTEID)
	}
}

// A session that only ever existed before the generation must not satisfy the
// wait: the procedure would otherwise report resources it never established.
func TestAwaitPDUSessionTimesOutOnStaleResources(t *testing.T) {
	g := newTestGnodeB()

	g.storePDUSession(1, &PDUSessionInformation{PDUSessionID: 1, DLTEID: 3})

	if _, err := g.awaitPDUSession(1, 1, g.sessionGeneration(), 50*time.Millisecond); err == nil {
		t.Fatal("awaitPDUSession accepted a session established before the procedure started")
	}
}

// CloseTunnel matches s1enb's: tearing down a session the network already
// released is not an error.
func TestCloseTunnelIsSilentOnAMissingTunnel(t *testing.T) {
	g := newTestGnodeB()
	g.tunnels = map[uint32]*Tunnel{}

	g.CloseTunnel(7)
}
