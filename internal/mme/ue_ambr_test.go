// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

func ueAMBRUE(t *testing.T, m *MME) (*UeContext, *captureConn) {
	t.Helper()

	ue, cc := connectedBearerUE(t, m)
	ue.Ambr = testAmbr("1 Gbps", "1 Gbps")
	ue.Conn().holdUEAMBR(new(S1APUEAMBR(ue.RANUEAMBR())))

	return ue, cc
}

func addIMSPDN(ue *UeContext) *PdnConnection {
	ims := ue.EnsurePDN(6)
	ims.Apn = "ims"
	ims.SessionRef = "ref-ims"
	ims.SessAmbrDLBps = 300_000_000
	ims.SessAmbrULBps = 300_000_000

	return ims
}

func sentUEContextModification(t *testing.T, pdu []byte) *s1ap.UEContextModificationRequest {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal S1AP: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcUEContextModification {
		t.Fatalf("got %T, want UE Context Modification Request", msg)
	}

	req, err := s1ap.ParseUEContextModificationRequest(im.Value)
	if err != nil {
		t.Fatalf("parse UE Context Modification Request: %v", err)
	}

	return req
}

func assertS1APUEAMBR(t *testing.T, got *s1ap.UEAggregateMaximumBitRate, dl, ul uint64) {
	t.Helper()

	if got == nil {
		t.Fatal("UE-AMBR IE absent")
	}

	if uint64(got.DL) != dl || uint64(got.UL) != ul {
		t.Fatalf("UE-AMBR = %d/%d bit/s (DL/UL), want %d/%d", got.DL, got.UL, dl, ul)
	}
}

func assertHeldUEAMBR(t *testing.T, c *UeConn, dl, ul uint64) {
	t.Helper()

	held, ok := c.HeldUEAMBR()
	if !ok {
		t.Fatal("no UE-AMBR recorded for the connection")
	}

	if held.Downlink.Bps() != dl || held.Uplink.Bps() != ul {
		t.Fatalf("held UE-AMBR = %s/%s (DL/UL), want %d/%d bit/s", held.Downlink, held.Uplink, dl, ul)
	}
}

func TestSyncUEAMBRIsSilentWhenTheENBHoldsTheValue(t *testing.T) {
	m := newTestMME(t)
	ue, cc := ueAMBRUE(t, m)

	m.SyncUEAMBR(context.Background(), ue)

	if cc.count() != 0 {
		t.Fatalf("sent %d messages for an unchanged UE-AMBR", cc.count())
	}
}

func TestSyncUEAMBRSignalsASubscriptionChange(t *testing.T) {
	m := newTestMME(t)
	ue, cc := ueAMBRUE(t, m)

	ue.SetSubscribedUEAMBR(*testAmbr("40 Mbps", "50 Mbps"))
	m.SyncUEAMBR(context.Background(), ue)

	if cc.count() != 1 {
		t.Fatalf("expected one UE Context Modification Request, got %d", cc.count())
	}

	req := sentUEContextModification(t, cc.snapshot()[0])
	if req.MMEUES1APID != ue.Conn().MMEUES1APID || req.ENBUES1APID != ue.Conn().ENBUES1APID() {
		t.Fatalf("UE S1AP IDs = %d/%d, want %d/%d", req.MMEUES1APID, req.ENBUES1APID, ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID())
	}

	assertS1APUEAMBR(t, req.UEAggregateMaximumBitRate, 50_000_000, 40_000_000)
	assertHeldUEAMBR(t, ue.Conn(), 50_000_000, 40_000_000)

	m.SyncUEAMBR(context.Background(), ue)

	if cc.count() != 1 {
		t.Fatalf("re-signalled a UE-AMBR the eNB already holds (%d messages)", cc.count())
	}
}

func TestSyncUEAMBRWaitsForInitialContextSetup(t *testing.T) {
	m := newTestMME(t)
	ue, cc := ueAMBRUE(t, m)

	ue.Conn().SetICS(ICSPending)
	ue.SetSubscribedUEAMBR(*testAmbr("50 Mbps", "50 Mbps"))
	m.SyncUEAMBR(context.Background(), ue)

	if cc.count() != 0 {
		t.Fatalf("sent %d messages while the Initial Context Setup was pending", cc.count())
	}

	ue.Conn().SetICS(ICSCompleted)
	m.SyncUEAMBR(context.Background(), ue)

	if cc.count() != 1 {
		t.Fatalf("expected one UE Context Modification Request once the context was set up, got %d", cc.count())
	}
}

func TestSyncUEAMBRDefersDuringAnS1Handover(t *testing.T) {
	m := newTestMME(t)
	ue, cc := ueAMBRUE(t, m)

	ue.handover = &handoverContext{state: hoPreparing}
	ue.SetSubscribedUEAMBR(*testAmbr("50 Mbps", "50 Mbps"))
	m.SyncUEAMBR(context.Background(), ue)

	if cc.count() != 0 {
		t.Fatalf("sent %d messages during a handover", cc.count())
	}
}

func TestSyncUEAMBRResendsAfterAFailure(t *testing.T) {
	m := newTestMME(t)
	ue, cc := ueAMBRUE(t, m)

	ue.Conn().ForgetUEAMBR()
	m.SyncUEAMBR(context.Background(), ue)

	if cc.count() != 1 {
		t.Fatalf("expected the UE-AMBR to be re-signalled after a failure, got %d messages", cc.count())
	}

	assertS1APUEAMBR(t, sentUEContextModification(t, cc.snapshot()[0]).UEAggregateMaximumBitRate, 200_000_000, 100_000_000)
}

