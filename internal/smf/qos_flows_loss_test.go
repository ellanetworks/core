// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
	libngap "github.com/ellanetworks/core/ngap"
)

func reactivate(t *testing.T, s *smf.SMF, ref string, accepted ...uint8) *libngap.PDUSessionResourceSetupRequestTransfer {
	t.Helper()

	ctx := context.Background()

	raw, err := s.ActivateSmContext(ctx, ref)
	if err != nil {
		t.Fatalf("ActivateSmContext: %v", err)
	}

	req, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(raw)
	if err != nil {
		t.Fatalf("parse the Setup Request Transfer: %v", err)
	}

	associated := libngap.AssociatedQosFlowList{{QosFlowIdentifier: 1}}
	for _, qfi := range accepted {
		associated = append(associated, libngap.AssociatedQosFlowItem{QosFlowIdentifier: libngap.QosFlowIdentifier(qfi)})
	}

	rsp, err := (&libngap.PDUSessionResourceSetupResponseTransfer{
		DLQosFlowPerTNLInformation: libngap.QosFlowPerTNLInformation{
			UPTransportLayerInformation: testTunnel(0x7002, net.ParseIP("10.3.0.9")),
			AssociatedQosFlowList:       associated,
		},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, rsp); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	return req
}

func setupRequestCarriesVoice(req *libngap.PDUSessionResourceSetupRequestTransfer) bool {
	return slices.ContainsFunc(req.QosFlowSetupRequest, func(i libngap.QosFlowSetupRequestItem) bool {
		return i.QosFlowIdentifier == voiceQFI && i.QosFlowLevelQosParameters.GBRQosInformation != nil
	})
}

func voiceDownlinkInstalled(t *testing.T, upf *fakeUPF) bool {
	t.Helper()

	return slices.ContainsFunc(lastModify(t, upf).UpdatePDRs, func(p models.PDR) bool { return p.PDI.UEIPAddress.IsValid() && p.QERID >= 256 })
}

func TestUserInactivityKeepsTheVoiceFlowAcrossIdle(t *testing.T) {
	s, pcf, upf, amfCb, ref := activeFiveGVoice(t)
	ctx := context.Background()

	if err := s.DeactivateSmContext(ctx, ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	if !voiceDownlinkInstalled(t, upf) {
		t.Fatal("the idle UE's voice downlink PDR is gone, so buffered voice cannot name its flow (TS 23.502 §4.2.3.3)")
	}

	req := reactivate(t, s, ref, voiceQFI)
	if !setupRequestCarriesVoice(req) {
		t.Fatalf("Setup Request Transfer flows %+v, want the voice QFI with its GBR (TS 23.501 §5.7.1.3)", req.QosFlowSetupRequest)
	}

	requireOneFlowModification(t, amfCb)

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want the call kept (TS 23.502 §4.2.6 step 6a)", r)
	}
}

func TestRadioLossReleasesTheVoiceFlowAndSyncsTheUELater(t *testing.T) {
	s, pcf, upf, amfCb, ref := activeFiveGVoice(t)
	ctx := context.Background()

	if err := s.DeactivateSmContext(ctx, ref, false); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#1"}) {
		t.Fatalf("reported rules %v, want the call released (TS 23.502 §4.2.6 step 6a)", r)
	}

	if reports := pcf.reportedFailures(); reports[0].cause != smf.BearerReleased {
		t.Fatalf("report %+v, want BearerReleased", reports[0])
	}

	if slices.ContainsFunc(lastModify(t, upf).UpdatePDRs, func(p models.PDR) bool { return p.PDI.QFI == voiceQFI }) {
		t.Fatal("the lost voice flow kept its UPF rules")
	}

	pushRules(t, s, ref)
	requireOneFlowModification(t, amfCb)

	if req := reactivate(t, s, ref); setupRequestCarriesVoice(req) {
		t.Fatal("the lost voice flow was set up again on activation")
	}

	c := waitFlowModifications(t, amfCb, 2)[1]
	if c.n2Msg != nil {
		t.Fatal("the deletion asked the RAN to release a flow it no longer holds")
	}

	if cmd := flowCommand(t, c); len(cmd.QoSRules) != 1 || cmd.QoSRules[0].OperationCode != fgs.QoSRuleOpDelete ||
		len(cmd.QoSFlowDescriptions) != 1 || cmd.QoSFlowDescriptions[0].OperationCode != fgs.QoSFlowOpDelete {
		t.Fatalf("command %+v, want the UE's rule and flow deleted at the next activation (TS 23.502 §4.3.3.2 step 1d)", cmd)
	}
}

