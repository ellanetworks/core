// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
)

// TestClaimICSExactlyOneWinner asserts the ICS claim is atomic: among many
// concurrent claimers starting from ICSNotStarted, exactly one wins (and thus is
// the sole sender of the InitialContextSetupRequest). This is the TOCTOU guarantee
// the SMF N1N2 path relies on.
func TestClaimICSExactlyOneWinner(t *testing.T) {
	conn := &amf.UeConn{}

	const n = 128

	var (
		wins int64
		wg   sync.WaitGroup
		gate = make(chan struct{})
	)

	for range n {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-gate

			if conn.ClaimICS() {
				atomic.AddInt64(&wins, 1)
			}
		}()
	}

	close(gate)
	wg.Wait()

	if wins != 1 {
		t.Fatalf("ClaimICS from ICSNotStarted: expected exactly 1 winner, got %d", wins)
	}

	if got := conn.ICS(); got != amf.ICSPending {
		t.Fatalf("ICS after claim = %v, want ICSPending", got)
	}
}

// TestICSConcurrentAccessNoRace exercises the three goroutine contexts that touch
// the ICS state — the SMF N1N2 claim, the NGAP-dispatch completion, and the
// NAS-guard timer re-pend — concurrently on one connection. Its value is under
// `-race`: on a plain (non-atomic) field this fails; through the atomic accessors
// it is clean.
func TestICSConcurrentAccessNoRace(t *testing.T) {
	conn := &amf.UeConn{}

	var wg sync.WaitGroup

	for _, op := range []func(){
		func() { conn.ClaimICS() },         // SMF N1N2 path
		func() { conn.MarkICSCompleted() }, // NGAP dispatch (ICS response)
		func() { conn.MarkICSPending() },   // NAS-guard timer re-pend
		func() { _ = conn.ICS() },          // status read
		func() { conn.ResetICS() },         // rollback on send failure
	} {
		wg.Add(1)

		go func(f func()) {
			defer wg.Done()

			for range 1000 {
				f()
			}
		}(op)
	}

	wg.Wait()
}
