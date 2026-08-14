// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

func TestHandoverFromEPSCommitsWhileAPolicyChangeRacesIt(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc, ref, _, _ := prepareArrival(t, s, upf)
	before := invariantsOf(t, sc)

	ack, err := buildHandoverRequestAcknowledgeTransfer(targetGnbTEID, targetGnbIPv4)
	if err != nil {
		t.Fatalf("build the Handover Request Acknowledge transfer: %v", err)
	}

	if _, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	var (
		wg          sync.WaitGroup
		start       = make(chan struct{})
		handoverErr error
	)

	wg.Add(2)

	go func() {
		defer wg.Done()

		<-start

		handoverErr = s.UpdateSmContextN2HandoverComplete(ctx, ref)
	}()

	go func() {
		defer wg.Done()

		<-start

		_ = s.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
			SmContextRef: ref,
			Reason:       models.ReconcilePolicyChange,
			NewPolicy: &models.SessionPolicyDelta{
				SessionAmbrUplink:   "100 Mbps",
				SessionAmbrDownlink: "200 Mbps",
				Var5qi:              6,
				Arp:                 1,
			},
		})
	}()

	close(start)
	wg.Wait()

	if handoverErr != nil {
		t.Fatalf("UpdateSmContextN2HandoverComplete: %v", handoverErr)
	}

	if after := invariantsOf(t, sc); after != before {
		t.Errorf("session invariants changed across the handover: %+v, want %+v", after, before)
	}

	sc.Mutex.Lock()
	access, ebi := sc.Access, sc.EBI
	an, dpAccess := sc.Tunnel.AN, sc.Tunnel.Access
	sc.Mutex.Unlock()

	if access != smf.Access5G {
		t.Errorf("session is on %s after the UE arrived on the target NG-RAN node", access)
	}

	if ebi != epsTestEBI {
		t.Errorf("session EPS bearer identity = %d, want the %d it arrived with", ebi, epsTestEBI)
	}

	if an.TEID != targetGnbTEID || !an.IPv4.Equal(targetGnbIPv4) {
		t.Errorf("downlink points at %s/%#x, want the target gNB %s/%#x", an.IPv4, an.TEID, targetGnbIPv4, targetGnbTEID)
	}

	if dpAccess != smf.Access5G {
		t.Errorf("the downlink is still addressed as %s after the session moved onto 5GS", dpAccess)
	}

	if s.SessionCount() != 1 {
		t.Errorf("sessions = %d, want 1", s.SessionCount())
	}
}
