// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/nas/eps"
)

// waitFor polls until cond holds or the deadline passes, so a timer-driven
// assertion does not depend on a fixed sleep.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", what)
}

// TS 24.301 §6.6.1.2.6 a): the first two expiries of T3489 resend the ESM
// INFORMATION REQUEST; the third aborts the procedure and rejects the attach.
func TestT3489RetransmitsTwiceThenRejects(t *testing.T) {
	m := esmInfoTestMME()
	m.SetT3489ConfigForTest(5*time.Millisecond, 2)

	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue)

	// Initial request, two retransmissions, the Attach Reject and the release.
	waitFor(t, "T3489 to exhaust its retransmissions", func() bool { return cc.count() >= 5 })

	if got := cc.count(); got != 5 {
		t.Fatalf("message count is %d, want 5 (request, 2 retransmissions, Attach Reject, UE Context Release Command)", got)
	}

	for i := range 3 {
		req := parseESMInformationRequest(t, ue, cc.sent[i])
		if req.PTI != 3 {
			t.Errorf("attempt %d PTI = %d, want 3", i, req.PTI)
		}
	}

	rej, err := eps.ParseAttachReject(decodeProtectedDownlink(t, ue, cc.sent[3]))
	if err != nil {
		t.Fatalf("not an Attach Reject: %v", err)
	}

	if rej.Cause != eps.EMMCauseESMFailure {
		t.Fatalf("Attach Reject EMM cause = %d, want %d", rej.Cause, eps.EMMCauseESMFailure)
	}

	esm, err := eps.ParsePDNConnectivityReject(rej.ESMMessageContainer)
	if err != nil {
		t.Fatalf("ESM message container is not a PDN Connectivity Reject: %v", err)
	}

	if esm.Cause != eps.ESMCauseESMInformationNotReceived {
		t.Errorf("carried ESM cause = %d, want %d", esm.Cause, eps.ESMCauseESMInformationNotReceived)
	}

	if ue.PendingESMInfo() != nil {
		t.Error("the ESM information procedure is still outstanding after its final expiry")
	}
}

// The response stops T3489, so no retransmission follows a resumed attach.
func TestESMInformationResponseStopsT3489(t *testing.T) {
	m := esmInfoTestMME()
	m.SetT3489ConfigForTest(5*time.Millisecond, 2)

	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue)

	apn := eps.APN("internet")
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{
		PTI:             3,
		AccessPointName: &apn,
	})

	if ue.Conn().T3489ActiveForTest() {
		t.Fatal("T3489 is still armed after the ESM Information Response")
	}

	settled := cc.count()

	time.Sleep(30 * time.Millisecond)

	if got := cc.count(); got != settled {
		t.Errorf("message count grew from %d to %d after the response; T3489 kept firing", settled, got)
	}
}

// The ESM INFORMATION RESPONSE and T3489's final expiry race for the same
// transaction. Exactly one concludes it, so the attach is never both resumed and
// rejected.
func TestESMInformationResponseRacesTheTimeout(t *testing.T) {
	for range 200 {
		m := esmInfoTestMME()
		ue, _ := esmInfoAttachUe(t, m, 3)

		activateDefaultBearer(context.Background(), m, ue)

		var (
			concluded = make(chan bool, 2)
			start     = make(chan struct{})
		)

		go func() {
			<-start

			concluded <- ue.TakeESMInfoWaitFor(3) != nil
		}()

		go func() {
			<-start

			concluded <- ue.TakeESMInfoWait() != nil
		}()

		close(start)

		if won := <-concluded; won == (<-concluded) {
			t.Fatalf("both racers concluded the transaction (won=%v); exactly one must", won)
		}
	}
}
