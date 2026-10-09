// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

const dedicatedUplinkTEID = 0x1000 + 16

var voiceARP = models.Arp{PriorityLevel: 2, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable}

func voiceRule() smf.PCCRule {
	rate := models.BitRateFromBps(41000)

	return smf.PCCRule{
		ID:  "af;1#1",
		QCI: 1,
		ARP: voiceARP,
		MBR: models.Ambr{Uplink: rate, Downlink: rate},
		GBR: models.Ambr{Uplink: rate, Downlink: rate},
		Filters: []models.SDFFilter{
			{Direction: models.FilterDownlink, Precedence: 32, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), LocalPort: 50000, RemotePort: 49000},
			{Direction: models.FilterUplink, Precedence: 33, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), LocalPort: 50000, RemotePort: 49000},
		},
	}
}

var nextRevision uint64 = 1 << 40

func pushRules(t *testing.T, s *smf.SMF, ref string, rules ...smf.PCCRule) {
	t.Helper()

	policy := committedPolicy(s, ref)
	nextRevision++

	d := &smf.PolicyDecision{
		Revision:    nextRevision,
		PolicyID:    policy.PolicyID,
		Var5qi:      policy.QosData.Var5qi,
		Arp:         policy.QosData.Arp.PriorityLevel,
		SessionAMBR: policy.Ambr,
		Rules:       rules,
	}

	if err := s.UpdateNotify(context.Background(), ref, d); err != nil {
		t.Fatalf("UpdateNotify: %v", err)
	}
}

func (f *fakePCF) reportedRules() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.failedRules)
}

func TestVoiceRuleActivatesADedicatedBearer(t *testing.T) {
	s, pcf, upf, mmeCb, ref := epsReconcileFixture(t, 0)

	pushRules(t, s, ref, voiceRule())

	if m := lastModify(t, upf); len(bearerUplinkPDRs(m)) != 1 {
		t.Fatalf("UPF modification %+v adds no uplink endpoint for the bearer", m.UpdatePDRs)
	}

	acts := mmeCb.dedicatedActivations()
	if len(acts) != 1 {
		t.Fatalf("activations %+v, want one", acts)
	}

	a := acts[0]
	if a.LinkedEBI != epsTestEBI || a.QCI != 1 || a.ARP != voiceARP || a.SGW.TEID != dedicatedUplinkTEID || a.SessionRef != ref {
		t.Fatalf("activation %+v, want QCI 1 linked to EBI %d on TEID %#x", a, epsTestEBI, dedicatedUplinkTEID)
	}

	if a.GBR.Uplink.Bps() != 41000 || a.MBR.Downlink.Bps() != 41000 {
		t.Fatalf("bitrates MBR %+v GBR %+v, want 41 kbps", a.MBR, a.GBR)
	}

	if len(a.Filters) != 2 || a.Filters[0].Precedence == a.Filters[1].Precedence {
		t.Fatalf("filters %+v, want two with distinct precedences", a.Filters)
	}

	enb := models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}
	if err := s.DedicatedBearerActivated(context.Background(), ref, dedicatedUplinkTEID, 6, enb); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	pushRules(t, s, ref, voiceRule())

	if n := len(mmeCb.dedicatedActivations()); n != 1 {
		t.Fatalf("%d activations after an unchanged decision, want 1", n)
	}

	pushRules(t, s, ref)

	if d := mmeCb.dedicatedDeactivations(); !slices.Equal(d, []uint8{6}) {
		t.Fatalf("deactivations %v, want EBI 6", d)
	}

	s.DedicatedBearerReleased(context.Background(), ref, dedicatedUplinkTEID)

	if m := lastModify(t, upf); !slices.Contains(m.RemovePDRs, 256) || !slices.Contains(m.RemoveQERs, 256) || len(bearerUplinkPDRs(m)) != 0 {
		t.Fatalf("UPF modification removes PDRs %v QERs %v, want the bearer's rule", m.RemovePDRs, m.RemoveQERs)
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("a requested release reported failed rules %v", r)
	}
}

