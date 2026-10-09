// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	libngap "github.com/ellanetworks/core/ngap"
)

const (
	sessionEBI = 5
	flowEBI    = 6
)

func interworkingVoiceFixture(t *testing.T) (*smf.SMF, *fakeAMF, string) {
	t.Helper()

	pcf, store, upf, amfCb, _ := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	sc := establish5GSWithEBI(t, s, sessionEBI)

	return s, amfCb, sc.Ref
}

func mappedParameter(t *testing.T, c fgs.MappedEPSBearerContext, id fgs.EPSParameterIdentifier) []byte {
	t.Helper()

	for _, p := range c.Parameters {
		if p.Identifier == id {
			return p.Contents
		}
	}

	t.Fatalf("mapped EPS bearer context %+v carries no %s", c, id)

	return nil
}

func TestInterworkingFlowCarriesItsEPSBearer(t *testing.T) {
	s, amfCb, ref := interworkingVoiceFixture(t)

	pushRules(t, s, ref, voiceRule())

	c := waitFlowModifications(t, amfCb, 1)[0]
	cmd := flowCommand(t, c)

	if ebi, ok := cmd.QoSFlowDescriptions[0].EPSBearerID(); !ok || ebi != flowEBI {
		t.Fatalf("QoS flow description EBI %d (present %v), want the AMF-assigned %d (TS 23.502 §4.11.1.4.1)", ebi, ok, flowEBI)
	}

	if len(cmd.MappedEPSBearerContexts) != 1 {
		t.Fatalf("mapped EPS bearer contexts %+v, want one for the flow (TS 24.501 §9.11.4.8)", cmd.MappedEPSBearerContexts)
	}

	mapped := cmd.MappedEPSBearerContexts[0]
	if mapped.EPSBearerIdentity != flowEBI || mapped.Operation != fgs.MappedEPSBearerOpCreate {
		t.Fatalf("mapped EPS bearer context %+v, want EBI %d created", mapped, flowEBI)
	}

	qos, err := eps.ParseEPSQoS(mappedParameter(t, mapped, fgs.EPSParameterMappedEPSQoS))
	if err != nil {
		t.Fatalf("mapped EPS QoS: %v", err)
	}

	if rates, ok := qos.GBRBitRates(); qos.QCI != 1 || !ok || rates.GuaranteedDownlinkKbps != 41 || rates.MaxUplinkKbps != 41 {
		t.Fatalf("mapped EPS QoS %+v, want QCI 1 with the flow's 41 kbps (TS 29.513 §7.4.2)", qos)
	}

	tft, err := eps.ParseTrafficFlowTemplate(mappedParameter(t, mapped, fgs.EPSParameterTrafficFlowTemplate))
	if err != nil {
		t.Fatalf("mapped TFT: %v", err)
	}

	if tft.Operation != eps.TFTCreate || len(tft.Filters) != 2 || tft.Filters[0].Precedence == tft.Filters[1].Precedence {
		t.Fatalf("mapped TFT %+v, want both filters with their own precedence (TS 24.008 §10.5.6.12)", tft)
	}

	tr := modifyTransfer(t, c)
	if item := tr.QosFlowAddOrModifyRequest[0]; item.ERABID == nil || *item.ERABID != flowEBI {
		t.Fatalf("N2 QoS flow %+v, want E-RAB ID %d (TS 38.413 §9.3.4.3)", item, flowEBI)
	}

	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	pushRules(t, s, ref)

	cmd = flowCommand(t, waitFlowModifications(t, amfCb, 2)[1])
	if len(cmd.MappedEPSBearerContexts) != 1 || cmd.MappedEPSBearerContexts[0].Operation != fgs.MappedEPSBearerOpDelete ||
		cmd.MappedEPSBearerContexts[0].EPSBearerIdentity != flowEBI {
		t.Fatalf("mapped EPS bearer contexts %+v, want EBI %d deleted with the flow", cmd.MappedEPSBearerContexts, flowEBI)
	}

	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{}, nil)

	waitFor(t, "the flow EBI released to the AMF", func() bool { return slices.Equal(amfCb.released(), []uint8{flowEBI}) })
}

