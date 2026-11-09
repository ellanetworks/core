// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	libngap "github.com/ellanetworks/core/ngap"
)

const (
	markingEF   uint16 = 0xB8FC
	markingAF21 uint16 = 0x48FC
	markingAF11 uint16 = 0x28FC
	markingCS5  uint16 = 0xA0FC

	farIDSessionDownlink uint32 = 2
	farIDEPSBearer       uint32 = 16
	farIDQoSFlow         uint32 = 48
)

var (
	highPriorityARP = models.Arp{PriorityLevel: 3, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityPreemptable}
	lowPriorityARP  = models.Arp{PriorityLevel: 8, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityPreemptable}
)

func bulkDataRule(arp models.Arp, version uint64) smf.PCCRule {
	rate := models.BitRateFromBps(64000)

	return smf.PCCRule{
		ID:      "af;data#1",
		Version: version,
		QCI:     8,
		ARP:     arp,
		MBR:     models.Ambr{Uplink: rate, Downlink: rate},
		Filters: []models.SDFFilter{
			{Direction: models.FilterDownlink, Precedence: 32, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.30/32"), LocalPort: 50010, RemotePort: 49010},
			{Direction: models.FilterUplink, Precedence: 33, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.30/32"), LocalPort: 50010, RemotePort: 49010},
		},
	}
}

func farMarking(t *testing.T, upf *fakeUPF, farID uint32) uint16 {
	t.Helper()

	upf.mu.Lock()
	defer upf.mu.Unlock()

	for i := len(upf.modifyCalls) - 1; i >= 0; i-- {
		for _, f := range upf.modifyCalls[i].UpdateFARs {
			if f.FARID == farID && f.ForwardingParameters != nil {
				return f.ForwardingParameters.TransportLevelMarking
			}
		}
	}

	if upf.lastEstablish != nil {
		for _, f := range upf.lastEstablish.FARs {
			if f.FARID == farID && f.ForwardingParameters != nil {
				return f.ForwardingParameters.TransportLevelMarking
			}
		}
	}

	t.Fatalf("FAR %d was never installed in the UPF", farID)

	return 0
}

func whileUPFRefuses(t *testing.T, upf *fakeUPF, refused func()) {
	t.Helper()

	upf.mu.Lock()
	upf.err = errors.New("upf unavailable")
	before := len(upf.modifyCalls)
	upf.mu.Unlock()

	refused()

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) == before {
		t.Fatal("the UPF was sent no modification to refuse")
	}

	upf.err = nil
	upf.modifyCalls = upf.modifyCalls[:before]
}

func requireMarking(t *testing.T, upf *fakeUPF, farID uint32, want uint16, when string) {
	t.Helper()

	if got := farMarking(t, upf, farID); got != want {
		t.Fatalf("FAR %d marking %s = %#04x, want %#04x", farID, when, got, want)
	}
}

func TestDefaultQoSFlow5QIChangeRemarksTheDownlinkOnceTheUEAccepts(t *testing.T) {
	s, pcf, upf, _, ref := fiveGVoiceFixture(t)

	requireMarking(t, upf, farIDSessionDownlink, markingAF21, "after establishment")

	if err := reconcileWithPolicy(context.Background(), s, pcf, ref, &policyChange{
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              5,
		Arp:                 1,
	}); err != nil {
		t.Fatal(err)
	}

	requireMarking(t, upf, farIDSessionDownlink, markingAF21, "before the UE accepts")

	ueAnswers(t, s, ref, true)

	requireMarking(t, upf, farIDSessionDownlink, markingCS5, "after the UE accepts")
}

func TestDefaultQoSFlowARPChangeRemarksTheDownlinkOnceTheUEAccepts(t *testing.T) {
	s, pcf, upf, _, ref := fiveGVoiceFixture(t)

	if err := reconcileWithPolicy(context.Background(), s, pcf, ref, &policyChange{
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 8,
	}); err != nil {
		t.Fatal(err)
	}

	requireMarking(t, upf, farIDSessionDownlink, markingAF21, "before the UE accepts")

	ueAnswers(t, s, ref, true)

	requireMarking(t, upf, farIDSessionDownlink, markingAF11, "after the UE accepts")
}

func TestIdleDefaultQoSFlowARPChangeRemarksTheDownlink(t *testing.T) {
	s, pcf, upf, amfCb, ref := fiveGVoiceFixture(t)

	if err := s.DeactivateSmContext(context.Background(), ref, true); err != nil {
		t.Fatal(err)
	}

	if err := reconcileWithPolicy(context.Background(), s, pcf, ref, &policyChange{
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 8,
	}); err != nil {
		t.Fatal(err)
	}

	if got := modifyCallCount(amfCb); got != 0 {
		t.Fatalf("an ARP-only change signalled the idle UE %d times", got)
	}

	requireMarking(t, upf, farIDSessionDownlink, markingAF11, "after the idle ARP change")
}

func TestIdleDefaultQoSFlowARPChangeTheUPFRefusedIsRetried(t *testing.T) {
	s, pcf, upf, _, ref := fiveGVoiceFixture(t)

	if err := s.DeactivateSmContext(context.Background(), ref, true); err != nil {
		t.Fatal(err)
	}

	whileUPFRefuses(t, upf, func() {
		if err := reconcileWithPolicy(context.Background(), s, pcf, ref, &policyChange{
			SessionAmbrUplink:   "100 Mbps",
			SessionAmbrDownlink: "200 Mbps",
			Var5qi:              9,
			Arp:                 8,
		}); err != nil {
			t.Fatal(err)
		}
	})

	requireMarking(t, upf, farIDSessionDownlink, markingAF21, "after the UPF refused the ARP change")

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	requireMarking(t, upf, farIDSessionDownlink, markingAF11, "after the retry")
}

func TestDefaultEPSBearerQCIChangeRemarksTheDownlinkOnceTheUEAccepts(t *testing.T) {
	s, pcf, upf, _, ref := epsReconcileFixture(t, 0)

	requireMarking(t, upf, farIDSessionDownlink, markingAF21, "after attach")

	changePolicy(pcf, func(p *smf.Policy) { p.QosData.Var5qi = 5 })

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	requireMarking(t, upf, farIDSessionDownlink, markingAF21, "before the UE accepts")

	s.CommitEPSBearerModification(context.Background(), ref, true)

	requireMarking(t, upf, farIDSessionDownlink, markingCS5, "after the UE accepts")
}

func TestDefaultEPSBearerARPChangeRemarksTheDownlinkOnceTheUEAccepts(t *testing.T) {
	s, pcf, upf, _, ref := epsReconcileFixture(t, 0)

	changePolicy(pcf, func(p *smf.Policy) { p.QosData.Arp = &lowPriorityARP })

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	requireMarking(t, upf, farIDSessionDownlink, markingAF21, "before the UE accepts")

	s.CommitEPSBearerModification(context.Background(), ref, true)

	requireMarking(t, upf, farIDSessionDownlink, markingAF11, "after the UE accepts")
}

func activeBulkDataBearer(t *testing.T) (*smf.SMF, *fakeUPF, *fakeMME, string) {
	t.Helper()

	s, _, upf, mmeCb, ref := epsReconcileFixture(t, 0)

	pushRules(t, s, ref, bulkDataRule(highPriorityARP, 0))

	if err := s.DedicatedBearerActivated(context.Background(), ref, dedicatedUplinkTEID, 6, models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	requireMarking(t, upf, farIDEPSBearer, markingAF21, "after the bearer activation")

	return s, upf, mmeCb, ref
}

func TestDedicatedBearerARPChangeRemarksItsDownlinkOnceCommitted(t *testing.T) {
	s, upf, mmeCb, ref := activeBulkDataBearer(t)

	pushRules(t, s, ref, bulkDataRule(lowPriorityARP, 1))

	if mod := waitDedicatedModifications(t, mmeCb, 1)[0]; mod.ARP == nil || *mod.ARP != lowPriorityARP {
		t.Fatalf("modification %+v, want the ARP changed in place", mod)
	}

	requireMarking(t, upf, farIDEPSBearer, markingAF21, "before the modification is committed")
	requireMarking(t, upf, farIDSessionDownlink, markingAF21, "of the default bearer")

	s.DedicatedBearerModified(context.Background(), ref, dedicatedUplinkTEID, models.ModificationCommitted)

	requireMarking(t, upf, farIDEPSBearer, markingAF11, "after the modification is committed")
	requireMarking(t, upf, farIDSessionDownlink, markingAF21, "of the default bearer")
}

func TestDedicatedBearerARPChangeKeepsItsMarkingWhenRefused(t *testing.T) {
	s, upf, mmeCb, ref := activeBulkDataBearer(t)

	pushRules(t, s, ref, bulkDataRule(lowPriorityARP, 1))
	waitDedicatedModifications(t, mmeCb, 1)

	s.DedicatedBearerModified(context.Background(), ref, dedicatedUplinkTEID, models.ModificationFailed)

	requireMarking(t, upf, farIDEPSBearer, markingAF21, "after the modification is refused")
}

func activeBulkDataFlow(t *testing.T) (*smf.SMF, *fakeUPF, *fakeAMF, string) {
	t.Helper()

	s, _, upf, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, bulkDataRule(highPriorityARP, 0))
	waitFlowModifications(t, amfCb, 1)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	requireMarking(t, upf, farIDQoSFlow, markingAF21, "after the QoS flow is set up")

	return s, upf, amfCb, ref
}

func TestQoSFlowARPChangeRemarksItsDownlinkOnceTheRANAccepts(t *testing.T) {
	s, upf, amfCb, ref := activeBulkDataFlow(t)

	pushRules(t, s, ref, bulkDataRule(lowPriorityARP, 1))
	waitFlowModifications(t, amfCb, 2)

	requireMarking(t, upf, farIDQoSFlow, markingAF21, "before the RAN accepts")

	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	requireMarking(t, upf, farIDQoSFlow, markingAF11, "after the RAN accepts")
	requireMarking(t, upf, farIDSessionDownlink, markingAF21, "of the default QoS flow")
}

func TestQoSFlowARPChangeKeepsItsMarkingWhenTheRANRefuses(t *testing.T) {
	s, upf, amfCb, ref := activeBulkDataFlow(t)

	pushRules(t, s, ref, bulkDataRule(lowPriorityARP, 1))
	waitFlowModifications(t, amfCb, 2)

	ranAnswers(t, s, ref, nil, []uint8{voiceQFI})

	requireMarking(t, upf, farIDQoSFlow, markingAF21, "after the RAN refuses")
}

func TestIdleQoSFlowARPChangeRemarksItsDownlink(t *testing.T) {
	s, upf, amfCb, ref := activeBulkDataFlow(t)

	if err := s.DeactivateSmContext(context.Background(), ref, false); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	pushRules(t, s, ref, bulkDataRule(lowPriorityARP, 1))

	if n := len(amfCb.modifications()); n != 1 {
		t.Fatalf("%d QoS flow modifications sent, want none for the idle UE's ARP change", n-1)
	}

	requireMarking(t, upf, farIDQoSFlow, markingAF11, "after the idle ARP change")
}

func firstSwitchOfSessionDownlink(t *testing.T, upf *fakeUPF, teid uint32) *models.ModifyRequest {
	t.Helper()

	upf.mu.Lock()
	defer upf.mu.Unlock()

	for _, req := range upf.modifyCalls {
		i := slices.IndexFunc(req.UpdateFARs, func(f models.FAR) bool {
			return f.FARID == farIDSessionDownlink && f.ForwardingParameters != nil &&
				f.ForwardingParameters.OuterHeaderCreation != nil && f.ForwardingParameters.OuterHeaderCreation.TEID == teid
		})
		if i >= 0 {
			return req
		}
	}

	t.Fatalf("the session's downlink FAR was never switched to TEID %#x", teid)

	return nil
}

func requireFlowFollowsTheSwitch(t *testing.T, req *models.ModifyRequest, teid uint32, gnb net.IP) {
	t.Helper()

	i := slices.IndexFunc(req.UpdateFARs, func(f models.FAR) bool { return f.FARID == farIDQoSFlow })
	if i < 0 {
		t.Fatalf("UpdateFARs %+v, want the voice flow's FAR switched along with the session's", req.UpdateFARs)
	}

	fp := req.UpdateFARs[i].ForwardingParameters
	if fp == nil || fp.OuterHeaderCreation == nil || fp.OuterHeaderCreation.TEID != teid || !fp.OuterHeaderCreation.IPv4Address.Equal(gnb) {
		t.Fatalf("the voice flow's FAR tunnels into %+v, want TEID %#x at %s", fp, teid, gnb)
	}

	if fp.TransportLevelMarking != markingEF {
		t.Errorf("the voice flow's FAR marking after the switch = %#04x, want %#04x", fp.TransportLevelMarking, markingEF)
	}

	if !req.SendEndMarkers {
		t.Error("the switch asked for no End Markers on the old N3 tunnel")
	}
}

// TS 23.502 §4.9.1.2.2
func TestXnHandoverMovesTheQoSFlowFARWithItsMarking(t *testing.T) {
	s, _, upf, _, ref := activeFiveGVoice(t)

	requireMarking(t, upf, farIDQoSFlow, markingEF, "before the handover")

	const targetTEID = uint32(0x8001)

	target := net.ParseIP("10.3.0.10").To4()

	n2, err := (&libngap.PathSwitchRequestTransfer{
		DLNGUUPTNLInformation: testTunnel(targetTEID, target),
		QosFlowAccepted:       libngap.QosFlowAcceptedList{{QosFlowIdentifier: 1}, {QosFlowIdentifier: voiceQFI}},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.UpdateSmContextXnHandoverPathSwitchReq(context.Background(), ref, n2); err != nil {
		t.Fatalf("path switch: %v", err)
	}

	requireFlowFollowsTheSwitch(t, firstSwitchOfSessionDownlink(t, upf, targetTEID), targetTEID, target)
}

// TS 23.502 §4.9.1.3.3
func TestN2HandoverMovesTheQoSFlowFARWithItsMarking(t *testing.T) {
	s, _, upf, _, ref := activeFiveGVoice(t)
	ctx := context.Background()

	const targetTEID = uint32(0x8002)

	target := net.ParseIP("10.3.0.11").To4()

	if _, err := s.UpdateSmContextN2HandoverPreparing(ctx, ref, n2HandoverRequired(t, false)); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPreparing: %v", err)
	}

	ack, err := (&libngap.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: testTunnel(targetTEID, target),
		QosFlowSetupResponse: libngap.QosFlowListWithDataForwarding{
			{QosFlowIdentifier: libngap.QosFlowIdentifier(models.DefaultQFI)},
			{QosFlowIdentifier: voiceQFI},
		},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	if err := s.UpdateSmContextN2HandoverComplete(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverComplete: %v", err)
	}

	requireFlowFollowsTheSwitch(t, firstSwitchOfSessionDownlink(t, upf, targetTEID), targetTEID, target)
}

func staleEPSUserPlane(t *testing.T) (*smf.SMF, *fakeUPF) {
	t.Helper()

	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	sc := establishEPSOnENB(t, s)

	requireMarking(t, upf, farIDSessionDownlink, markingAF21, "after attach")

	changePolicy(pcf, func(p *smf.Policy) { p.QosData.Var5qi = 5 })

	if err := s.ReconcileSession(context.Background(), sc.Ref); err != nil {
		t.Fatal(err)
	}

	whileUPFRefuses(t, upf, func() { s.CommitEPSBearerModification(context.Background(), sc.Ref, true) })

	requireMarking(t, upf, farIDSessionDownlink, markingAF21, "after the UPF refused the accepted QCI change")

	return s, upf
}

func requireDefaultFlowFromPolicy(t *testing.T, upf *fakeUPF, when string) {
	t.Helper()

	requireMarking(t, upf, farIDSessionDownlink, markingCS5, when)

	req := lastModify(t, upf)

	i := slices.IndexFunc(req.UpdateQERs, func(q models.QER) bool { return q.QERID == 1 })
	if i < 0 || req.UpdateQERs[i].FiveQI != 5 {
		t.Fatalf("UpdateQERs %+v %s, want the default QER at the policy's 5QI 5", req.UpdateQERs, when)
	}
}

// TS 23.502 §4.11.1.2.2
func TestHandoverFromEPSAppliesThePolicyQoSTheUPFMissed(t *testing.T) {
	s, upf := staleEPSUserPlane(t)
	ctx := context.Background()

	ref, _, _, err := s.PrepareSmContextFromEPS(ctx, testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai)
	if err != nil {
		t.Fatalf("PrepareSmContextFromEPS: %v", err)
	}

	ack, err := buildHandoverRequestAcknowledgeTransfer(0x8001, net.ParseIP("10.3.0.11"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	if err := s.UpdateSmContextN2HandoverComplete(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverComplete: %v", err)
	}

	requireDefaultFlowFromPolicy(t, upf, "after the handover to 5GS")
}

// TS 23.502 §4.11.1.3.3
func TestIdleTransferFromEPSAppliesThePolicyQoSTheUPFMissed(t *testing.T) {
	s, upf := staleEPSUserPlane(t)

	if _, err := s.TransferIdle(context.Background(), testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, smf.Access5G); err != nil {
		t.Fatalf("TransferIdle: %v", err)
	}

	requireDefaultFlowFromPolicy(t, upf, "after the idle move to 5GS")
}