func TestFailedDedicatedBearerReportsItsRules(t *testing.T) {
	s, pcf, upf, _, ref := epsReconcileFixture(t, 0)

	pushRules(t, s, ref, voiceRule())

	s.DedicatedBearerReleased(context.Background(), ref, dedicatedUplinkTEID)

	if r := pcf.reportedRules(); !slices.Equal(r, []string{"af;1#1"}) {
		t.Fatalf("reported rules %v, want af;1#1", r)
	}

	if m := lastModify(t, upf); !slices.Contains(m.RemovePDRs, 256) {
		t.Fatalf("UPF modification removes PDRs %v, want the failed bearer's", m.RemovePDRs)
	}

	if err := s.DedicatedBearerActivated(context.Background(), ref, dedicatedUplinkTEID, 6, models.FTEID{}); err == nil {
		t.Fatal("a released bearer was activated")
	}
}

func TestDedicatedBearerWaitsForAnIdleUE(t *testing.T) {
	s, _, _, mmeCb, ref := epsReconcileFixture(t, 0)

	mmeCb.mu.Lock()
	mmeCb.activateErr = smf.ErrUENotReachable
	mmeCb.mu.Unlock()

	pushRules(t, s, ref, voiceRule())

	mmeCb.mu.Lock()
	mmeCb.activateErr = nil
	mmeCb.mu.Unlock()

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	acts := mmeCb.dedicatedActivations()
	if len(acts) != 1 || acts[0].SGW.TEID != dedicatedUplinkTEID {
		t.Fatalf("activations %+v, want one on the bearer's original endpoint", acts)
	}
}

func TestVoiceRulesAreIgnoredOn5G(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	mmeCb := &fakeMME{}
	s.SetMME(mmeCb)

	_, ref := setupSessionWithTunnel(t, s)

	policy := currentPolicy(s, ref)

	d := &smf.PolicyDecision{
		Revision:    1 << 41,
		PolicyID:    policy.PolicyID,
		Var5qi:      policy.QosData.Var5qi,
		Arp:         policy.QosData.Arp.PriorityLevel,
		SessionAMBR: policy.Ambr,
		Rules:       []smf.PCCRule{voiceRule()},
	}

	if err := s.UpdateNotify(context.Background(), ref, d); err != nil {
		t.Fatal(err)
	}

	if n := len(mmeCb.dedicatedActivations()); n != 0 {
		t.Fatalf("%d dedicated bearer activations for a 5G session", n)
	}
}

func TestFailedRuleIsNotRetriedUntilItChanges(t *testing.T) {
	s, _, _, mmeCb, ref := epsReconcileFixture(t, 0)

	pushRules(t, s, ref, voiceRule())
	s.DedicatedBearerReleased(context.Background(), ref, dedicatedUplinkTEID)

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	if n := len(mmeCb.dedicatedActivations()); n != 1 {
		t.Fatalf("%d activations after the failure, want the failed rule left alone", n)
	}

	changed := voiceRule()
	changed.Filters[0].LocalPort = 50002
	pushRules(t, s, ref, changed)

	if n := len(mmeCb.dedicatedActivations()); n != 2 {
		t.Fatalf("%d activations after the rule changed, want a new attempt", n)
	}
}

