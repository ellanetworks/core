// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func activeVoiceBearer(t *testing.T) (*MME, *UeContext, *captureConn, *fakeSessionManager) {
	t.Helper()

	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)
	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6}}})

	return m, ue, cc, m.Session.(*fakeSessionManager)
}

func secondCallFilters() []models.SDFFilter {
	remote := netip.MustParsePrefix("192.0.2.20/32")

	return []models.SDFFilter{
		{ID: 3, Direction: models.FilterDownlink, Precedence: 34, Protocol: 17, Remote: remote, LocalPort: 50002, RemotePort: 49002},
		{ID: 4, Direction: models.FilterUplink, Precedence: 35, Protocol: 17, Remote: remote, LocalPort: 50002, RemotePort: 49002},
	}
}

func lastSent(cc *captureConn) []byte {
	sent := cc.snapshot()
	return sent[len(sent)-1]
}

func waitModification(t *testing.T, fake *fakeSessionManager) []modificationOutcome {
	t.Helper()

	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if got := fake.modifications(); len(got) >= 1 {
			return got
		}
	}

	return fake.modifications()
}

func TestSecondCallModifiesTheVoiceBearer(t *testing.T) {
	m, ue, cc, fake := activeVoiceBearer(t)

	rate := models.BitRateFromBps(88000)
	mod := models.DedicatedBearerModification{
		SessionRef: "ref-internet", SGWTEID: voiceSGWTEID,
		QoSChanged: true, MBR: models.Ambr{Uplink: rate, Downlink: rate}, GBR: models.Ambr{Uplink: rate, Downlink: rate},
		Operation: models.TFTAddFilters, Filters: secondCallFilters(),
	}

	if err := m.ModifyDedicatedBearer(context.Background(), ue.imsiOrEmpty(), 6, mod); err != nil {
		t.Fatalf("ModifyDedicatedBearer: %v", err)
	}

	erab := sentERABModify(t, lastSent(cc))
	if len(erab.ERABToBeModified) != 1 {
		t.Fatalf("E-RAB Modify items %+v, want the voice bearer", erab.ERABToBeModified)
	}

	item := erab.ERABToBeModified[0]
	if item.ERABID != 6 || item.QoS.QCI != 1 || item.QoS.GBR == nil || item.QoS.GBR.GuaranteedBitrateDL != 88000 {
		t.Fatalf("E-RAB Modify item %+v, want EBI 6 at QCI 1 with an 88 kbps GBR", item)
	}

	req := sentModifyRequest(t, ue, []byte(item.NASPDU))
	if req.EPSBearerIdentity != 6 || req.NewEPSQoS == nil || req.TFT == nil || req.TFT.Operation != eps.TFTAddFilters || len(req.TFT.Filters) != 2 || req.TFT.Filters[0].Identifier != 3 {
		t.Fatalf("Modify EPS Bearer Context Request %+v, want new QoS and the two added filters", req)
	}

	if !m.DedicatedBearerModifyAccepted(context.Background(), ue, 6) {
		t.Fatal("the accept was not matched to the modification")
	}

	if got := fake.modifications(); len(got) != 0 {
		t.Fatalf("modification reported %+v before the eNB answered", got)
	}

	m.RadioBearerModified(context.Background(), ue, 6, true)

	if got := waitModification(t, fake); len(got) != 1 || !got[0].accepted || got[0].teid != voiceSGWTEID {
		t.Fatalf("SMF told %+v, want one accepted modification", got)
	}

	_, b := m.LookupDedicated(ue, 6)
	if len(b.Filters) != 4 || b.GBR.Downlink.Bps() != 88000 {
		t.Fatalf("bearer after modification: %d filters, GBR %v; want 4 filters and 88 kbps", len(b.Filters), b.GBR)
	}
}