func TestInterworkingFlowModificationUpdatesTheMappedBearer(t *testing.T) {
	s, amfCb, ref := interworkingVoiceFixture(t)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	pushRules(t, s, ref, voiceRule(), secondCallRule())

	cmd := flowCommand(t, waitFlowModifications(t, amfCb, 2)[1])
	if len(cmd.MappedEPSBearerContexts) != 1 || cmd.MappedEPSBearerContexts[0].Operation != fgs.MappedEPSBearerOpModify {
		t.Fatalf("mapped EPS bearer contexts %+v, want EBI %d modified", cmd.MappedEPSBearerContexts, flowEBI)
	}

	mapped := cmd.MappedEPSBearerContexts[0]

	qos, err := eps.ParseEPSQoS(mappedParameter(t, mapped, fgs.EPSParameterMappedEPSQoS))
	if err != nil {
		t.Fatalf("mapped EPS QoS: %v", err)
	}

	if ebi, ok := cmd.QoSFlowDescriptions[0].EPSBearerID(); !ok || ebi != flowEBI {
		t.Fatalf("modified QoS flow description %+v, want EBI %d repeated: E=1 replaces every parameter (TS 24.501 §9.11.4.12)", cmd.QoSFlowDescriptions[0], flowEBI)
	}

	if rates, _ := qos.GBRBitRates(); rates.GuaranteedUplinkKbps != 88 {
		t.Fatalf("mapped EPS QoS %+v, want the summed 82 kbps rounded up to the 8 kbps steps of TS 24.301 §9.9.4.3", rates)
	}

	tft, err := eps.ParseTrafficFlowTemplate(mappedParameter(t, mapped, fgs.EPSParameterTrafficFlowTemplate))
	if err != nil {
		t.Fatalf("mapped TFT: %v", err)
	}

	if tft.Operation != eps.TFTAddFilters || len(tft.Filters) != 2 {
		t.Fatalf("mapped TFT %+v, want the second call's two filters added (TS 24.008 table 10.5.162)", tft)
	}

	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	pushRules(t, s, ref, secondCallRule())

	cmd = flowCommand(t, waitFlowModifications(t, amfCb, 3)[2])

	var deleted []uint8

	for _, m := range cmd.MappedEPSBearerContexts {
		for _, p := range m.Parameters {
			if p.Identifier != fgs.EPSParameterTrafficFlowTemplate {
				continue
			}

			if tft, err := eps.ParseTrafficFlowTemplate(p.Contents); err == nil && tft.Operation == eps.TFTDeleteFilters {
				deleted = append(deleted, tft.DeleteIdentifiers...)
			}
		}
	}

	if len(deleted) != 2 {
		t.Fatalf("mapped EPS bearer contexts %+v, want the first call's two filters deleted", cmd.MappedEPSBearerContexts)
	}
}

func TestFlowWithoutAFreeEBIStaysIn5GS(t *testing.T) {
	s, amfCb, ref := interworkingVoiceFixture(t)

	amfCb.mu.Lock()
	amfCb.noFreeEBI = true
	amfCb.mu.Unlock()

	pushRules(t, s, ref, voiceRule())

	c := waitFlowModifications(t, amfCb, 1)[0]
	cmd := flowCommand(t, c)

	if _, ok := cmd.QoSFlowDescriptions[0].EPSBearerID(); ok || cmd.MappedEPSBearerContexts != nil {
		t.Fatalf("command %+v, want no EPS bearer for a flow the AMF gave no EBI (TS 23.502 §4.11.1.4.1)", cmd)
	}

	if item := modifyTransfer(t, c).QosFlowAddOrModifyRequest[0]; item.ERABID != nil {
		t.Fatalf("N2 QoS flow %+v, want no E-RAB ID", item)
	}
}

func TestSessionSetupCarriesTheDefaultERABID(t *testing.T) {
	s, amfCb, ref := interworkingVoiceFixture(t)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	if err := s.DeactivateSmContext(context.Background(), ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	n2, err := s.ActivateSmContext(context.Background(), ref)
	if err != nil {
		t.Fatalf("ActivateSmContext: %v", err)
	}

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(n2)
	if err != nil {
		t.Fatalf("decode the setup transfer: %v", err)
	}

	got := map[uint8]uint8{}

	for _, item := range transfer.QosFlowSetupRequest {
		if item.ERABID != nil {
			got[uint8(item.QosFlowIdentifier)] = uint8(*item.ERABID)
		}
	}

	if got[1] != sessionEBI || got[voiceQFI] != flowEBI {
		t.Fatalf("E-RAB IDs by QFI %v, want the default flow on %d and the voice flow on %d (TS 38.413 §9.3.4.1)", got, sessionEBI, flowEBI)
	}
}

func interworkingEPSVoice(t *testing.T) (*smf.SMF, *fakeMME, string) {
	t.Helper()

	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{policy: epsPolicy()}, store, upf, &fakeAMF{})

	mmeCb := &fakeMME{}
	s.SetMME(mmeCb)

	req := epsRequest(1)
	req.PDUSessionID = 1
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.ModifyEPSSession(context.Background(), bearer.Ref, epsTestEBI, models.FTEID{TEID: 0x55, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 3})}, nil); err != nil {
		t.Fatal(err)
	}

	return s, mmeCb, bearer.Ref
}

