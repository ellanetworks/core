// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
	libngap "github.com/ellanetworks/core/ngap"
)

const voiceQFI = 2

func fiveGVoiceFixture(t *testing.T) (*smf.SMF, *fakePCF, *fakeUPF, *fakeAMF, string) {
	t.Helper()

	pcf, store, upf, amfCb, _ := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	sc := establish5GS(t, s)

	return s, pcf, upf, amfCb, sc.Ref
}

func (f *fakeAMF) modifications() []n1n2Call {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.modifyCalls)
}

func waitFlowModifications(t *testing.T, f *fakeAMF, n int) []n1n2Call {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for {
		if calls := f.modifications(); len(calls) >= n {
			return calls
		}

		if time.Now().After(deadline) {
			t.Fatalf("%d QoS flow modifications sent, want %d", len(f.modifications()), n)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func requireOneFlowModification(t *testing.T, f *fakeAMF) {
	t.Helper()

	time.Sleep(100 * time.Millisecond)

	if got := len(f.modifications()); got != 1 {
		t.Fatalf("%d QoS flow modifications sent, want 1", got)
	}
}

func flowCommand(t *testing.T, c n1n2Call) *fgs.PDUSessionModificationCommand {
	t.Helper()

	if c.n1Msg == nil {
		t.Fatal("the modification carries no N1 message")
	}

	cmd, err := fgs.ParsePDUSessionModificationCommand(c.n1Msg)
	if err != nil {
		t.Fatalf("parse PDU Session Modification Command: %v", err)
	}

	return cmd
}

func modifyTransfer(t *testing.T, c n1n2Call) *libngap.PDUSessionResourceModifyRequestTransfer {
	t.Helper()

	if c.n2Msg == nil {
		t.Fatal("the modification carries no N2 transfer")
	}

	tr, err := libngap.ParsePDUSessionResourceModifyRequestTransfer(c.n2Msg)
	if err != nil {
		t.Fatalf("parse PDU Session Resource Modify Request Transfer: %v", err)
	}

	return tr
}

func ueAnswers(t *testing.T, s *smf.SMF, ref string, accepted bool) {
	t.Helper()

	var (
		raw []byte
		err error
	)

	if accepted {
		raw, err = (&fgs.PDUSessionModificationComplete{PDUSessionID: 3}).MarshalBinary()
	} else {
		raw, err = (&fgs.PDUSessionModificationCommandReject{PDUSessionID: 3, Cause: fgs.GSMCauseSemanticErrorsInPacketFilters}).MarshalBinary()
	}

	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, raw); err != nil {
		t.Fatalf("UpdateSmContextN1Msg: %v", err)
	}
}

func ranAnswers(t *testing.T, s *smf.SMF, ref string, accepted, failed []uint8) {
	t.Helper()

	var tr libngap.PDUSessionResourceModifyResponseTransfer

	for _, qfi := range accepted {
		tr.QosFlowAddOrModifyResponse = append(tr.QosFlowAddOrModifyResponse, libngap.QosFlowAddOrModifyResponseItem{QosFlowIdentifier: libngap.QosFlowIdentifier(qfi)})
	}

	for _, qfi := range failed {
		tr.QosFlowFailedToAddOrModify = append(tr.QosFlowFailedToAddOrModify, libngap.QosFlowWithCauseItem{
			QosFlowIdentifier: libngap.QosFlowIdentifier(qfi),
			Cause:             libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: libngap.CauseRadioNetworkRadioResourcesNotAvailable},
		})
	}

	raw, err := tr.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateSmContextN2InfoPduResModifyRsp(context.Background(), ref, raw); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResModifyRsp: %v", err)
	}
}

func ranFallsBack(t *testing.T, s *smf.SMF, ref string) {
	t.Helper()

	raw, err := (&libngap.PDUSessionResourceModifyUnsuccessfulTransfer{
		Cause: libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: libngap.CauseRadioNetworkIMSVoiceEPSFallbackTriggered},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateSmContextN2InfoPduResModifyFail(context.Background(), ref, raw); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResModifyFail: %v", err)
	}
}