func TestTFTOnlyModificationUsesDownlinkNAS(t *testing.T) {
	m, ue, cc, fake := activeVoiceBearer(t)

	mod := models.DedicatedBearerModification{SessionRef: "ref-internet", SGWTEID: voiceSGWTEID, Operation: models.TFTDeleteFilters, DeleteIDs: []uint8{1}}

	if err := m.ModifyDedicatedBearer(context.Background(), ue.imsiOrEmpty(), 6, mod); err != nil {
		t.Fatalf("ModifyDedicatedBearer: %v", err)
	}

	req := sentModifyRequest(t, ue, decodeDownlinkNAS(t, lastSent(cc)))
	if req.NewEPSQoS != nil || req.TFT == nil || req.TFT.Operation != eps.TFTDeleteFilters || len(req.TFT.DeleteIdentifiers) != 1 {
		t.Fatalf("Modify EPS Bearer Context Request %+v, want only the filter deletion (TS 23.401 §5.4.3)", req)
	}

	m.DedicatedBearerModifyAccepted(context.Background(), ue, 6)

	if got := waitModification(t, fake); len(got) != 1 || !got[0].accepted {
		t.Fatalf("SMF told %+v, want one accepted modification", got)
	}

	if _, b := m.LookupDedicated(ue, 6); len(b.Filters) != 1 || b.Filters[0].ID != 2 {
		t.Fatalf("filters after deletion %+v, want only filter 2", b.Filters)
	}
}

func qosModification() models.DedicatedBearerModification {
	rate := models.BitRateFromBps(88000)

	return models.DedicatedBearerModification{
		SessionRef: "ref-internet", SGWTEID: voiceSGWTEID,
		QoSChanged: true, MBR: models.Ambr{Uplink: rate, Downlink: rate}, GBR: models.Ambr{Uplink: rate, Downlink: rate},
		Operation: models.TFTAddFilters, Filters: secondCallFilters(),
	}
}

func requirePreviousConfiguration(t *testing.T, m *MME, ue *UeContext) {
	t.Helper()

	_, b := m.LookupDedicated(ue, 6)
	if b == nil || len(b.Filters) != 2 || b.GBR.Downlink.Bps() != 41000 {
		t.Fatalf("bearer after a failed modification: %+v, want the previous configuration (TS 24.301 §6.4.3.4)", b)
	}
}

func requireDeactivationSent(t *testing.T, ue *UeContext, cc *captureConn) {
	t.Helper()

	cmd, err := s1ap.ParseERABReleaseCommand(initiating(t, lastSent(cc), s1ap.ProcERABRelease))
	if err != nil {
		t.Fatalf("parse E-RAB Release Command: %v", err)
	}

	if deact, err := eps.ParseDeactivateEPSBearerContextRequest(downlinkPlain(t, ue, []byte(cmd.NASPDU))); err != nil || deact.EPSBearerIdentity != 6 {
		t.Fatalf("deactivation %+v (%v), want EBI 6", deact, err)
	}
}

func TestFailedModificationKeepsTheBearer(t *testing.T) {
	tests := []struct {
		name string
		fail func(m *MME, ue *UeContext)
	}{
		{"eNB fails the E-RAB", func(m *MME, ue *UeContext) { m.RadioBearerModified(context.Background(), ue, 6, false) }},
		{"ESM STATUS and the eNB fails the E-RAB", func(m *MME, ue *UeContext) {
			m.DedicatedESMStatus(context.Background(), ue, 6, eps.ESMCauseSemanticErrorInTFT)
			m.RadioBearerModified(context.Background(), ue, 6, false)
		}},
		{"user-inactivity release before the UE answered", func(m *MME, ue *UeContext) {
			ue.Conn().releaseCause.Store(&s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUserInactivity})
			m.FreeUeConn(context.Background(), ue)
		}},
		{"handover before the eNB answered", func(m *MME, ue *UeContext) { m.ResumeBearerReconfigurationAfterHandover(context.Background(), ue) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, ue, _, fake := activeVoiceBearer(t)

			if err := m.ModifyDedicatedBearer(context.Background(), ue.imsiOrEmpty(), 6, qosModification()); err != nil {
				t.Fatalf("ModifyDedicatedBearer: %v", err)
			}

			tc.fail(m, ue)

			if got := waitModification(t, fake); len(got) != 1 || got[0].accepted {
				t.Fatalf("SMF told %+v, want one failed modification", got)
			}

			requirePreviousConfiguration(t, m, ue)
		})
	}
}

