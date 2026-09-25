// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

func TestModifyEPSBearerPagesAnIdleUE(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)
	testPDN(ue).Qci = 9

	err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{APNAMBR: testAmbr("1 Gbps", "1 Gbps")})
	if !errors.Is(err, ErrUENotReachable) {
		t.Fatalf("ModifyEPSBearer error = %v, want ErrUENotReachable", err)
	}

	if !m.pagingActive(ue) {
		t.Fatal("the idle UE was not paged for the bearer modification")
	}
}

func TestModifyEPSBearerDoesNotPageForAnARPOnlyChange(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)
	testPDN(ue).Qci = 9

	err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{QoS: &models.EPSBearerQoS{QCI: 9, ARP: 3}})
	if !errors.Is(err, ErrUENotReachable) {
		t.Fatalf("ModifyEPSBearer error = %v, want ErrUENotReachable", err)
	}

	if m.pagingActive(ue) {
		t.Fatal("the idle UE was paged for an ARP-only change")
	}
}

func TestReactivateEPSBearerPagesAnIdleUE(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	if err := m.ReactivateEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID); !errors.Is(err, ErrUENotReachable) {
		t.Fatalf("ReactivateEPSBearer error = %v, want ErrUENotReachable", err)
	}

	if !m.pagingActive(ue) {
		t.Fatal("the idle UE was not paged for the reactivation")
	}
}

func TestQoSModificationCommitsOnlyOnceTheENBAndTheUEAgree(t *testing.T) {
	m := newTestMME(t)
	ue, _ := connectedBearerUE(t, m)
	p := testPDN(ue)
	fake := m.Session.(*fakeSessionManager)

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{QoS: &models.EPSBearerQoS{QCI: 8, ARP: 2}}); err != nil {
		t.Fatal(err)
	}

	defer ue.Conn().StopNASGuard(t.Context())

	m.ConcludeBearerModification(context.Background(), ue, p, true)

	if got := fake.outcomes(); len(got) != 0 || p.Qci != 9 {
		t.Fatalf("committed before the eNB answered: SMF told %+v, QCI %d", got, p.Qci)
	}

	m.RadioBearerModified(context.Background(), ue, DefaultERABID, true)

	want := []bearerModificationOutcome{{ref: "ref-internet", accepted: true}}
	if got := fake.outcomes(); !slices.Equal(got, want) || p.Qci != 8 {
		t.Fatalf("after both answers: SMF told %+v, QCI %d, want %+v and 8", got, p.Qci, want)
	}
}

func TestFailedERABModifyAbandonsTheModification(t *testing.T) {
	m := newTestMME(t)
	ue, _ := connectedBearerUE(t, m)
	p := testPDN(ue)
	fake := m.Session.(*fakeSessionManager)

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{QoS: &models.EPSBearerQoS{QCI: 8, ARP: 2}}); err != nil {
		t.Fatal(err)
	}

	m.RadioBearerModified(context.Background(), ue, DefaultERABID, false)

	want := []bearerModificationOutcome{{ref: "ref-internet", accepted: false}}
	if got := fake.outcomes(); !slices.Equal(got, want) {
		t.Fatalf("SMF told %+v, want %+v", got, want)
	}

	if p.Modifying != nil || p.Qci != 9 {
		t.Fatalf("modification left Modifying %+v and QCI %d, want nil and 9", p.Modifying, p.Qci)
	}
}

func TestHandoverResumesAnInterruptedModification(t *testing.T) {
	m := newTestMME(t)
	ue, _ := connectedBearerUE(t, m)
	p := testPDN(ue)
	fake := m.Session.(*fakeSessionManager)

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{APNAMBR: testAmbr("300 Mbps", "400 Mbps")}); err != nil {
		t.Fatal(err)
	}

	m.ResumeBearerReconfigurationAfterHandover(context.Background(), ue)

	if p.Modifying != nil {
		t.Fatal("the interrupted modification is still outstanding")
	}

	want := []bearerModificationOutcome{{ref: "ref-internet", accepted: false}}
	if got := fake.outcomes(); !slices.Equal(got, want) {
		t.Fatalf("SMF told %+v, want %+v", got, want)
	}

	if !slices.Contains(fake.reconciled, "ref-internet") {
		t.Fatal("the bearer was not reconciled after the handover")
	}
}