func activeFiveGVoice(t *testing.T) (*smf.SMF, *fakePCF, *fakeUPF, *fakeAMF, string) {
	t.Helper()

	s, pcf, upf, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)

	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	return s, pcf, upf, amfCb, ref
}

func flowDescriptionRate(t *testing.T, d fgs.QoSFlowDescription, id fgs.QoSFlowParameterID) uint64 {
	t.Helper()

	for _, p := range d.Parameters {
		if p.ID == id {
			kbps, _ := p.Kbps()
			return kbps
		}
	}

	t.Fatalf("flow description %+v carries no %s", d, id)

	return 0
}

func TestVoiceRuleCreatesAGBRQoSFlow(t *testing.T) {
	s, pcf, upf, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, voiceRule())

	c := waitFlowModifications(t, amfCb, 1)[0]
	cmd := flowCommand(t, c)

	if len(cmd.QoSRules) != 1 {
		t.Fatalf("QoS rules %+v, want one for the call", cmd.QoSRules)
	}

	rule := cmd.QoSRules[0]
	if rule.OperationCode != fgs.QoSRuleOpCreate || rule.Identifier == 1 || rule.DQR != 0 || rule.Parameters == nil ||
		rule.Parameters.QFI != voiceQFI || rule.Parameters.Precedence >= 70 || rule.Parameters.Precedence == 0 || len(rule.Filters) != 2 {
		t.Fatalf("QoS rule %+v, want a new non-default rule on QFI %d with both filters (TS 24.501 §9.11.4.13)", rule, voiceQFI)
	}

	wantComponents := []fgs.PacketFilterComponent{
		{Type: 0x10, Value: []byte{192, 0, 2, 10, 255, 255, 255, 255}},
		{Type: 0x30, Value: []byte{17}},
		{Type: 0x40, Value: []byte{0xc3, 0x50}},
		{Type: 0x50, Value: []byte{0xbf, 0x68}},
	}

	for _, dir := range []fgs.PacketFilterDirection{fgs.PacketFilterDownlink, fgs.PacketFilterUplink} {
		i := slices.IndexFunc(rule.Filters, func(f fgs.PacketFilter) bool { return f.Direction == dir })
		if i < 0 || !slices.EqualFunc(rule.Filters[i].Components, wantComponents, func(a, b fgs.PacketFilterComponent) bool {
			return a.Type == b.Type && bytes.Equal(a.Value, b.Value)
		}) {
			t.Fatalf("packet filters %+v, want a %v filter on the peer 192.0.2.10/32, UDP, local port 50000, remote port 49000 (TS 24.501 table 9.11.4.13.1)", rule.Filters, dir)
		}
	}

	if rule.Filters[0].Identifier == rule.Filters[1].Identifier {
		t.Fatalf("packet filters %+v share an identifier (TS 24.501 §9.11.4.13)", rule.Filters)
	}

	if len(cmd.QoSFlowDescriptions) != 1 {
		t.Fatalf("QoS flow descriptions %+v, want one", cmd.QoSFlowDescriptions)
	}

	desc := cmd.QoSFlowDescriptions[0]
	if desc.QFI != voiceQFI || desc.OperationCode != fgs.QoSFlowOpCreate || !desc.EBit {
		t.Fatalf("QoS flow description %+v, want a new QFI %d", desc, voiceQFI)
	}

	if gfbr := flowDescriptionRate(t, desc, fgs.QoSFlowParamGFBRDownlink); gfbr != 41 {
		t.Fatalf("GFBR downlink %d kbps, want 41 (TS 24.501 §6.2.5.1.1.4)", gfbr)
	}

	tr := modifyTransfer(t, c)
	if len(tr.QosFlowAddOrModifyRequest) != 1 {
		t.Fatalf("N2 add/modify list %+v, want the voice QFI", tr.QosFlowAddOrModifyRequest)
	}

	item := tr.QosFlowAddOrModifyRequest[0]
	if item.QosFlowIdentifier != voiceQFI || item.QosFlowLevelQosParameters == nil || item.QosFlowLevelQosParameters.GBRQosInformation == nil ||
		item.QosFlowLevelQosParameters.QosCharacteristics.NonDynamic5QI.FiveQI != 1 ||
		item.QosFlowLevelQosParameters.GBRQosInformation.GuaranteedFlowBitRateDL != 41000 {
		t.Fatalf("N2 QoS flow %+v, want 5QI 1 with GBR QoS Flow Information (TS 38.413 §8.2.3.4)", item)
	}

	wantARP := libngap.AllocationAndRetentionPriority{
		PriorityLevelARP:        libngap.PriorityLevelARP(voiceARP.PriorityLevel),
		PreemptionCapability:    libngap.PreemptionMayTrigger,
		PreemptionVulnerability: libngap.PreemptionNotPreemptable,
	}
	if arp := item.QosFlowLevelQosParameters.AllocationAndRetentionPriority; arp != wantARP {
		t.Fatalf("N2 ARP %+v, want the rule's %+v (TS 38.413 §9.3.1.19)", arp, wantARP)
	}

	m := lastModify(t, upf)
	if slices.ContainsFunc(m.UpdatePDRs, func(p models.PDR) bool { return p.PDI.UEIPAddress.IsValid() && p.QERID >= 256 }) {
		t.Fatal("the flow's downlink PDR was installed before the RAN accepted the QFI (TS 23.502 §4.3.3.2 step 8)")
	}

	if !slices.ContainsFunc(m.UpdatePDRs, func(p models.PDR) bool { return p.PDI.QFI == voiceQFI && p.PDI.LocalFTEID != nil }) {
		t.Fatalf("UPF PDRs %+v, want the voice uplink on the session F-TEID with QFI %d", m.UpdatePDRs, voiceQFI)
	}

	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	m = lastModify(t, upf)
	if !slices.ContainsFunc(m.UpdatePDRs, func(p models.PDR) bool { return p.PDI.UEIPAddress.IsValid() && p.QERID >= 256 }) {
		t.Fatalf("UPF PDRs %+v, want the voice downlink once both legs accepted", m.UpdatePDRs)
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none", r)
	}

	requireOneFlowModification(t, amfCb)
}