func TestT3486ExpiryFailsTheModification(t *testing.T) {
	m, ue, cc, fake := activeVoiceBearer(t)
	m.esmGuardCfg.ExpireTime = 5 * time.Millisecond
	m.esmGuardCfg.MaxRetryTimes = 2

	mod := models.DedicatedBearerModification{SessionRef: "ref-internet", SGWTEID: voiceSGWTEID, Operation: models.TFTDeleteFilters, DeleteIDs: []uint8{1}}
	if err := m.ModifyDedicatedBearer(context.Background(), ue.imsiOrEmpty(), 6, mod); err != nil {
		t.Fatalf("ModifyDedicatedBearer: %v", err)
	}

	sent := cc.count()

	if got := waitModification(t, fake); len(got) != 1 || got[0].accepted {
		t.Fatalf("SMF told %+v, want the modification failed on the fifth expiry (TS 24.301 §6.4.3.6)", got)
	}

	if retransmitted := cc.count() - sent; retransmitted != 2 {
		t.Fatalf("%d retransmissions, want 2", retransmitted)
	}

	requirePreviousConfiguration(t, m, ue)
}

func TestRejectedModificationRealignsTheERAB(t *testing.T) {
	m, ue, cc, fake := activeVoiceBearer(t)
	ctx := context.Background()

	if err := m.ModifyDedicatedBearer(ctx, ue.imsiOrEmpty(), 6, qosModification()); err != nil {
		t.Fatalf("ModifyDedicatedBearer: %v", err)
	}

	m.RadioBearerModified(ctx, ue, 6, true)
	m.DedicatedBearerModifyRejected(ctx, ue, 6, eps.ESMCauseSemanticErrorInTFT)

	if got := waitModification(t, fake); len(got) != 1 || got[0].accepted {
		t.Fatalf("SMF told %+v, want one failed modification", got)
	}

	erab := sentERABModify(t, lastSent(cc))
	if item := erab.ERABToBeModified[0]; item.ERABID != 6 || item.QoS.GBR == nil || item.QoS.GBR.GuaranteedBitrateDL != 41000 {
		t.Fatalf("realignment %+v, want the E-RAB back at the committed 41 kbps (TS 24.301 §6.4.3.4)", item)
	}

	if req := sentModifyRequest(t, ue, []byte(erab.ERABToBeModified[0].NASPDU)); req.NewEPSQoS == nil || req.TFT != nil {
		t.Fatalf("realignment NAS %+v, want the committed QoS and no TFT change", req)
	}

	m.RadioBearerModified(ctx, ue, 6, true)
	m.DedicatedBearerModifyAccepted(ctx, ue, 6)

	time.Sleep(50 * time.Millisecond)

	if got := fake.modifications(); len(got) != 1 {
		t.Fatalf("SMF told %+v, want the realignment kept internal", got)
	}

	requirePreviousConfiguration(t, m, ue)
}

func TestFailedRealignmentDeactivatesTheBearer(t *testing.T) {
	m, ue, cc, _ := activeVoiceBearer(t)
	ctx := context.Background()

	if err := m.ModifyDedicatedBearer(ctx, ue.imsiOrEmpty(), 6, qosModification()); err != nil {
		t.Fatalf("ModifyDedicatedBearer: %v", err)
	}

	m.RadioBearerModified(ctx, ue, 6, true)
	m.DedicatedBearerModifyRejected(ctx, ue, 6, eps.ESMCauseSemanticErrorInTFT)
	m.RadioBearerModified(ctx, ue, 6, false)

	requireDeactivationSent(t, ue, cc)
}