func mappedFiveGSQoS(t *testing.T, containers []nas.PCOContainer) (fgs.QoSRules, fgs.QoSFlowDescriptions) {
	t.Helper()

	var (
		rules fgs.QoSRules
		flows fgs.QoSFlowDescriptions
	)

	for _, c := range containers {
		var err error

		switch c.ID {
		case nas.PCOContainerQoSRules:
			rules, err = fgs.ParseQoSRules(c.Content)
		case nas.PCOContainerQoSFlowDescriptions:
			flows, err = fgs.ParseQoSFlowDescriptions(c.Content)
		}

		if err != nil {
			t.Fatalf("container %#x: %v", c.ID, err)
		}
	}

	return rules, flows
}

func TestDedicatedBearerCarriesItsQoSFlowForN1Mode(t *testing.T) {
	s, mmeCb, ref := interworkingEPSVoice(t)

	pushRules(t, s, ref, voiceRule())

	activations := mmeCb.dedicatedActivations()
	if len(activations) != 1 {
		t.Fatalf("%d activations, want one", len(activations))
	}

	rules, flows := mappedFiveGSQoS(t, activations[0].MappedFiveGSQoS)

	if len(rules) != 1 || rules[0].OperationCode != fgs.QoSRuleOpCreate || rules[0].Parameters == nil || rules[0].Parameters.QFI < 2 ||
		rules[0].Identifier < 2 || len(rules[0].Filters) != 2 {
		t.Fatalf("QoS rules %+v, want the call's rule on a dedicated QFI (TS 24.501 §6.1.4.1)", rules)
	}

	if len(flows) != 1 || flows[0].QFI != rules[0].Parameters.QFI || flows[0].OperationCode != fgs.QoSFlowOpCreate ||
		flowDescriptionRate(t, flows[0], fgs.QoSFlowParamGFBRUplink) != 41 {
		t.Fatalf("QoS flow descriptions %+v, want the rule's QFI with its GFBR", flows)
	}

	if err := s.DedicatedBearerActivated(context.Background(), ref, activations[0].SGW.TEID, 6, models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	pushRules(t, s, ref, voiceRule(), secondCallRule())

	mod := waitDedicatedModifications(t, mmeCb, 1)[0]

	added, modified := mappedFiveGSQoS(t, mod.MappedFiveGSQoS)
	if len(added) != 1 || added[0].OperationCode != fgs.QoSRuleOpCreate || added[0].Identifier == rules[0].Identifier ||
		added[0].Parameters.QFI != rules[0].Parameters.QFI || added[0].Parameters.Precedence == rules[0].Parameters.Precedence {
		t.Fatalf("QoS rules %+v, want the second call's rule on the same QFI with its own QRI and precedence", added)
	}

	if len(modified) != 1 || modified[0].OperationCode != fgs.QoSFlowOpModify {
		t.Fatalf("QoS flow descriptions %+v, want the flow's rates modified", modified)
	}
}

func TestDedicatedBearerWithoutN1ModeCarriesNoQoSFlow(t *testing.T) {
	s, _, _, mmeCb, ref := epsReconcileFixture(t, 0)

	pushRules(t, s, ref, voiceRule())

	if a := mmeCb.dedicatedActivations(); len(a) != 1 || a[0].MappedFiveGSQoS != nil {
		t.Fatalf("activations %+v, want no 5GS QoS for a PDN connection without a PDU session ID", a)
	}
}

func interworkingCall(t *testing.T) (*smf.SMF, *fakePCF, *fakeUPF, string) {
	t.Helper()

	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ref := establish5GSWithEBI(t, s, sessionEBI).Ref

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	return s, pcf, upf, ref
}

func bearerTEIDUplinkPDRs(m *models.ModifyRequest) int {
	n := 0

	for _, p := range m.UpdatePDRs {
		if p.PDI.SourceInterface == models.InterfaceAccess && p.PDI.LocalFTEID != nil && p.PDI.LocalFTEID.ChooseID == 16 && len(p.PDI.SDFFilters) > 0 {
			n++
		}
	}

	return n
}

func TestHandoverToEPSCarriesTheVoiceBearer(t *testing.T) {
	s, pcf, upf, ref := interworkingCall(t)
	ctx := context.Background()

	bearer, err := s.CreateEPSSession(ctx, epsMove(movedPDUSessionID))
	if err != nil {
		t.Fatalf("move to EPS: %v", err)
	}

	if len(bearer.Dedicated) != 1 {
		t.Fatalf("dedicated bearer contexts %+v, want the voice flow (TS 23.502 §4.11.1.2.1 step 2)", bearer.Dedicated)
	}

	d := bearer.Dedicated[0]
	if d.EBI != flowEBI || d.QCI != 1 || d.GBR.Uplink.Bps() != 41000 || len(d.Filters) != 2 || d.SGW.TEID == 0 || d.SGW.TEID == bearer.SGW.TEID {
		t.Fatalf("dedicated bearer context %+v, want EBI %d at QCI 1 with its own S-GW TEID", d, flowEBI)
	}

	if bearerTEIDUplinkPDRs(lastModify(t, upf)) == 0 {
		t.Fatal("no uplink on the bearer's own TEID at preparation (TS 23.502 §4.11.1.2.1 step 2b)")
	}

	if err := s.ModifyEPSSession(ctx, bearer.Ref, epsTestEBI, models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}, nil); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	enb := models.FTEID{TEID: 0x6002, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.DedicatedBearerMoved(ctx, ref, d.SGW.TEID, enb); err != nil {
		t.Fatalf("DedicatedBearerMoved on the TEID handed to the MME: %v", err)
	}

	m := lastModify(t, upf)
	if slices.ContainsFunc(m.UpdatePDRs, func(p models.PDR) bool { return p.PDI.QFI == voiceQFI }) {
		t.Fatal("the voice flow still matches on its QFI after the move to EPS")
	}

	if !slices.ContainsFunc(m.UpdateFARs, func(f models.FAR) bool {
		return f.ForwardingParameters != nil && f.ForwardingParameters.OuterHeaderCreation != nil && f.ForwardingParameters.OuterHeaderCreation.TEID == enb.TEID
	}) {
		t.Fatalf("FARs %+v, want the voice downlink on the eNB's E-RAB endpoint", m.UpdateFARs)
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none for a flow that survived the move (TS 29.214 §4.4.6.2)", r)
	}
}

func TestFlowWithoutEBIIsReleasedAtTheMoveToEPS(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	amfCb.noFreeEBI = true
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ref := establish5GSWithEBI(t, s, sessionEBI).Ref

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	ctx := context.Background()

	bearer, err := s.CreateEPSSession(ctx, epsMove(movedPDUSessionID))
	if err != nil {
		t.Fatalf("move to EPS: %v", err)
	}

	if len(bearer.Dedicated) != 0 {
		t.Fatalf("dedicated bearer contexts %+v, want none for a flow without EBI", bearer.Dedicated)
	}

	if err := s.ModifyEPSSession(ctx, bearer.Ref, epsTestEBI, models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}, nil); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	if r := waitReportedRules(t, pcf); !slices.Contains(r, voiceRule().ID) {
		t.Fatalf("reported rules %v, want the voice rule lost (TS 23.502 §4.11.1.2.1 step 14a)", r)
	}
}