func TestSecondCallAddsAQoSRuleToTheFlow(t *testing.T) {
	s, _, _, amfCb, ref := activeFiveGVoice(t)
	first := flowCommand(t, amfCb.modifications()[0]).QoSRules[0]

	pushRules(t, s, ref, voiceRule(), secondCallRule())

	c := waitFlowModifications(t, amfCb, 2)[1]
	cmd := flowCommand(t, c)

	if len(cmd.QoSRules) != 1 || cmd.QoSRules[0].OperationCode != fgs.QoSRuleOpCreate || cmd.QoSRules[0].Identifier == first.Identifier ||
		cmd.QoSRules[0].Parameters.Precedence == first.Parameters.Precedence || cmd.QoSRules[0].Parameters.QFI != voiceQFI {
		t.Fatalf("QoS rules %+v, want one new rule with its own QRI and precedence on QFI %d (TS 24.501 §6.3.2.4 a)5)", cmd.QoSRules, voiceQFI)
	}

	if len(cmd.QoSFlowDescriptions) != 1 || cmd.QoSFlowDescriptions[0].OperationCode != fgs.QoSFlowOpModify ||
		flowDescriptionRate(t, cmd.QoSFlowDescriptions[0], fgs.QoSFlowParamGFBRUplink) != 82 {
		t.Fatalf("QoS flow descriptions %+v, want the flow modified to the summed 82 kbps (TS 29.513 §7.4.1)", cmd.QoSFlowDescriptions)
	}

	if tr := modifyTransfer(t, c); len(tr.QosFlowAddOrModifyRequest) != 1 || tr.QosFlowAddOrModifyRequest[0].QosFlowLevelQosParameters.GBRQosInformation.GuaranteedFlowBitRateUL != 82000 {
		t.Fatalf("N2 add/modify list %+v, want the flow's new GFBR", tr.QosFlowAddOrModifyRequest)
	}

	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	pushRules(t, s, ref, secondCallRule())

	cmd = flowCommand(t, waitFlowModifications(t, amfCb, 3)[2])
	if len(cmd.QoSRules) != 1 || cmd.QoSRules[0].OperationCode != fgs.QoSRuleOpDelete || cmd.QoSRules[0].Identifier != first.Identifier {
		t.Fatalf("QoS rules %+v, want the first call's rule deleted", cmd.QoSRules)
	}

	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	pushRules(t, s, ref)

	c = waitFlowModifications(t, amfCb, 4)[3]
	cmd = flowCommand(t, c)

	if len(cmd.QoSRules) != 1 || cmd.QoSRules[0].OperationCode != fgs.QoSRuleOpDelete ||
		len(cmd.QoSFlowDescriptions) != 1 || cmd.QoSFlowDescriptions[0].OperationCode != fgs.QoSFlowOpDelete {
		t.Fatalf("command %+v, want the last rule and the flow description deleted", cmd)
	}

	if tr := modifyTransfer(t, c); len(tr.QosFlowToRelease) != 1 || tr.QosFlowToRelease[0].QosFlowIdentifier != voiceQFI {
		t.Fatalf("N2 release list %+v, want QFI %d released", tr.QosFlowToRelease, voiceQFI)
	}
}

