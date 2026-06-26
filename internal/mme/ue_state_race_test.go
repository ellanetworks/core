// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"sync"
	"testing"
)

// TestUEStateConcurrentAccess drives the goroutines that touch a connected UE in
// production — the data-network reconcile backstop, the status API, a
// network-initiated detach, and the eNB dispatch loop committing a bearer
// modification — concurrently against one UE. Run with -race to surface
// unsynchronised access to the EMM/ECM state machine and the PDN-connection
// flags.
func TestUEStateConcurrentAccess(t *testing.T) {
	m := newTestMME(t)
	ue, _ := connectedBearerUE(t, m)

	ctx := context.Background()

	const iters = 1500

	var wg sync.WaitGroup

	// Reconcile backstop: reads emmState/ecmState and every PDN's flags.
	wg.Add(1)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			m.ReconcileDataNetwork(ctx)
		}
	}()

	// Status API: reads emmState, IMEI, and PDN fields.
	wg.Add(1)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			_ = m.ConnectedSubscribers()
		}
	}()

	// Dispatch loop committing/rejecting a bearer modification: writes the PDN
	// flags. A nil body falls back to the default PDN, exercising the flag writes.
	wg.Add(1)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			m.onModifyBearerReject(ue, nil)
		}
	}()

	// Dispatch loop advancing the EMM/ECM state machine while the reconcile and
	// status goroutines read it.
	wg.Add(1)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			ue.emmState.store(EMMRegistered)
		}
	}()

	wg.Wait()
}

// TestS1IdentityConcurrentSendVsResume reproduces AC1: a UE resuming from
// ECM-IDLE rebinds its S1 identity (conn, MME/ENB-UE-S1AP-IDs) on the dispatch
// goroutine while an off-dispatch send (e.g. a network-initiated detach) reads
// it. Run with -race.
func TestS1IdentityConcurrentSendVsResume(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	cc2 := &captureConn{}

	const iters = 2000

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			m.establishS1Connection(ue, cc2, 9)
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			m.sendDownlink(context.Background(), ue, []byte{0x07, 0x42})
		}
	}()

	wg.Wait()
}