func TestAcceptedModificationTheENBDidNotApplyDeactivatesTheBearer(t *testing.T) {
	m, ue, cc, _ := activeVoiceBearer(t)
	ctx := context.Background()

	if err := m.ModifyDedicatedBearer(ctx, ue.imsiOrEmpty(), 6, qosModification()); err != nil {
		t.Fatalf("ModifyDedicatedBearer: %v", err)
	}

	m.DedicatedBearerModifyAccepted(ctx, ue, 6)
	m.RadioBearerModified(ctx, ue, 6, false)

	requireDeactivationSent(t, ue, cc)
}

func TestAcceptedModificationSurvivesAnS1Release(t *testing.T) {
	m, ue, _, fake := activeVoiceBearer(t)
	ctx := context.Background()

	if err := m.ModifyDedicatedBearer(ctx, ue.imsiOrEmpty(), 6, qosModification()); err != nil {
		t.Fatalf("ModifyDedicatedBearer: %v", err)
	}

	m.DedicatedBearerModifyAccepted(ctx, ue, 6)

	ue.Conn().releaseCause.Store(&s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUserInactivity})
	m.FreeUeConn(ctx, ue)

	if got := waitModification(t, fake); len(got) != 1 || !got[0].accepted {
		t.Fatalf("SMF told %+v, want the modification the UE accepted committed", got)
	}

	if _, b := m.LookupDedicated(ue, 6); b == nil || len(b.Filters) != 4 || b.GBR.Downlink.Bps() != 88000 {
		t.Fatalf("bearer %+v, want the modified configuration", b)
	}
}

func TestModificationRetransmitsOnTheConnectionAfterAHandover(t *testing.T) {
	m, ue, _, fake := activeVoiceBearer(t)
	m.esmGuardCfg.ExpireTime = 10 * time.Millisecond
	m.esmGuardCfg.MaxRetryTimes = 1

	mod := models.DedicatedBearerModification{SessionRef: "ref-internet", SGWTEID: voiceSGWTEID, Operation: models.TFTDeleteFilters, DeleteIDs: []uint8{1}}
	if err := m.ModifyDedicatedBearer(context.Background(), ue.imsiOrEmpty(), 6, mod); err != nil {
		t.Fatalf("ModifyDedicatedBearer: %v", err)
	}

	target := &captureConn{}
	c := m.NewUeConn(target, 9)
	c.ue.Store(ue)
	ue.active.Store(c)

	if got := waitModification(t, fake); len(got) != 1 || got[0].accepted {
		t.Fatalf("SMF told %+v, want T3486 to run to its end on the new connection", got)
	}

	if target.count() != 1 {
		t.Fatalf("target eNB got %d messages, want the retransmission", target.count())
	}
}

func TestModificationThatEmptiesTheTFTIsRefused(t *testing.T) {
	m, ue, _, _ := activeVoiceBearer(t)

	mod := models.DedicatedBearerModification{SessionRef: "ref-internet", SGWTEID: voiceSGWTEID, Operation: models.TFTDeleteFilters, DeleteIDs: []uint8{1, 2}}
	if err := m.ModifyDedicatedBearer(context.Background(), ue.imsiOrEmpty(), 6, mod); !errors.Is(err, ErrInvalidModification) {
		t.Fatalf("error = %v, want ErrInvalidModification", err)
	}

	mod.SGWTEID = voiceSGWTEID + 1
	mod.DeleteIDs = []uint8{1}

	if err := m.ModifyDedicatedBearer(context.Background(), ue.imsiOrEmpty(), 6, mod); err != ErrNoDedicatedBearer {
		t.Fatalf("error = %v for another bearer's TEID, want ErrNoDedicatedBearer", err)
	}
}

func TestModificationOnAnUnknownBearerIsRefused(t *testing.T) {
	m, ue, _, _ := activeVoiceBearer(t)

	if err := m.ModifyDedicatedBearer(context.Background(), ue.imsiOrEmpty(), 7, models.DedicatedBearerModification{SessionRef: "ref-internet"}); err != ErrNoDedicatedBearer {
		t.Fatalf("error = %v, want ErrNoDedicatedBearer", err)
	}
}

