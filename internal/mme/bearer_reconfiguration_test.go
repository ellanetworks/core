// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel/trace"
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

func TestModifyEPSBearerCommitsAnIdleUEsARPOnlyChange(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)
	p := testPDN(ue)
	p.Qci = 9
	p.SessionRef = "ref-internet"

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{QoS: &models.EPSBearerQoS{QCI: 9, ARP: 3}}); err != nil {
		t.Fatalf("ModifyEPSBearer: %v", err)
	}

	if m.pagingActive(ue) {
		t.Fatal("the idle UE was paged for an ARP-only change")
	}

	if p.Arp != 3 {
		t.Fatalf("ARP = %d, want the committed 3", p.Arp)
	}

	want := []bearerModificationOutcome{{ref: "ref-internet", accepted: true}}
	if got := m.Session.(*fakeSessionManager).outcomes(); !slices.Equal(got, want) {
		t.Fatalf("SMF told %+v, want %+v", got, want)
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

func TestUnansweredERABModifyAbandonsTheModification(t *testing.T) {
	m := newTestMME(t)
	m.SetESMGuardConfigForTest(20*time.Millisecond, 0)

	ue, _ := connectedBearerUE(t, m)
	p := testPDN(ue)

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{QoS: &models.EPSBearerQoS{QCI: 8, ARP: 2}}); err != nil {
		t.Fatal(err)
	}

	m.StopESMGuard(p)
	m.ConcludeBearerModification(context.Background(), ue, p, true)

	want := []bearerModificationOutcome{{ref: "ref-internet", accepted: false}}
	if got := waitForOutcome(m); !slices.Equal(got, want) {
		t.Fatalf("SMF told %+v, want %+v once the eNB never answered", got, want)
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	if p.Modifying != nil || p.Qci != 9 {
		t.Fatalf("modification left Modifying %+v and QCI %d, want nil and 9", p.Modifying, p.Qci)
	}
}

func TestPDNDisconnectEndsAnInFlightModification(t *testing.T) {
	m := newTestMME(t)
	ue, _ := connectedBearerUE(t, m)

	ims := ue.EnsurePDN(6)
	ims.Apn = "ims"
	ims.SessionRef = "ref-ims"

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), 6, models.EPSBearerModification{APNAMBR: testAmbr("300 Mbps", "400 Mbps")}); err != nil {
		t.Fatal(err)
	}

	m.DisconnectBearer(context.Background(), ue, ims, 36, 3)

	defer m.StopESMGuard(ims)

	if ims.Modifying != nil {
		t.Fatal("the modification outlived the PDN disconnect")
	}

	want := []bearerModificationOutcome{{ref: "ref-ims", accepted: false}}
	if got := m.Session.(*fakeSessionManager).outcomes(); !slices.Equal(got, want) {
		t.Fatalf("SMF told %+v, want %+v", got, want)
	}
}

func TestUnansweredSignallingPageSuppressesNoDownlinkData(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)
	testPDN(ue).Qci = 9

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{APNAMBR: testAmbr("1 Gbps", "1 Gbps")}); !errors.Is(err, ErrUENotReachable) {
		t.Fatalf("ModifyEPSBearer error = %v, want ErrUENotReachable", err)
	}

	m.abandonPaging(trace.SpanContext{}, ue, ue.paging.attempt)

	if got := m.Session.(*fakeSessionManager).suppressCalls; got != 0 {
		t.Fatalf("downlink notifications suppressed %d times after a signalling page", got)
	}
}