func TestAbandonedMoveRemovesTheTargetUplink(t *testing.T) {
	s, _, upf, ref := interworkingCall(t)
	ctx := context.Background()

	if _, err := s.CreateEPSSession(ctx, epsMove(movedPDUSessionID)); err != nil {
		t.Fatalf("move to EPS: %v", err)
	}

	if err := s.ReleaseEPSSession(ctx, ref); err != nil {
		t.Fatalf("ReleaseEPSSession: %v", err)
	}

	m := lastModify(t, upf)
	if bearerTEIDUplinkPDRs(m) != 0 || len(m.RemovePDRs) == 0 {
		t.Fatalf("modification %+v, want the target uplink removed when the MME drops the move", m)
	}
}

func TestIdleMoveToEPSCarriesTheVoiceBearer(t *testing.T) {
	s, _, upf, ref := interworkingCall(t)
	ctx := context.Background()

	if err := s.DeactivateSmContext(ctx, ref, true); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	bearer, err := s.TransferIdleToEPS(ctx, testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai)
	if err != nil {
		t.Fatalf("TransferIdleToEPS: %v", err)
	}

	if len(bearer.Dedicated) != 1 || bearer.Dedicated[0].EBI != flowEBI || bearer.Dedicated[0].SGW.TEID == 0 {
		t.Fatalf("dedicated bearer contexts %+v, want the held voice flow (TS 23.502 §4.11.1.3.2 step 5c)", bearer.Dedicated)
	}

	if bearerTEIDUplinkPDRs(lastModify(t, upf)) == 0 {
		t.Fatal("the voice bearer has no uplink on its own TEID after the idle move")
	}
}

