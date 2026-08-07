// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// The ESM information wait is written by the NAS goroutine and cleared by the S1
// connection release, which runs under the registry lock and not ue.mu. Run under
// -race.
func TestESMInformationWaitIsSafeAgainstConnectionRelease(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	done := make(chan struct{})

	go func() {
		defer close(done)

		for i := range 500 {
			ue.AwaitESMInformation(uint8(i%254+1), nil)
			_ = ue.AwaitingESMInformation()
			_ = ue.PendingESMInfo()
			ue.TakeESMInfoWait()
		}
	}()

	for range 500 {
		m.AttachUeConn(ue, m.NewUeConn(cc, s1ap.ENBUES1APID(7)))
		m.FreeUeConn(ue)
	}

	<-done
}

// The T3489 abort runs on the timer goroutine while the NAS goroutine records a
// fresh attach's parameters, so the abort reads only the transaction identity the
// wait recorded. Run under -race.
func TestT3489AbortReadsNoMutableAttachState(t *testing.T) {
	m := newTestMME(t)
	m.SetT3489ForTest(5*time.Millisecond, 0)

	ue, _ := securedUE(t, m)

	ue.AwaitESMInformation(4, nil)

	if !requestESMInformation(context.Background(), ue, func(pti uint8) {
		rejectAttachESM(context.Background(), m, ue, pti, eps.ESMCauseESMInformationNotReceived)
	}) {
		t.Fatal("requestESMInformation reported no wait, want true")
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		for i := range 5000 {
			ue.RequestedPTI = nas.ProcedureTransactionIdentity(uint8(i%254 + 1))
			ue.RequestedAPN = "internet"
			ue.RequestedPDUSessionID = uint8(i % 16)
			ue.RequestedType = eps.RequestTypeInitialRequest
		}
	}()

	waitForNoESMInformation(t, ue)

	<-done
}
