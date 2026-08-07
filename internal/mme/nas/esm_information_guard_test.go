// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/nas/eps"
)

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

func TestT3489RetransmitsTwiceThenRejects(t *testing.T) {
	m := esmInfoTestMME()
	m.SetT3489ConfigForTest(5*time.Millisecond, 2)

	ue, cc := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue)

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

func TestS1ReleaseDropsTheESMInformationWait(t *testing.T) {
	m := esmInfoTestMME()
	ue, _ := esmInfoAttachUe(t, m, 3)

	activateDefaultBearer(context.Background(), m, ue)

	if ue.PendingESMInfo() == nil {
		t.Fatal("the ESM information procedure is not outstanding before the release")
	}

	m.FreeUeConn(ue)

	if ue.PendingESMInfo() != nil {
		t.Error("the ESM information wait survived the S1 release")
	}
}