func TestRANNotAdmittingTheFlowOnActivationLosesTheCall(t *testing.T) {
	s, pcf, _, amfCb, ref := activeFiveGVoice(t)

	if err := s.DeactivateSmContext(context.Background(), ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	reactivate(t, s, ref)

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#1"}) {
		t.Fatalf("reported rules %v, want the call lost (TS 23.502 §4.2.3.2 step 18)", r)
	}

	pushRules(t, s, ref)

	if cmd := flowCommand(t, waitFlowModifications(t, amfCb, 2)[1]); len(cmd.QoSFlowDescriptions) != 1 || cmd.QoSFlowDescriptions[0].OperationCode != fgs.QoSFlowOpDelete {
		t.Fatalf("command %+v, want the flow deleted in the UE", cmd)
	}
}

func ranReleasesVoice(t *testing.T, s *smf.SMF, ref string) {
	t.Helper()

	raw, err := (&libngap.PDUSessionResourceNotifyTransfer{
		QosFlowReleased: libngap.QosFlowListWithCause{{
			QosFlowIdentifier: voiceQFI,
			Cause:             libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: libngap.CauseRadioNetworkRadioResourcesNotAvailable},
		}},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateSmContextN2InfoNotify(context.Background(), ref, raw); err != nil {
		t.Fatalf("UpdateSmContextN2InfoNotify: %v", err)
	}
}

func TestRANReleasingTheFlowEndsTheCall(t *testing.T) {
	tests := []struct {
		name    string
		release func(t *testing.T) (*smf.SMF, *fakePCF, *fakeAMF, string)
	}{
		{"after the call is up", func(t *testing.T) (*smf.SMF, *fakePCF, *fakeAMF, string) {
			s, pcf, _, amfCb, ref := activeFiveGVoice(t)
			ranReleasesVoice(t, s, ref)

			return s, pcf, amfCb, ref
		}},
		{"before the UE completes", func(t *testing.T) (*smf.SMF, *fakePCF, *fakeAMF, string) {
			s, pcf, _, amfCb, ref := fiveGVoiceFixture(t)

			pushRules(t, s, ref, voiceRule())
			waitFlowModifications(t, amfCb, 1)
			ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)
			ranReleasesVoice(t, s, ref)
			ueAnswers(t, s, ref, true)

			return s, pcf, amfCb, ref
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, pcf, amfCb, ref := tc.release(t)

			if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#1"}) {
				t.Fatalf("reported rules %v, want the call released (TS 38.413 §8.2.4.2)", r)
			}

			pushRules(t, s, ref)

			c := waitFlowModifications(t, amfCb, 2)[1]
			if c.n2Msg != nil || len(flowCommand(t, c).QoSFlowDescriptions) != 1 {
				t.Fatalf("modification %+v, want an N1-only deletion of the released flow", c)
			}
		})
	}
}