func TestUEDeletingTheMappedBearerKeepsTheFlowIn5GS(t *testing.T) {
	s, amfCb, ref := interworkingVoiceFixture(t)

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	cause := fgs.GSMCauseInvalidMappedEPSBearerIdentity

	raw, err := (&fgs.PDUSessionModificationRequest{
		PDUSessionID:            fgs.PDUSessionID(movedPDUSessionID),
		PTI:                     7,
		Cause:                   &cause,
		MappedEPSBearerContexts: fgs.MappedEPSBearerContexts{{EPSBearerIdentity: flowEBI, Operation: fgs.MappedEPSBearerOpDelete}},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	res, err := s.UpdateSmContextN1Msg(context.Background(), ref, raw)
	if err != nil || res == nil || res.N1Msg == nil {
		t.Fatalf("UpdateSmContextN1Msg = %+v, %v; want a command (TS 24.501 §6.4.2.3)", res, err)
	}

	cmd, err := fgs.ParsePDUSessionModificationCommand(res.N1Msg)
	if err != nil || cmd.PTI != 7 || len(cmd.MappedEPSBearerContexts) != 1 || cmd.MappedEPSBearerContexts[0].Operation != fgs.MappedEPSBearerOpDelete {
		t.Fatalf("answer %+v (%v), want the mapped EPS bearer context deleted with the UE's PTI", cmd, err)
	}

	if len(cmd.QoSFlowDescriptions) != 1 || cmd.QoSFlowDescriptions[0].QFI != voiceQFI {
		t.Fatalf("QoS flow descriptions %+v, want the voice flow restated", cmd.QoSFlowDescriptions)
	}

	if _, ok := cmd.QoSFlowDescriptions[0].EPSBearerID(); ok {
		t.Fatal("the voice flow still names an EPS bearer the UE refused")
	}

	if !slices.Equal(amfCb.released(), []uint8{flowEBI}) {
		t.Fatalf("released EBIs %v, want %d back to the AMF", amfCb.released(), flowEBI)
	}
}

func TestMoveToEPSCarriesTheUEsViewOfARealignedFlow(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ref := establish5GSWithEBI(t, s, sessionEBI).Ref

	pushRules(t, s, ref, voiceRule())
	waitFlowModifications(t, amfCb, 1)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	pushRules(t, s, ref, voiceRule(), secondCallRule())
	waitFlowModifications(t, amfCb, 2)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, nil, []uint8{voiceQFI})

	bearer, err := s.CreateEPSSession(context.Background(), epsMove(movedPDUSessionID))
	if err != nil {
		t.Fatalf("move to EPS: %v", err)
	}

	if len(bearer.Dedicated) != 1 || len(bearer.Dedicated[0].Filters) != 4 || bearer.Dedicated[0].GBR.Uplink.Bps() != 82000 {
		t.Fatalf("dedicated bearer contexts %+v, want the UE's mapped EPS bearer with both calls (TS 24.501 §6.1.4.1)", bearer.Dedicated)
	}
}

func TestHandoverToEPSBindsTheVoiceBearerWithTheDefaultBearer(t *testing.T) {
	s, _, upf, ref := interworkingCall(t)
	ctx := context.Background()

	bearer, err := s.CreateEPSSession(ctx, epsMove(movedPDUSessionID))
	if err != nil {
		t.Fatalf("move to EPS: %v", err)
	}

	defaultENB := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	voiceENB := models.FTEID{TEID: 0x6002, Addr: netip.MustParseAddr("192.168.40.10")}

	if err := s.ModifyEPSSession(ctx, bearer.Ref, epsTestEBI, defaultENB,
		[]models.DedicatedBearerEndpoint{{SGWTEID: bearer.Dedicated[0].SGW.TEID, ENB: voiceENB}}); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	m := lastModify(t, upf)
	for _, teid := range []uint32{defaultENB.TEID, voiceENB.TEID} {
		if !slices.ContainsFunc(m.UpdateFARs, func(f models.FAR) bool {
			return f.ForwardingParameters != nil && f.ForwardingParameters.OuterHeaderCreation != nil && f.ForwardingParameters.OuterHeaderCreation.TEID == teid
		}) {
			t.Fatalf("FARs %+v, want the default and voice downlinks switched in one modification (no voice on the default bearer)", m.UpdateFARs)
		}
	}

	before := modifyCount(upf)

	if err := s.DedicatedBearerMoved(ctx, ref, bearer.Dedicated[0].SGW.TEID, voiceENB); err != nil {
		t.Fatalf("DedicatedBearerMoved: %v", err)
	}

	if got := modifyCount(upf) - before; got != 0 {
		t.Fatalf("%d PFCP modifications for an endpoint already bound, want 0", got)
	}
}
