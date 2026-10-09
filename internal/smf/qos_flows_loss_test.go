// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net"
	"slices"
	"testing"

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

func setupRequestCarries(req *libngap.PDUSessionResourceSetupRequestTransfer, qfi uint8) bool {
	return slices.ContainsFunc(req.QosFlowSetupRequest, func(i libngap.QosFlowSetupRequestItem) bool {
		return i.QosFlowIdentifier == libngap.QosFlowIdentifier(qfi) && i.QosFlowLevelQosParameters.GBRQosInformation != nil
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
	if !setupRequestCarries(req, voiceQFI) {
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

	if req := reactivate(t, s, ref); setupRequestCarries(req, voiceQFI) {
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

func TestRANReleasingTheFlowEndsTheCall(t *testing.T) {
	s, pcf, _, amfCb, ref := activeFiveGVoice(t)

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

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#1"}) {
		t.Fatalf("reported rules %v, want the call released (TS 38.413 §8.2.4.2)", r)
	}

	pushRules(t, s, ref)

	c := waitFlowModifications(t, amfCb, 2)[1]
	if c.n2Msg != nil || len(flowCommand(t, c).QoSFlowDescriptions) != 1 {
		t.Fatalf("modification %+v, want an N1-only deletion of the released flow", c)
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

func TestUEDeletingTheVoiceRuleIsAccepted(t *testing.T) {
	s, pcf, _, amfCb, ref := activeFiveGVoice(t)
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

	res, err := s.UpdateSmContextN1Msg(context.Background(), ref, raw)
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
}

func TestRepeatedEPSFallbackStopsRetrying(t *testing.T) {
	s, pcf, _, amfCb, ref := fiveGVoiceFixture(t)
	s.SetDedicatedAwaitLimitForTest(10 * 1e6)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)
	ranFallsBack(t, s, ref)

	waitFlowModifications(t, amfCb, 2)
	ranFallsBack(t, s, ref)

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#1"}) {
		t.Fatalf("reported rules %v, want the call failed once EPS fallback did not happen", r)
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

func TestUEDeletionReleasesTheFlowInTheRAN(t *testing.T) {
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

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, raw); err != nil {
		t.Fatalf("UpdateSmContextN1Msg: %v", err)
	}

	waitReportedRules(t, pcf)
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

	if req := reactivate(t, s, ref); setupRequestCarries(req, voiceQFI) {
		t.Fatal("the ended call's voice flow was set up again on activation")
	}
}