func TestReleasingAnAdditionalPDNResignalsTheUEAMBR(t *testing.T) {
	m := newTestMME(t)
	ue, cc := ueAMBRUE(t, m)
	ims := addIMSPDN(ue)
	ue.Conn().holdUEAMBR(new(S1APUEAMBR(ue.RANUEAMBR())))

	m.ReleasePDN(context.Background(), ue, ims)

	if cc.count() != 1 {
		t.Fatalf("expected one UE Context Modification Request, got %d", cc.count())
	}

	assertS1APUEAMBR(t, sentUEContextModification(t, cc.snapshot()[0]).UEAggregateMaximumBitRate, 200_000_000, 100_000_000)
}

func TestERABReleaseCarriesTheUEAMBRWithoutThePDN(t *testing.T) {
	m := newTestMME(t)
	ue, cc := ueAMBRUE(t, m)
	ims := addIMSPDN(ue)
	ue.Conn().holdUEAMBR(new(S1APUEAMBR(ue.RANUEAMBR())))

	if err := m.ReactivateEPSBearer(context.Background(), ue.imsiOrEmpty(), ims.Ebi); err != nil {
		t.Fatal(err)
	}

	defer m.StopESMGuard(ims)

	if cc.count() != 1 {
		t.Fatalf("expected one E-RAB Release Command, got %d", cc.count())
	}

	msg, err := s1ap.Unmarshal(cc.snapshot()[0])
	if err != nil {
		t.Fatal(err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcERABRelease {
		t.Fatalf("got %T, want E-RAB Release Command", msg)
	}

	cmd, err := s1ap.ParseERABReleaseCommand(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	assertS1APUEAMBR(t, cmd.UEAggregateMaximumBitRate, 200_000_000, 100_000_000)
	assertHeldUEAMBR(t, ue.Conn(), 200_000_000, 100_000_000)

	m.ReleasePDN(context.Background(), ue, ims)

	if cc.count() != 1 {
		t.Fatalf("the PDN's removal re-signalled a UE-AMBR the E-RAB Release already carried (%d messages)", cc.count())
	}
}

func TestERABModifyCarriesTheUpdatedUEAMBR(t *testing.T) {
	m := newTestMME(t)
	ue, cc := ueAMBRUE(t, m)

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{
		QoS:     &models.EPSBearerQoS{QCI: 8, ARP: 2},
		APNAMBR: testAmbr("300 Mbps", "400 Mbps"),
	}); err != nil {
		t.Fatal(err)
	}

	defer m.StopESMGuard(testPDN(ue))

	if cc.count() != 1 {
		t.Fatalf("expected one E-RAB Modify Request, got %d", cc.count())
	}

	req := sentERABModify(t, cc.snapshot()[0])
	assertS1APUEAMBR(t, req.UEAggregateMaximumBitRate, 400_000_000, 300_000_000)
}

func TestAcceptedAPNAMBRChangeResignalsTheUEAMBR(t *testing.T) {
	m := newTestMME(t)
	ue, cc := ueAMBRUE(t, m)
	p := testPDN(ue)

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{
		APNAMBR: testAmbr("300 Mbps", "400 Mbps"),
	}); err != nil {
		t.Fatal(err)
	}

	m.StopESMGuard(p)

	if cc.count() != 1 {
		t.Fatalf("expected one Modify EPS Bearer Context Request, got %d", cc.count())
	}

	m.ConcludeBearerModification(context.Background(), ue, p, true)

	if cc.count() != 2 {
		t.Fatalf("expected a UE Context Modification Request after the UE accepted, got %d messages", cc.count())
	}

	assertS1APUEAMBR(t, sentUEContextModification(t, cc.snapshot()[1]).UEAggregateMaximumBitRate, 400_000_000, 300_000_000)
}

func TestRejectedAPNAMBRChangeLeavesTheUEAMBR(t *testing.T) {
	m := newTestMME(t)
	ue, cc := ueAMBRUE(t, m)
	p := testPDN(ue)

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{
		APNAMBR: testAmbr("300 Mbps", "400 Mbps"),
	}); err != nil {
		t.Fatal(err)
	}

	m.StopESMGuard(p)
	m.ConcludeBearerModification(context.Background(), ue, p, false)

	if cc.count() != 1 {
		t.Fatalf("a rejected APN-AMBR change was followed by %d more messages", cc.count()-1)
	}
}

func TestReconcileUEAMBRAppliesTheProfile(t *testing.T) {
	m := newTestMME(t)
	ue, cc := ueAMBRUE(t, m)

	ue.SetSubscribedUEAMBR(*testAmbr("50 Mbps", "50 Mbps"))
	ue.Conn().holdUEAMBR(new(S1APUEAMBR(ue.RANUEAMBR())))

	m.ReconcileUEAMBR(context.Background())

	if cc.count() != 1 {
		t.Fatalf("expected one UE Context Modification Request, got %d", cc.count())
	}

	assertS1APUEAMBR(t, sentUEContextModification(t, cc.snapshot()[0]).UEAggregateMaximumBitRate, 200_000_000, 100_000_000)

	if ul, dl := ue.AmbrRates(); ul.Bps() != 1_000_000_000 || dl.Bps() != 1_000_000_000 {
		t.Fatalf("subscribed UE-AMBR = %s/%s, want the profile's 1 Gbps", ul, dl)
	}
}