func TestRefusedRuleRemovalIsHeldUntilThePolicyChanges(t *testing.T) {
	for _, tc := range []struct {
		name   string
		refuse func(t *testing.T, s *smf.SMF, ref string)
	}{
		{"ue rejects", func(t *testing.T, s *smf.SMF, ref string) {
			ueAnswers(t, s, ref, false)
			ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)
		}},
		{"ran refuses", func(t *testing.T, s *smf.SMF, ref string) {
			ueAnswers(t, s, ref, true)
			ranAnswers(t, s, ref, nil, []uint8{voiceQFI})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, pcf, upf, amfCb, ref := activeFiveGVoice(t)

			pushRules(t, s, ref, voiceRule(), secondCallRule())
			waitFlowModifications(t, amfCb, 2)
			ueAnswers(t, s, ref, true)
			ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

			pushRules(t, s, ref, secondCallRule())
			waitFlowModifications(t, amfCb, 3)

			tc.refuse(t, s, ref)

			if enforcesPeer(lastModify(t, upf), firstCallPeer) {
				t.Fatal("the UPF still enforces the rule the PCF removed (TS 23.502 §4.3.3.2 step 8)")
			}

			realign := waitFlowModifications(t, amfCb, 4)[3]
			if realign.n1Msg != nil {
				ueAnswers(t, s, ref, true)
			}

			if realign.n2Msg != nil {
				ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)
			}

			if err := s.ReconcileSession(context.Background(), ref); err != nil {
				t.Fatalf("ReconcileSession: %v", err)
			}

			time.Sleep(100 * time.Millisecond)

			if got := len(amfCb.modifications()); got != 4 {
				t.Fatalf("%d QoS flow modifications sent, want 4: the refused removal and its realignment, then nothing until the rules change", got)
			}

			if r := pcf.reportedRules(); len(r) != 0 {
				t.Fatalf("reported rules %v, want none: a removal does not fail", r)
			}

			pushRules(t, s, ref)

			cmd := flowCommand(t, waitFlowModifications(t, amfCb, 5)[4])
			if len(cmd.QoSFlowDescriptions) != 1 || cmd.QoSFlowDescriptions[0].OperationCode != fgs.QoSFlowOpDelete {
				t.Fatalf("command %+v, want the flow deleted once the last call ended", cmd)
			}
		})
	}
}