func TestRANUEAMBRIsTheSumOfAPNAMBRsCappedBySubscription(t *testing.T) {
	m := newTestMME(t)
	ue, _ := connectedBearerUE(t, m)

	ims := ue.EnsurePDN(6)
	ims.SessionRef = "ref-ims"
	ims.SessAmbrDLBps = 50_000_000
	ims.SessAmbrULBps = 20_000_000

	ambr := ue.SetSubscribedUEAMBR(models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("1 Gbps")})

	if ambr.Downlink.Bps() != 250_000_000 || ambr.Uplink.Bps() != 100_000_000 {
		t.Fatalf("RAN UE-AMBR = %s/%s, want 250 Mbps down (sum) and 100 Mbps up (subscribed)", ambr.Downlink, ambr.Uplink)
	}
}

func sentUEContextModification(t *testing.T, cc *captureConn) *s1ap.UEContextModificationRequest {
	t.Helper()

	for _, b := range slices.Backward(cc.sent) {
		pdu, err := s1ap.Unmarshal(b)
		if err != nil {
			continue
		}

		if im, ok := pdu.(*s1ap.InitiatingMessage); ok && im.ProcedureCode == s1ap.ProcUEContextModification {
			req, err := s1ap.ParseUEContextModificationRequest(im.Value)
			if err != nil {
				t.Fatalf("parse UE Context Modification Request: %v", err)
			}

			return req
		}
	}

	return nil
}

func TestAcceptedAPNAMBRChangeSignalsTheUEAMBR(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	ue.SetSubscribedUEAMBR(models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")})

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{APNAMBR: testAmbr("300 Mbps", "400 Mbps")}); err != nil {
		t.Fatal(err)
	}

	defer ue.Conn().StopNASGuard(t.Context())

	m.ConcludeBearerModification(context.Background(), ue, testPDN(ue), true)

	req := sentUEContextModification(t, cc)
	if req == nil || req.UEAggregateMaximumBitRate == nil {
		t.Fatal("no UE Context Modification carried the new UE-AMBR")
	}

	if req.UEAggregateMaximumBitRate.DL != 400_000_000 || req.UEAggregateMaximumBitRate.UL != 300_000_000 {
		t.Fatalf("UE-AMBR = %d/%d, want 400/300 Mbps", req.UEAggregateMaximumBitRate.DL, req.UEAggregateMaximumBitRate.UL)
	}
}

func TestRefreshUEAMBRsSignalsASubscriptionChange(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	ue.SetSubscribedUEAMBR(models.Ambr{Uplink: models.MustParseBitRate("10 Mbps"), Downlink: models.MustParseBitRate("10 Mbps")})

	m.RefreshUEAMBRs(context.Background())

	req := sentUEContextModification(t, cc)
	if req == nil || req.UEAggregateMaximumBitRate == nil {
		t.Fatal("no UE Context Modification carried the new UE-AMBR")
	}

	if req.UEAggregateMaximumBitRate.DL != 200_000_000 || req.UEAggregateMaximumBitRate.UL != 100_000_000 {
		t.Fatalf("UE-AMBR = %d/%d, want the APN-AMBR sum 200/100 Mbps under the 1 Gbps subscription", req.UEAggregateMaximumBitRate.DL, req.UEAggregateMaximumBitRate.UL)
	}

	before := len(cc.sent)

	m.RefreshUEAMBRs(context.Background())

	if len(cc.sent) != before {
		t.Fatal("an unchanged UE-AMBR was signalled again")
	}
}