func TestPathSwitchWithoutTheVoiceFlowKeepsTheSession(t *testing.T) {
	s, pcf, _, _, ref := activeFiveGVoice(t)

	n2, err := buildPathSwitchRequestTransfer(0x8001, net.ParseIP("10.3.0.10"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.UpdateSmContextXnHandoverPathSwitchReq(context.Background(), ref, n2); err != nil {
		t.Fatalf("path switch: %v", err)
	}

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#1"}) {
		t.Fatalf("reported rules %v, want the call lost with the session kept (TS 23.502 §4.9.1.2.2)", r)
	}
}

func TestUEDeletingTheVoiceRuleReleasesTheFlow(t *testing.T) {
	s, pcf, _, amfCb, ref := activeFiveGVoice(t)
	ctx := context.Background()
	rule := flowCommand(t, amfCb.modifications()[0]).QoSRules[0]

	cause := fgs.GSMCauseSemanticErrorsInPacketFilters

	raw, err := (&fgs.PDUSessionModificationRequest{
		PDUSessionID:      3,
		PTI:               7,
		Cause:             &cause,
		RequestedQoSRules: fgs.QoSRules{{Identifier: rule.Identifier, OperationCode: fgs.QoSRuleOpDelete}},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	res, err := s.UpdateSmContextN1Msg(ctx, ref, raw)
	if err != nil || res == nil || res.N1Msg == nil {
		t.Fatalf("UpdateSmContextN1Msg = %+v, %v; want a command", res, err)
	}

	cmd, err := fgs.ParsePDUSessionModificationCommand(res.N1Msg)
	if err != nil || cmd.PTI != 7 || len(cmd.QoSRules) != 1 || cmd.QoSRules[0].OperationCode != fgs.QoSRuleOpDelete {
		t.Fatalf("answer %+v (%v), want the deletion accepted with the UE's PTI (TS 24.501 §6.4.2.3)", cmd, err)
	}

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#1"}) {
		t.Fatalf("reported rules %v, want the call lost", r)
	}

	pushRules(t, s, ref)

	complete, err := (&fgs.PDUSessionModificationComplete{PDUSessionID: 3, PTI: 7}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, complete); err != nil {
		t.Fatalf("UpdateSmContextN1Msg(Complete): %v", err)
	}

	if c := waitFlowModifications(t, amfCb, 2)[1]; c.n2Msg == nil {
		t.Fatal("the deleted flow was never released in the RAN (TS 23.502 §4.3.3.2 step 3b)")
	}
}

func TestEveryEPSFallbackKeepsTheCallsRules(t *testing.T) {
	s, pcf, _, amfCb, ref := fiveGVoiceFixture(t)
	s.SetDedicatedAwaitLimitForTest(10 * 1e6)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)
	pushRules(t, s, ref)
	ranFallsBack(t, s, ref)

	pushRules(t, s, ref, secondCallRule())
	waitFlowModifications(t, amfCb, 2)
	s.SetDedicatedAwaitLimitForTest(time.Hour)
	ranFallsBack(t, s, ref)

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none: the SMF maintains the PCC rules on every EPS fallback (TS 23.502 §4.13.6.1 step 4)", r)
	}
}

func TestEPSFallbackThatNeverMovesTheUEFailsTheCall(t *testing.T) {
	s, pcf, _, amfCb, ref := fiveGVoiceFixture(t)
	s.SetDedicatedAwaitLimitForTest(10 * time.Millisecond)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)
	ranFallsBack(t, s, ref)

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#1"}) {
		t.Fatalf("reported rules %v, want the call failed once the UE stayed on 5GS", r)
	}

	if reports := pcf.reportedFailures(); reports[0].cause != smf.ResourcesNotAllocated {
		t.Fatalf("report %+v, want ResourcesNotAllocated", reports[0])
	}

	time.Sleep(50 * time.Millisecond)

	if n := len(amfCb.modifications()); n != 1 {
		t.Fatalf("%d QoS flow modifications, want the gNB not asked to fall back again", n)
	}
}