func TestRANRefusingTheFlowWithdrawsItFromTheUE(t *testing.T) {
	s, pcf, _, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)

	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, nil, []uint8{voiceQFI})

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#1"}) {
		t.Fatalf("reported rules %v, want the call failed (TS 29.214 §4.4.6.2)", r)
	}

	c := waitFlowModifications(t, amfCb, 2)[1]
	if c.n2Msg != nil {
		t.Fatal("the withdrawal asked the RAN to release a QFI it never admitted")
	}

	cmd := flowCommand(t, c)
	if len(cmd.QoSRules) != 1 || cmd.QoSRules[0].OperationCode != fgs.QoSRuleOpDelete || len(cmd.QoSFlowDescriptions) != 1 {
		t.Fatalf("command %+v, want the UE's rule and flow description deleted (TS 23.502 §4.3.3.2 step 7)", cmd)
	}
}

func TestUERejectingAnAdmittedFlowReleasesItOnN2(t *testing.T) {
	s, pcf, _, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)

	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)
	ueAnswers(t, s, ref, false)

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#1"}) {
		t.Fatalf("reported rules %v, want the call failed", r)
	}

	c := waitFlowModifications(t, amfCb, 2)[1]
	if c.n1Msg != nil {
		t.Fatal("the release sent the UE a command for a flow it rejected")
	}

	if tr := modifyTransfer(t, c); len(tr.QosFlowToRelease) != 1 || tr.QosFlowToRelease[0].QosFlowIdentifier != voiceQFI {
		t.Fatalf("N2 release list %+v, want the admitted QFI released", tr.QosFlowToRelease)
	}
}

func TestEPSFallbackKeepsTheVoiceRulePending(t *testing.T) {
	s, pcf, upf, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)

	ranFallsBack(t, s, ref)

	if slices.ContainsFunc(lastModify(t, upf).UpdatePDRs, func(p models.PDR) bool { return p.PDI.QFI == voiceQFI }) {
		t.Fatal("the voice flow's user plane outlived the fallback")
	}

	requireOneFlowModification(t, amfCb)

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none: EPS fallback keeps the PCC rules (TS 23.502 §4.13.6.1 step 4)", r)
	}
}

func waitReportedRules(t *testing.T, pcf *fakePCF) []string {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for len(pcf.reportedRules()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	return pcf.reportedRules()
}

func ranFailsWithCause(t *testing.T, s *smf.SMF, ref string, value int) {
	t.Helper()

	raw, err := (&libngap.PDUSessionResourceModifyUnsuccessfulTransfer{
		Cause: libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: value},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateSmContextN2InfoPduResModifyFail(context.Background(), ref, raw); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResModifyFail: %v", err)
	}
}

func TestRANRefusingAJoiningCallKeepsTheFirstCall(t *testing.T) {
	s, pcf, _, amfCb, ref := activeFiveGVoice(t)

	pushRules(t, s, ref, voiceRule(), secondCallRule())
	waitFlowModifications(t, amfCb, 2)

	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, nil, []uint8{voiceQFI})

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;2#1"}) {
		t.Fatalf("reported rules %v, want only the joining call failed: the RAN kept the previous configuration (TS 38.413 §8.2.3.2)", r)
	}

	if reports := pcf.reportedFailures(); reports[0].cause != smf.ResourcesNotAllocated {
		t.Fatalf("report %+v, want a resource allocation failure, not a bearer release", reports[0])
	}

	cmd := flowCommand(t, waitFlowModifications(t, amfCb, 3)[2])
	if len(cmd.QoSRules) != 1 || cmd.QoSRules[0].OperationCode != fgs.QoSRuleOpDelete ||
		len(cmd.QoSFlowDescriptions) != 1 || flowDescriptionRate(t, cmd.QoSFlowDescriptions[0], fgs.QoSFlowParamGFBRUplink) != 41 {
		t.Fatalf("command %+v, want the UE aligned back to the first call alone (TS 23.502 §4.3.3.2 step 7)", cmd)
	}

	if c := amfCb.modifications()[2]; c.n2Msg != nil {
		t.Fatal("the realignment touched the RAN, which kept the previous configuration")
	}
}