func TestIdleUEDedicatedBearerFailsAfterTheDeadline(t *testing.T) {
	s, pcf, _, mmeCb, ref := epsReconcileFixture(t, 0)
	s.SetDedicatedAwaitLimitForTest(10 * time.Millisecond)

	mmeCb.mu.Lock()
	mmeCb.activateErr = smf.ErrUENotReachable
	mmeCb.mu.Unlock()

	pushRules(t, s, ref, voiceRule())

	deadline := time.Now().Add(5 * time.Second)

	for len(pcf.reportedRules()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the bearer waiting for an unreachable UE never failed")
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func TestReleasedBearerIsReplannedForANewRule(t *testing.T) {
	s, _, _, mmeCb, ref := epsReconcileFixture(t, 0)

	pushRules(t, s, ref, voiceRule())

	if err := s.DedicatedBearerActivated(context.Background(), ref, dedicatedUplinkTEID, 6, models.FTEID{}); err != nil {
		t.Fatal(err)
	}

	pushRules(t, s, ref)

	next := voiceRule()
	next.ID = "af;2#1"
	pushRules(t, s, ref, next)

	s.DedicatedBearerReleased(context.Background(), ref, dedicatedUplinkTEID)

	deadline := time.Now().Add(5 * time.Second)

	for len(mmeCb.dedicatedActivations()) < 2 {
		if time.Now().After(deadline) {
			t.Fatal("the rule that arrived during the release got no bearer")
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func bearerUplinkPDRs(m *models.ModifyRequest) []models.PDR {
	var out []models.PDR

	for _, p := range m.UpdatePDRs {
		if p.PDI.LocalFTEID != nil && p.PDI.LocalFTEID.ChooseID == 16 {
			out = append(out, p)
		}
	}

	return out
}

func bearerDownlink(m *models.ModifyRequest) (models.PDR, models.FAR, bool) {
	i := slices.IndexFunc(m.UpdatePDRs, func(p models.PDR) bool { return p.FARID == 16 && p.PDI.UEIPAddress.IsValid() })
	j := slices.IndexFunc(m.UpdateFARs, func(f models.FAR) bool { return f.FARID == 16 })

	if i < 0 || j < 0 {
		return models.PDR{}, models.FAR{}, false
	}

	return m.UpdatePDRs[i], m.UpdateFARs[j], true
}

func TestActiveBearerCarriesItsDownlink(t *testing.T) {
	s, _, upf, _, ref := epsReconcileFixture(t, 0)
	ctx := context.Background()

	pushRules(t, s, ref, voiceRule())

	m := lastModify(t, upf)
	if _, _, ok := bearerDownlink(m); ok {
		t.Fatalf("UPF modification %+v, want the downlink on the default bearer until the voice bearer is established (TS 23.401 §5.4.1)", m.UpdatePDRs)
	}

	uplink := bearerUplinkPDRs(m)[0]
	if len(uplink.PDI.SDFFilters) != 1 || uplink.PDI.SDFFilters[0].Direction != models.FilterUplink {
		t.Fatalf("bearer uplink PDR filters %+v, want the uplink filter only", uplink.PDI.SDFFilters)
	}

	enb := models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}
	if err := s.DedicatedBearerActivated(ctx, ref, dedicatedUplinkTEID, 6, enb); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	m = lastModify(t, upf)

	pdr, far, ok := bearerDownlink(m)
	if !ok {
		t.Fatalf("UPF modification %+v carries no downlink PDR for the active bearer", m.UpdatePDRs)
	}

	if pdr.QERID != bearerUplinkPDRs(m)[0].QERID || pdr.URRID != 2 || pdr.Precedence != 32 || len(pdr.PDI.SDFFilters) != 1 || pdr.PDI.SDFFilters[0].Direction != models.FilterDownlink {
		t.Fatalf("bearer downlink PDR %+v, want the rule's QER, URR 2, precedence 32 and the downlink filter", pdr)
	}

	ohc := far.ForwardingParameters.OuterHeaderCreation
	if ohc == nil || ohc.TEID != 0x66 || !ohc.S1U || !ohc.IPv4Address.Equal(netip.MustParseAddr("10.3.0.3").AsSlice()) || !far.ApplyAction.Forw {
		t.Fatalf("bearer FAR %+v, want forwarding to the eNB's S1-U endpoint", far)
	}

	if q := m.UpdateQERs[slices.IndexFunc(m.UpdateQERs, func(q models.QER) bool { return q.QERID == pdr.QERID })]; q.AveragingWindow == nil || *q.AveragingWindow != 2*time.Second {
		t.Fatalf("bearer QER averaging window %v, want 2 s", q.AveragingWindow)
	}

	if err := s.DeactivateEPSSession(ctx, ref); err != nil {
		t.Fatalf("DeactivateEPSSession: %v", err)
	}

	if _, far, ok := bearerDownlink(lastModify(t, upf)); !ok || far.ApplyAction != (models.ApplyAction{Buff: true, Nocp: true}) {
		t.Fatalf("idle UE's bearer downlink FAR %+v, want buffering with a report naming the bearer (TS 23.401 §5.3.4.3)", far)
	}

	if err := s.ModifyEPSSession(ctx, ref, epsTestEBI, models.FTEID{TEID: 0x77, Addr: netip.MustParseAddr("10.3.0.4")}); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	if _, far, ok := bearerDownlink(lastModify(t, upf)); ok {
		t.Fatalf("bearer FAR %+v: the downlink resumed before the voice bearer had a radio endpoint", far)
	}

	if err := s.DedicatedBearerActivated(ctx, ref, dedicatedUplinkTEID, 6, models.FTEID{TEID: 0x78, Addr: netip.MustParseAddr("10.3.0.4")}); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	if _, far, ok := bearerDownlink(lastModify(t, upf)); !ok || !far.ApplyAction.Forw || far.ForwardingParameters.OuterHeaderCreation.TEID != 0x78 {
		t.Fatalf("bearer FAR %+v, want the downlink forwarded to its new eNB endpoint", far)
	}
}

func TestPagingFailureOnTheVoiceBearerFindsItsSession(t *testing.T) {
	s, _, upf, _, ref := activeVoiceSession(t)

	if err := s.HandleEPSPagingFailure(context.Background(), s.GetSession(ref).Supi.IMSI(), 6, models.EPSPagingUENotResponding); err != nil {
		t.Fatalf("HandleEPSPagingFailure on the voice EBI: %v", err)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if seid := s.GetSession(ref).PFCPContext.SEID; !slices.Equal(upf.suppressDDNCalls, []uint64{seid}) {
		t.Fatalf("suppressed DDNs for %v, want the voice bearer's session %d", upf.suppressDDNCalls, seid)
	}
}

func TestErrorIndicationOnTheBearerTunnelBuffersTheUE(t *testing.T) {
	s, _, upf, mmeCb, ref := epsReconcileFixture(t, 0)
	ctx := context.Background()

	pushRules(t, s, ref, voiceRule())

	enb := models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}
	if err := s.DedicatedBearerActivated(ctx, ref, dedicatedUplinkTEID, 6, enb); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	seid := s.GetSession(ref).PFCPContext.SEID

	stale := &models.ErrorIndicationReport{SEID: seid, FARID: 16, RemoteFTEID: models.FTEID{TEID: 0x99, Addr: enb.Addr}}
	if err := s.HandleErrorIndicationReport(ctx, stale); err != nil {
		t.Fatalf("HandleErrorIndicationReport: %v", err)
	}

	if n := len(mmeCb.notifyCauses); n != 0 {
		t.Fatalf("a report naming no current tunnel notified the MME %d times", n)
	}

	if err := s.HandleErrorIndicationReport(ctx, &models.ErrorIndicationReport{SEID: seid, FARID: 16, RemoteFTEID: enb}); err != nil {
		t.Fatalf("HandleErrorIndicationReport: %v", err)
	}

	if _, far, ok := bearerDownlink(lastModify(t, upf)); !ok || far.ApplyAction.Forw {
		t.Fatalf("bearer FAR %+v: the broken bearer tunnel still receives downlink", far)
	}

	if !slices.Equal(mmeCb.notifyCauses, []models.DownlinkDataNotificationCause{models.DownlinkDataErrorIndication}) {
		t.Fatalf("MME notifications %v, want one Error Indication", mmeCb.notifyCauses)
	}

	if ebis := mmeCb.notifiedEBIs(); !slices.Equal(ebis, []uint8{6}) {
		t.Fatalf("notified EBIs %v, want the voice bearer the Error Indication named (TS 29.274 Table 7.2.11.1-1)", ebis)
	}
}

func TestDefaultBearerMoveKeepsTheVoiceDownlink(t *testing.T) {
	s, _, upf, _, ref := epsReconcileFixture(t, 0)
	ctx := context.Background()

	pushRules(t, s, ref, voiceRule())

	menb := models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}
	if err := s.DedicatedBearerActivated(ctx, ref, dedicatedUplinkTEID, 6, menb); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	if err := s.ModifyEPSSession(ctx, ref, epsTestEBI, models.FTEID{TEID: 0x99, Addr: netip.MustParseAddr("10.3.0.9")}); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	m := lastModify(t, upf)

	_, far, ok := bearerDownlink(m)
	if !ok || far.ForwardingParameters.OuterHeaderCreation.TEID != 0x66 {
		t.Fatalf("moving the default bearer moved or dropped the voice downlink: %+v", m.UpdateFARs)
	}

	if slices.Contains(m.RemovePDRs, 32) {
		t.Fatal("moving the default bearer removed the voice downlink PDR")
	}
}

func TestVoiceBearerSwitchSendsEndMarkers(t *testing.T) {
	s, _, upf, _, ref := epsReconcileFixture(t, 0)
	ctx := context.Background()

	pushRules(t, s, ref, voiceRule())

	if err := s.DedicatedBearerActivated(ctx, ref, dedicatedUplinkTEID, 6, models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	if m := lastModify(t, upf); m.SendEndMarkers {
		t.Fatal("the first downlink endpoint of the bearer asked for end markers")
	}

	if err := s.DedicatedBearerActivated(ctx, ref, dedicatedUplinkTEID, 6, models.FTEID{TEID: 0x67, Addr: netip.MustParseAddr("10.3.0.4")}); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	m := lastModify(t, upf)
	if _, far, ok := bearerDownlink(m); !ok || far.ForwardingParameters.OuterHeaderCreation.TEID != 0x67 || !m.SendEndMarkers {
		t.Fatalf("switching the voice bearer's eNB endpoint: FARs %+v, end markers %v; want the new endpoint and end markers", m.UpdateFARs, m.SendEndMarkers)
	}
}

func TestBearersBeyondTheUserPlaneCapacityFail(t *testing.T) {
	s, pcf, _, mmeCb, ref := epsReconcileFixture(t, 0)

	wideRule := func(id string, priority int32, base uint16) smf.PCCRule {
		r := voiceRule()
		r.ID = id
		r.ARP.PriorityLevel = priority
		r.Filters = nil

		for i := range uint16(6) {
			remote := netip.MustParsePrefix("192.0.2.10/32")
			r.Filters = append(r.Filters,
				models.SDFFilter{Direction: models.FilterDownlink, Protocol: 17, Remote: remote, LocalPort: base + i, RemotePort: 49000},
				models.SDFFilter{Direction: models.FilterUplink, Protocol: 17, Remote: remote, LocalPort: base + i, RemotePort: 49000})
		}

		return r
	}

	pushRules(t, s, ref, wideRule("a#1", 2, 50000), wideRule("b#1", 3, 51000), wideRule("c#1", 4, 52000))

	if n := len(mmeCb.dedicatedActivations()); n != 2 {
		t.Fatalf("%d activations, want 2: 36 filters exceed the user plane's %d", n, models.MaxSessionSDFRules)
	}

	if r := pcf.reportedRules(); len(r) != 1 {
		t.Fatalf("reported rules %v, want the one that did not fit", r)
	}
}

func secondCallRule() smf.PCCRule {
	r := voiceRule()
	r.ID = "af;2#1"
	r.Filters = []models.SDFFilter{
		{Direction: models.FilterDownlink, Precedence: 32, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.20/32"), LocalPort: 50002, RemotePort: 49002},
		{Direction: models.FilterUplink, Precedence: 33, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.20/32"), LocalPort: 50002, RemotePort: 49002},
	}

	return r
}

func activeVoiceSession(t *testing.T) (*smf.SMF, *fakePCF, *fakeUPF, *fakeMME, string) {
	t.Helper()

	s, pcf, upf, mmeCb, ref := epsReconcileFixture(t, 0)

	pushRules(t, s, ref, voiceRule())

	if err := s.DedicatedBearerActivated(context.Background(), ref, dedicatedUplinkTEID, 6, models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	return s, pcf, upf, mmeCb, ref
}

func waitDedicatedModifications(t *testing.T, mmeCb *fakeMME, n int) []models.DedicatedBearerModification {
	t.Helper()

	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if got := mmeCb.dedicatedModifications(); len(got) >= n {
			return got
		}
	}

	t.Fatalf("%d dedicated bearer modifications sent, want %d", len(mmeCb.dedicatedModifications()), n)

	return nil
}

func bearerUplinkFilters(t *testing.T, upf *fakeUPF) int {
	t.Helper()

	pdrs := bearerUplinkPDRs(lastModify(t, upf))
	if len(pdrs) == 0 {
		t.Fatal("no bearer uplink PDR")
	}

	n := 0
	for _, p := range pdrs {
		n += len(p.PDI.SDFFilters)
	}

	return n
}

func TestSecondCallAndHangUpModifyTheSharedBearer(t *testing.T) {
	s, _, upf, mmeCb, ref := activeVoiceSession(t)
	ctx := context.Background()

	pushRules(t, s, ref, voiceRule(), secondCallRule())

	add := waitDedicatedModifications(t, mmeCb, 1)[0]
	if add.Operation != models.TFTAddFilters || len(add.Filters) != 2 || add.Filters[0].ID != 3 || add.Filters[1].ID != 4 || !add.QoSChanged || add.GBR.Downlink.Bps() != 82000 {
		t.Fatalf("modification %+v, want filters 3 and 4 added with an 82 kbps GBR", add)
	}

	if n := len(mmeCb.dedicatedActivations()); n != 1 {
		t.Fatalf("%d activations, want the second call on the existing bearer (IR.92 §4.4)", n)
	}

	if n := bearerUplinkFilters(t, upf); n != 2 {
		t.Fatalf("bearer uplink PDR has %d filters while the UE has not accepted, want old and new (2)", n)
	}

	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, true)

	pushRules(t, s, ref, secondCallRule())

	del := waitDedicatedModifications(t, mmeCb, 2)[1]
	if del.Operation != models.TFTDeleteFilters || !slices.Equal(del.DeleteIDs, []uint8{1, 2}) || del.GBR.Downlink.Bps() != 41000 {
		t.Fatalf("modification %+v, want filters 1 and 2 deleted with a 41 kbps GBR", del)
	}

	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, true)

	if n := bearerUplinkFilters(t, upf); n != 1 {
		t.Fatalf("bearer uplink PDR has %d filters after the first call ended, want 1", n)
	}

	if n := len(mmeCb.dedicatedDeactivations()); n != 0 {
		t.Fatalf("the bearer was deactivated while the second call goes on")
	}
}

func TestPortChangeReplacesFiltersInPlace(t *testing.T) {
	s, _, _, mmeCb, ref := activeVoiceSession(t)

	moved := voiceRule()
	for i := range moved.Filters {
		moved.Filters[i].LocalPort = 50010
	}

	pushRules(t, s, ref, moved)

	mod := waitDedicatedModifications(t, mmeCb, 1)[0]
	if mod.Operation != models.TFTReplaceFilters || len(mod.Filters) != 2 || mod.Filters[0].ID != 1 || mod.Filters[1].ID != 2 || mod.QoSChanged {
		t.Fatalf("modification %+v, want filters 1 and 2 replaced without a QoS change (TS 23.401 §5.4.3)", mod)
	}

	if mod.Filters[0].Precedence != 32 || mod.Filters[1].Precedence != 33 {
		t.Fatalf("precedences %d/%d, want the original 32/33", mod.Filters[0].Precedence, mod.Filters[1].Precedence)
	}
}

func TestSwappedCallAddsBeforeItDeletes(t *testing.T) {
	s, _, _, mmeCb, ref := activeVoiceSession(t)
	ctx := context.Background()

	pushRules(t, s, ref, secondCallRule())

	if first := waitDedicatedModifications(t, mmeCb, 1)[0]; first.Operation != models.TFTAddFilters {
		t.Fatalf("first step %+v, want the additions so the TFT never empties (TS 24.301 §6.4.3.4 a3)", first)
	}

	s.DedicatedBearerModified(ctx, ref, dedicatedUplinkTEID, true)

	if second := waitDedicatedModifications(t, mmeCb, 2)[1]; second.Operation != models.TFTDeleteFilters || !slices.Equal(second.DeleteIDs, []uint8{1, 2}) {
		t.Fatalf("second step %+v, want filters 1 and 2 deleted", second)
	}
}

func TestRejectedModificationKeepsTheFirstCall(t *testing.T) {
	s, pcf, upf, mmeCb, ref := activeVoiceSession(t)

	pushRules(t, s, ref, voiceRule(), secondCallRule())
	waitDedicatedModifications(t, mmeCb, 1)

	s.DedicatedBearerModified(context.Background(), ref, dedicatedUplinkTEID, false)

	if r := pcf.reportedRules(); !slices.Equal(r, []string{"af;2#1"}) {
		t.Fatalf("reported rules %v, want only the second call's", r)
	}

	if n := bearerUplinkFilters(t, upf); n != 1 {
		t.Fatalf("bearer uplink PDR has %d filters after the rejection, want the first call's 1", n)
	}

	if n := len(mmeCb.dedicatedDeactivations()); n != 0 {
		t.Fatal("a rejected modification deactivated the bearer")
	}
}

func TestVoiceBearerGetsItsOwnForwardingTunnel(t *testing.T) {
	s, _, upf, _, ref := activeVoiceSession(t)
	ctx := context.Background()

	target := models.FTEID{TEID: 0x88, Addr: netip.MustParseAddr("10.3.0.8")}

	local, err := s.OpenEPSForwardingTunnel(ctx, ref, 6, target)
	if err != nil {
		t.Fatalf("OpenEPSForwardingTunnel: %v", err)
	}

	if local.TEID != 0x1000+32 {
		t.Fatalf("forwarding TEID %#x, want the bearer's forwarding PDR's", local.TEID)
	}

	m := lastModify(t, upf)

	i := slices.IndexFunc(m.UpdateFARs, func(f models.FAR) bool { return f.FARID == 32 })
	if i < 0 || m.UpdateFARs[i].ForwardingParameters.OuterHeaderCreation.TEID != 0x88 {
		t.Fatalf("FARs %+v, want the voice bearer's forwarding FAR to the target eNB", m.UpdateFARs)
	}

	if slices.ContainsFunc(m.UpdatePDRs, func(p models.PDR) bool { return p.PDRID == 4 }) {
		t.Fatal("opening the voice bearer's tunnel also opened the session's")
	}

	if err := s.CloseEPSForwardingTunnel(ctx, ref); err != nil {
		t.Fatalf("CloseEPSForwardingTunnel: %v", err)
	}

	if m := lastModify(t, upf); !slices.Contains(m.RemovePDRs, 64) || !slices.Contains(m.RemoveFARs, 32) {
		t.Fatalf("closing removes PDRs %v FARs %v, want the bearer's forwarding PDR and FAR", m.RemovePDRs, m.RemoveFARs)
	}
}

func TestBufferedVoiceIsReportedOnTheVoiceBearer(t *testing.T) {
	s, _, _, mmeCb, ref := activeVoiceSession(t)
	ctx := context.Background()

	if err := s.DeactivateEPSSession(ctx, ref); err != nil {
		t.Fatalf("DeactivateEPSSession: %v", err)
	}

	seid := s.GetSession(ref).PFCPContext.SEID

	if err := s.HandleDownlinkDataReport(ctx, &models.DownlinkDataReport{SEID: seid, PDRID: 257}); err != nil {
		t.Fatalf("HandleDownlinkDataReport: %v", err)
	}

	if err := s.HandleDownlinkDataReport(ctx, &models.DownlinkDataReport{SEID: seid, PDRID: 2}); err != nil {
		t.Fatalf("HandleDownlinkDataReport: %v", err)
	}

	if got := mmeCb.notifiedEBIs(); !slices.Equal(got, []uint8{6, epsTestEBI}) {
		t.Fatalf("Downlink Data Notifications for EBIs %v, want the voice bearer's then the default's (TS 23.401 §5.3.4.3)", got)
	}
}