func TestBufferedVoicePagesWithTheVoiceFlowARP(t *testing.T) {
	s, _, upf, amfCb, ref := activeFiveGVoice(t)
	ctx := context.Background()

	if err := s.DeactivateSmContext(ctx, ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	i := slices.IndexFunc(lastModify(t, upf).UpdatePDRs, func(p models.PDR) bool { return p.PDI.UEIPAddress.IsValid() && p.QERID >= 256 })
	if i < 0 {
		t.Fatal("no voice downlink PDR while idle")
	}

	pdr := lastModify(t, upf).UpdatePDRs[i]
	seid := s.GetSession(ref).PFCPContext.SEID

	if err := s.HandleDownlinkDataReport(ctx, &models.DownlinkDataReport{SEID: seid, PDRID: pdr.PDRID}); err != nil {
		t.Fatalf("HandleDownlinkDataReport: %v", err)
	}

	amfCb.mu.Lock()
	arps := slices.Clone(amfCb.pageARPs)
	amfCb.mu.Unlock()

	if len(arps) != 1 || arps[0] == nil || *arps[0] != voiceARP {
		t.Fatalf("paging ARPs %+v, want the voice flow's %+v (TS 23.502 §4.2.3.3 step 3a)", arps, voiceARP)
	}
}

func TestSecondReleaseWithAnotherCauseKeepsPreservedFlows(t *testing.T) {
	s, pcf, upf, _, ref := activeFiveGVoice(t)
	ctx := context.Background()

	if err := s.DeactivateSmContext(ctx, ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	if err := s.DeactivateSmContext(ctx, ref, false); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	if !voiceDownlinkInstalled(t, upf) {
		t.Fatal("the second pass of the same AN release dropped the preserved voice flow")
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want the call kept", r)
	}
}

func TestRulesRemovedWhileIdleAreNotSetUpAgain(t *testing.T) {
	s, _, upf, _, ref := activeFiveGVoice(t)

	if err := s.DeactivateSmContext(context.Background(), ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	pushRules(t, s, ref)

	if voiceDownlinkInstalled(t, upf) {
		t.Fatal("the ended call's voice downlink still pages the idle UE")
	}

	if req := reactivate(t, s, ref); setupRequestCarriesVoice(req) {
		t.Fatal("the ended call's voice flow was set up again on activation")
	}
}

func TestVoiceRuleForAnIdleUEPagesThenSetsUpTheFlow(t *testing.T) {
	s, _, _, amfCb, ref := fiveGVoiceFixture(t)
	ctx := context.Background()

	if err := s.DeactivateSmContext(ctx, ref, false); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	pushRules(t, s, ref, voiceRule())

	waitFor(t, "the UE paged for the voice flow", func() bool {
		amfCb.mu.Lock()
		defer amfCb.mu.Unlock()

		return len(amfCb.pageARPs) == 1 && amfCb.pageARPs[0] != nil && *amfCb.pageARPs[0] == voiceARP
	})

	if got := amfCb.modifications(); len(got) != 0 {
		t.Fatalf("%d QoS flow modifications sent to an idle UE, want none until it returns (TS 23.502 §4.2.3.3)", len(got))
	}

	if _, err := s.ActivateSmContext(ctx, ref); err != nil {
		t.Fatalf("ActivateSmContext: %v", err)
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7002, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	cmd := flowCommand(t, waitFlowModifications(t, amfCb, 1)[0])
	if len(cmd.QoSRules) != 1 || cmd.QoSRules[0].OperationCode != fgs.QoSRuleOpCreate || cmd.QoSRules[0].Parameters.QFI != voiceQFI {
		t.Fatalf("QoS rules %+v, want the voice rule created once the UE is back", cmd.QoSRules)
	}
}

func TestAnswersArrivingAfterTheReleaseOfASessionAreIgnored(t *testing.T) {
	s, _, upf, amfCb, ref := activeFiveGVoice(t)
	ctx := context.Background()

	pushRules(t, s, ref, voiceRule(), secondCallRule())
	waitFlowModifications(t, amfCb, 2)

	if err := s.ReleaseSmContext(ctx, ref, models.ReleaseFor(models.ReleaseUERequested)); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	upf.mu.Lock()
	modifies := len(upf.modifyCalls)
	upf.mu.Unlock()

	raw, err := (&fgs.PDUSessionModificationComplete{PDUSessionID: 3}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	_, _ = s.UpdateSmContextN1Msg(ctx, ref, raw)

	tr := libngap.PDUSessionResourceModifyResponseTransfer{QosFlowAddOrModifyResponse: []libngap.QosFlowAddOrModifyResponseItem{{QosFlowIdentifier: voiceQFI}}}

	n2, err := tr.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	_ = s.UpdateSmContextN2InfoPduResModifyRsp(ctx, ref, n2)

	time.Sleep(100 * time.Millisecond)

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != modifies {
		t.Fatal("a late answer reprogrammed the user plane of a released session")
	}

	if s.GetSession(ref) != nil {
		t.Fatal("a late answer revived the released session")
	}
}

func TestRulePushRacingTheReleaseOfASession(t *testing.T) {
	s, _, _, _, ref := activeFiveGVoice(t) //nolint:dogsled // only the session is under test
	ctx := context.Background()
	policy := committedPolicy(s, ref)

	decisions := make([]*smf.PolicyDecision, 0, 40)

	for i := range 40 {
		nextRevision++

		rules := []smf.PCCRule{voiceRule()}
		if i%2 == 0 {
			rules = append(rules, secondCallRule())
		}

		decisions = append(decisions, &smf.PolicyDecision{
			Revision: nextRevision, PolicyID: policy.PolicyID, Var5qi: policy.QosData.Var5qi,
			Arp: policy.QosData.Arp.PriorityLevel, SessionAMBR: policy.Ambr, Rules: rules,
		})
	}

	var wg sync.WaitGroup

	wg.Go(func() {
		for _, d := range decisions {
			_ = s.UpdateNotify(ctx, ref, d)
		}
	})

	wg.Go(func() {
		if err := s.ReleaseSmContext(ctx, ref, models.ReleaseFor(models.ReleaseUERequested)); err != nil {
			t.Errorf("ReleaseSmContext: %v", err)
		}
	})

	wg.Wait()
}

func TestUnansweredFlowCommandIsRetransmittedThenFails(t *testing.T) {
	pcf, store, upf, amfCb, _ := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb, smf.WithT3591(procedureTimerInterval))
	sc := establish5GS(t, s)

	pushRules(t, s, sc.Ref, voiceRule())

	if r := waitReportedRules(t, pcf); len(r) != 1 {
		t.Fatalf("reported rules %v, want the voice rule failed once T3591 expired a fifth time (TS 24.501 §6.3.2.5 a)", r)
	}

	if got := len(amfCb.modifications()); got != 5 {
		t.Fatalf("%d transmissions of the QoS flow command, want the first plus four retransmissions", got)
	}
}

func refusedFlowRemoval(t *testing.T) (*smf.SMF, *fakePCF, *fakeUPF, *fakeAMF, string) {
	t.Helper()

	s, pcf, upf, amfCb, ref := activeFiveGVoice(t)

	pushRules(t, s, ref)
	waitFlowModifications(t, amfCb, 2)

	ueAnswers(t, s, ref, false)
	ranAnswers(t, s, ref, nil, nil)

	return s, pcf, upf, amfCb, ref
}

func TestRefusedFlowRemovalStopsEnforcingTheRemovedRules(t *testing.T) {
	s, pcf, upf, amfCb, ref := refusedFlowRemoval(t)

	if slices.ContainsFunc(lastModify(t, upf).UpdatePDRs, func(p models.PDR) bool { return p.PDI.QFI == voiceQFI || p.QERID >= 256 }) {
		t.Fatal("the PCF removed the voice rule but the UPF still enforces it (TS 23.502 §4.3.3.2 step 8)")
	}

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatalf("ReconcileSession: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if got := len(amfCb.modifications()); got != 2 {
		t.Fatalf("%d QoS flow modifications sent, want 2: the deletion waits to be synchronised with the UE", got)
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none: a removal does not fail", r)
	}
}

func TestRefusedFlowRemovalIsResentWhenTheUserPlaneReturns(t *testing.T) {
	s, _, _, amfCb, ref := refusedFlowRemoval(t)

	if err := s.DeactivateSmContext(context.Background(), ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	if req := reactivate(t, s, ref); setupRequestCarriesVoice(req) {
		t.Fatal("the removed voice flow was set up again on activation")
	}

	c := waitFlowModifications(t, amfCb, 3)[2]
	if c.n2Msg != nil {
		t.Fatal("the resync asked the RAN to release a flow it no longer holds")
	}

	cmd := flowCommand(t, c)
	if len(cmd.QoSRules) != 1 || cmd.QoSRules[0].OperationCode != fgs.QoSRuleOpDelete ||
		len(cmd.QoSFlowDescriptions) != 1 || cmd.QoSFlowDescriptions[0].QFI != voiceQFI || cmd.QoSFlowDescriptions[0].OperationCode != fgs.QoSFlowOpDelete {
		t.Fatalf("command %+v, want the unsynchronised rule and flow deleted (TS 23.502 §4.3.3.2 step 1d)", cmd)
	}

	ueAnswers(t, s, ref, true)

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatalf("ReconcileSession: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if got := len(amfCb.modifications()); got != 3 {
		t.Fatalf("%d QoS flow modifications sent, want 3: the flow is gone once the UE confirmed", got)
	}
}

func TestRefusedFlowRemovalRidesOnTheNextCommand(t *testing.T) {
	s, _, _, amfCb, ref := refusedFlowRemoval(t)

	pushRules(t, s, ref, voiceRule())

	c := waitFlowModifications(t, amfCb, 3)[2]
	cmd := flowCommand(t, c)

	var created, deleted []uint8

	for _, d := range cmd.QoSFlowDescriptions {
		switch d.OperationCode {
		case fgs.QoSFlowOpCreate:
			created = append(created, d.QFI)
		case fgs.QoSFlowOpDelete:
			deleted = append(deleted, d.QFI)
		}
	}

	if !slices.Equal(deleted, []uint8{voiceQFI}) || len(created) != 1 || created[0] == voiceQFI {
		t.Fatalf("flow descriptions %+v, want the call on a new QFI and the unsynchronised flow deleted in the same command (TS 24.501 §6.3.2.2)", cmd.QoSFlowDescriptions)
	}

	rules := map[fgs.QoSRuleOperation]int{}
	for _, r := range cmd.QoSRules {
		rules[r.OperationCode]++
	}

	if rules[fgs.QoSRuleOpCreate] != 1 || rules[fgs.QoSRuleOpDelete] != 1 {
		t.Fatalf("QoS rules %+v, want one created and the stale one deleted", cmd.QoSRules)
	}
}

func TestPathSwitchClearsAPendingDeletionTheTargetNeverHeld(t *testing.T) {
	s, _, _, amfCb, ref := activeFiveGVoice(t)
	ctx := context.Background()

	pushRules(t, s, ref)
	waitFlowModifications(t, amfCb, 2)

	raw, err := (&libngap.PDUSessionResourceModifyUnsuccessfulTransfer{
		Cause: libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: libngap.CauseRadioNetworkRadioResourcesNotAvailable},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateSmContextN2InfoPduResModifyFail(ctx, ref, raw); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResModifyFail: %v", err)
	}

	n2, err := buildPathSwitchRequestTransfer(0x8001, net.ParseIP("10.3.0.10"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.UpdateSmContextXnHandoverPathSwitchReq(ctx, ref, n2); err != nil {
		t.Fatalf("path switch: %v", err)
	}

	c := waitFlowModifications(t, amfCb, 3)[2]
	if c.n2Msg != nil {
		t.Fatal("the deletion asked the target gNB to release a flow it never set up (TS 38.413 §8.4.4)")
	}

	if cmd := flowCommand(t, c); len(cmd.QoSFlowDescriptions) != 1 || cmd.QoSFlowDescriptions[0].OperationCode != fgs.QoSFlowOpDelete {
		t.Fatalf("command %+v, want the flow deleted in the UE", cmd)
	}
}

func TestRuleOnAReleasedFlowIsNotReportedActive(t *testing.T) {
	s, pcf, _, amfCb, ref := activeFiveGVoice(t)

	ranReleasesVoice(t, s, ref)
	waitReportedRules(t, pcf)
	waitFlowModifications(t, amfCb, 2)
	ueAnswers(t, s, ref, false)

	updated := voiceRule()
	updated.Version = 2
	updated.Filters[0].RemotePort, updated.Filters[1].RemotePort = 49100, 49100
	pushRules(t, s, ref, updated)

	waitFlowModifications(t, amfCb, 3)
	ueAnswers(t, s, ref, false)
	ranAnswers(t, s, ref, nil, []uint8{voiceQFI + 1})

	reports := pcf.reportedFailures()
	if last := reports[len(reports)-1]; last.Rule.Version != 2 || last.Active != nil {
		t.Fatalf("report %+v, want version 2 failed with no rule left active: the released flow enforces nothing", last)
	}
}

func TestFlowTheRANDidNotSetUpIsNotEnforced(t *testing.T) {
	s, pcf, upf, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, nil, []uint8{voiceQFI})

	if slices.ContainsFunc(lastModify(t, upf).UpdatePDRs, func(p models.PDR) bool { return p.PDI.QFI == voiceQFI || p.QERID >= 256 }) {
		t.Fatal("the UPF enforces a QoS flow the NG-RAN node did not set up")
	}

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#1"}) {
		t.Fatalf("reported rules %v, want the call failed", r)
	}
}

func TestModificationTheUEAppliedSurvivesAnANRelease(t *testing.T) {
	s, _, _, amfCb, ref := activeFiveGVoice(t)

	faster := voiceRule()
	faster.Version = 1
	rate := models.BitRateFromBps(64000)
	faster.MBR, faster.GBR = models.Ambr{Uplink: rate, Downlink: rate}, models.Ambr{Uplink: rate, Downlink: rate}

	pushRules(t, s, ref, faster)
	waitFlowModifications(t, amfCb, 2)
	ueAnswers(t, s, ref, true)

	if err := s.DeactivateSmContext(context.Background(), ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	req := reactivate(t, s, ref, voiceQFI)

	i := slices.IndexFunc(req.QosFlowSetupRequest, func(f libngap.QosFlowSetupRequestItem) bool { return f.QosFlowIdentifier == voiceQFI })
	if i < 0 || req.QosFlowSetupRequest[i].QosFlowLevelQosParameters.GBRQosInformation.GuaranteedFlowBitRateDL != 64000 {
		t.Fatalf("Setup Request Transfer flows %+v, want the 64 kbps the UE applied before the release", req.QosFlowSetupRequest)
	}
}

func TestRefusedRuleRemovalIsResentWhenTheUserPlaneReturns(t *testing.T) {
	s, _, _, amfCb, ref := activeFiveGVoice(t)

	pushRules(t, s, ref, voiceRule(), secondCallRule())
	waitFlowModifications(t, amfCb, 2)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 3)
	ueAnswers(t, s, ref, false)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	realign := waitFlowModifications(t, amfCb, 4)[3]
	if realign.n1Msg != nil || modifyTransfer(t, realign) == nil {
		t.Fatal("want the NG-RAN node realigned with the rates the UE kept")
	}

	if err := s.DeactivateSmContext(context.Background(), ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	reactivate(t, s, ref, voiceQFI)

	cmd := flowCommand(t, waitFlowModifications(t, amfCb, 5)[4])
	if !slices.ContainsFunc(cmd.QoSRules, func(r fgs.QoSRule) bool { return r.OperationCode == fgs.QoSRuleOpDelete }) {
		t.Fatalf("command %+v, want the removed rule deleted once the user plane is back (TS 29.212 §4.5.12)", cmd)
	}
}

func TestModificationTheUEHasNotAnsweredIsAbortedByAnANRelease(t *testing.T) {
	s, pcf, _, amfCb, ref := activeFiveGVoice(t)

	slower := voiceRule()
	slower.Version = 1
	rate := models.BitRateFromBps(30000)
	slower.MBR, slower.GBR = models.Ambr{Uplink: rate, Downlink: rate}, models.Ambr{Uplink: rate, Downlink: rate}

	pushRules(t, s, ref, slower)
	waitFlowModifications(t, amfCb, 2)

	if err := s.DeactivateSmContext(context.Background(), ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	reports := pcf.reportedFailures()
	if len(reports) != 1 || reports[0].Rule.Version != 1 || reports[0].Active == nil || reports[0].Active.Version != 0 || reports[0].cause != smf.ResourcesNotAllocated {
		t.Fatalf("reports %+v, want version 1 not enforced with version 0 still active (TS 23.502 §4.3.3.2 step 13)", reports)
	}

	req := reactivate(t, s, ref, voiceQFI)

	i := slices.IndexFunc(req.QosFlowSetupRequest, func(f libngap.QosFlowSetupRequestItem) bool { return f.QosFlowIdentifier == voiceQFI })
	if i < 0 || req.QosFlowSetupRequest[i].QosFlowLevelQosParameters.GBRQosInformation.GuaranteedFlowBitRateDL != 41000 {
		t.Fatalf("Setup Request Transfer flows %+v, want the previous 41 kbps (TS 24.501 §6.3.2.5)", req.QosFlowSetupRequest)
	}

	pushRules(t, s, ref, slower)
	time.Sleep(100 * time.Millisecond)

	if n := len(amfCb.modifications()); n != 2 {
		t.Fatalf("%d QoS flow modifications, want the aborted one not sent again once the UE is back", n)
	}
}

func TestFlowCreationTheUEHasNotAnsweredIsAbortedByAnANRelease(t *testing.T) {
	s, pcf, _, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)

	if err := s.DeactivateSmContext(context.Background(), ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	reports := pcf.reportedFailures()
	if len(reports) != 1 || reports[0].Rule.ID != "af;1#1" || reports[0].Active != nil || reports[0].cause != smf.ResourcesNotAllocated {
		t.Fatalf("reports %+v, want the call's rule not installed (TS 23.502 §4.3.3.2 step 13)", reports)
	}

	if req := reactivate(t, s, ref); setupRequestCarriesVoice(req) {
		t.Fatal("the aborted voice flow was set up on activation")
	}

	time.Sleep(100 * time.Millisecond)

	if n := len(amfCb.modifications()); n != 1 {
		t.Fatalf("%d QoS flow modifications, want the aborted creation not sent again once the UE is back", n)
	}
}

func TestRuleRemovalAbortedByAnANReleaseIsResentWhenTheUserPlaneReturns(t *testing.T) {
	s, pcf, _, amfCb, ref := activeFiveGVoice(t)

	pushRules(t, s, ref, voiceRule(), secondCallRule())
	waitFlowModifications(t, amfCb, 2)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 3)

	if err := s.DeactivateSmContext(context.Background(), ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none: a removal does not fail (TS 29.212 §4.5.12)", r)
	}

	time.Sleep(100 * time.Millisecond)

	amfCb.mu.Lock()
	pages := len(amfCb.pageARPs)
	amfCb.mu.Unlock()

	if n := len(amfCb.modifications()); n != 3 || pages != 0 {
		t.Fatalf("%d QoS flow modifications and %d pages after the release, want the UE neither paged nor signalled until it returns", n, pages)
	}

	reactivate(t, s, ref, voiceQFI)

	cmd := flowCommand(t, waitFlowModifications(t, amfCb, 4)[3])
	if !slices.ContainsFunc(cmd.QoSRules, func(r fgs.QoSRule) bool { return r.OperationCode == fgs.QoSRuleOpDelete }) {
		t.Fatalf("command %+v, want the removed rule deleted once the user plane is back (TS 29.212 §4.5.12)", cmd)
	}
}