func arpModification() models.DedicatedBearerModification {
	return models.DedicatedBearerModification{
		SessionRef: "ref-internet", SGWTEID: voiceSGWTEID,
		ARP: &models.Arp{PriorityLevel: 1, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable},
	}
}

func TestARPChangeReconfiguresTheERAB(t *testing.T) {
	m, ue, cc, fake := activeVoiceBearer(t)
	ctx := context.Background()

	if err := m.ModifyDedicatedBearer(ctx, ue.imsiOrEmpty(), 6, arpModification()); err != nil {
		t.Fatalf("ModifyDedicatedBearer: %v", err)
	}

	erab := sentERABModify(t, lastSent(cc))
	if item := erab.ERABToBeModified[0]; item.QoS.ARP.PriorityLevel != 1 {
		t.Fatalf("E-RAB Modify QoS %+v, want ARP priority 1 (TS 23.401 §5.4.2.1)", item.QoS)
	}

	if req := sentModifyRequest(t, ue, []byte(erab.ERABToBeModified[0].NASPDU)); req.NewEPSQoS == nil || req.TFT != nil {
		t.Fatalf("Modify EPS Bearer Context Request %+v, want the EPS QoS (without ARP) and no TFT", req)
	}

	m.RadioBearerModified(ctx, ue, 6, true)
	m.DedicatedBearerModifyAccepted(ctx, ue, 6)

	if got := waitModification(t, fake); len(got) != 1 || !got[0].accepted {
		t.Fatalf("SMF told %+v, want the modification accepted", got)
	}

	if _, b := m.LookupDedicated(ue, 6); b.ARP.PriorityLevel != 1 {
		t.Fatalf("bearer ARP %+v, want priority 1", b.ARP)
	}
}

func TestIdleARPChangeIsCommittedWithoutPaging(t *testing.T) {
	m, ue, cc, fake := activeVoiceBearer(t)
	ctx := context.Background()

	ue.Conn().releaseCause.Store(&s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUserInactivity})
	m.FreeUeConn(ctx, ue)

	sent := cc.count()

	if err := m.ModifyDedicatedBearer(ctx, ue.imsiOrEmpty(), 6, arpModification()); err != nil {
		t.Fatalf("ModifyDedicatedBearer: %v", err)
	}

	if got := waitModification(t, fake); len(got) != 1 || !got[0].accepted {
		t.Fatalf("SMF told %+v, want the ARP change committed (TS 23.401 §5.4.2.1 step 3)", got)
	}

	if cc.count() != sent || ue.paging.guard.Active() {
		t.Fatal("an ARP-only change paged the idle UE")
	}

	if _, b := m.LookupDedicated(ue, 6); b.ARP.PriorityLevel != 1 {
		t.Fatalf("bearer ARP %+v, want priority 1", b.ARP)
	}
}

func TestRANEndpointsBindTheVoiceBearerWithItsPDNConnection(t *testing.T) {
	m, ue, _, fake := activeVoiceBearer(t)

	defaultENB := models.FTEID{TEID: 0x81, Addr: netip.MustParseAddr("10.3.0.4")}
	voiceENB := models.FTEID{TEID: 0x82, Addr: netip.MustParseAddr("10.3.0.4")}

	m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{Present: []RANBearer{{Ebi: 6, EnbFTEID: voiceENB}, {Ebi: DefaultERABID, EnbFTEID: defaultENB}}})

	want := []models.DedicatedBearerEndpoint{{SGWTEID: voiceSGWTEID, ENB: voiceENB}}
	if fake.modifiedENB != defaultENB || !slices.Equal(fake.boundDedicated, want) {
		t.Fatalf("default bound to %+v with %+v, want %+v with %+v so the voice downlink never takes the default bearer", fake.modifiedENB, fake.boundDedicated, defaultENB, want)
	}
}