func TestHandoverInterruptingAFlowRetriesWithoutReporting(t *testing.T) {
	s, pcf, _, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)

	ranFailsWithCause(t, s, ref, libngap.CauseRadioNetworkXnHandoverTriggered)

	if c := waitFlowModifications(t, amfCb, 2)[1]; len(flowCommand(t, c).QoSRules) != 1 {
		t.Fatal("the flow was not set up again after the handover (TS 24.501 §6.3.2.5 f)")
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none for a handover collision", r)
	}
}

func TestRANAnsweringWithoutListingTheFlowAppliesIt(t *testing.T) {
	tests := []struct {
		name    string
		fixture func(t *testing.T) (*smf.SMF, *fakePCF, *fakeUPF, *fakeAMF, string)
		rules   []smf.PCCRule
		mods    int
	}{
		{"setup activates the flow", fiveGVoiceFixture, []smf.PCCRule{voiceRule()}, 1},
		{"modification is applied without realigning the UE", activeFiveGVoice, []smf.PCCRule{voiceRule(), secondCallRule()}, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, pcf, upf, amfCb, ref := tc.fixture(t)

			pushRules(t, s, ref, tc.rules...)
			waitFlowModifications(t, amfCb, tc.mods)

			ueAnswers(t, s, ref, true)
			ranAnswers(t, s, ref, nil, nil)

			if m := lastModify(t, upf); !slices.ContainsFunc(m.UpdatePDRs, func(p models.PDR) bool { return p.PDI.UEIPAddress.IsValid() && p.QERID >= 256 }) {
				t.Fatalf("UPF PDRs %+v, want the voice downlink installed", m.UpdatePDRs)
			}

			time.Sleep(100 * time.Millisecond)

			if r := pcf.reportedRules(); len(r) != 0 {
				t.Fatalf("reported rules %v, want none: a successful response that lists no failed flow applies them (TS 38.413 §8.2.3.2)", r)
			}

			if got := len(amfCb.modifications()); got != tc.mods {
				t.Fatalf("%d QoS flow modifications sent, want %d: no realignment of the UE", got, tc.mods)
			}
		})
	}
}

func TestDownlinkOnlyRuleCreatesAQoSFlow(t *testing.T) {
	s, pcf, _, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, downlinkOnlyRule())

	cmd := flowCommand(t, waitFlowModifications(t, amfCb, 1)[0])

	if len(cmd.QoSRules) != 1 || len(cmd.QoSRules[0].Filters) != 1 || cmd.QoSRules[0].Filters[0].Direction != fgs.PacketFilterDownlink {
		t.Fatalf("QoS rules %+v, want one carrying only the rule's downlink filter", cmd.QoSRules)
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none", r)
	}
}

func TestRuleWithAnUnconstrainedFilterIsReported(t *testing.T) {
	s, pcf, _, amfCb, ref := fiveGVoiceFixture(t)

	rule := voiceRule()
	rule.Filters = append(rule.Filters, models.SDFFilter{Direction: models.FilterDownlink, Precedence: 34})

	pushRules(t, s, ref, rule)

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{rule.ID}) {
		t.Fatalf("reported rules %v, want the rule whose filter has no component (TS 24.501 §9.11.4.13, §6.2.5.1.1)", r)
	}

	if m := amfCb.modifications(); len(m) != 0 {
		t.Fatalf("%d QoS flow modifications sent, want none", len(m))
	}
}
