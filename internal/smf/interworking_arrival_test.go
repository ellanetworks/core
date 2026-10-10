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
	"github.com/ellanetworks/core/nas/fgs"
	libngap "github.com/ellanetworks/core/ngap"
)

type arrivingCall struct {
	s     *smf.SMF
	pcf   *fakePCF
	upf   *fakeUPF
	amfCb *fakeAMF
	sc    *smf.SMContext
	qfi   uint8
	sgw   uint32
}

func epsCallForArrival(t *testing.T) arrivingCall {
	t.Helper()

	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	sc := establishEPSForArrival(t, s)

	pushRules(t, s, sc.Ref, voiceRule())

	activations := mmeCb.dedicatedActivations()
	if len(activations) != 1 {
		t.Fatalf("%d activations, want one", len(activations))
	}

	_, flows := mappedFiveGSQoS(t, activations[0].MappedFiveGSQoS)
	if len(flows) != 1 {
		t.Fatalf("QoS flow descriptions %+v, want the bearer's 5GS QoS", flows)
	}

	enb := models.FTEID{TEID: 0x6002, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.DedicatedBearerActivated(context.Background(), sc.Ref, activations[0].SGW.TEID, flowEBI, enb); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	return arrivingCall{s: s, pcf: pcf, upf: upf, amfCb: amfCb, sc: sc, qfi: flows[0].QFI, sgw: activations[0].SGW.TEID}
}

func qfiUplinkOnSessionTEID(m *models.ModifyRequest, qfi uint8) bool {
	return slices.ContainsFunc(m.UpdatePDRs, func(p models.PDR) bool {
		return p.PDI.SourceInterface == models.InterfaceAccess && p.PDI.LocalFTEID != nil && p.PDI.LocalFTEID.ChooseID == 1 &&
			p.PDI.QFI == qfi && len(p.PDI.SDFFilters) > 0
	})
}

func handoverAck(t *testing.T, qfis ...uint8) []byte {
	t.Helper()

	transfer := libngap.HandoverRequestAcknowledgeTransfer{DLNGUUPTNLInformation: testTunnel(targetGnbTEID, targetGnbIPv4)}
	for _, qfi := range qfis {
		transfer.QosFlowSetupResponse = append(transfer.QosFlowSetupResponse, libngap.QosFlowItemWithDataForwarding{QosFlowIdentifier: libngap.QosFlowIdentifier(qfi)})
	}

	b, err := transfer.Marshal()
	if err != nil {
		t.Fatalf("build the Handover Request Acknowledge transfer: %v", err)
	}

	return b
}

func handOverTo5GS(t *testing.T, c arrivingCall, admitted ...uint8) {
	t.Helper()

	ctx := context.Background()

	ref, n2, flowEBIs, err := c.s.PrepareSmContextFromEPS(ctx, testSUPI(), arrivingPDUSessionID, epsTestEBI, testDNN, testSnssai)
	if err != nil {
		t.Fatalf("PrepareSmContextFromEPS: %v", err)
	}

	if !slices.Equal(flowEBIs, []uint8{flowEBI}) {
		t.Fatalf("flow EBIs %v, want the voice bearer's %d for the AMF (TS 23.502 §4.11.1.4.2)", flowEBIs, flowEBI)
	}

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(n2)
	if err != nil {
		t.Fatalf("parse the Handover Request transfer: %v", err)
	}

	if !slices.ContainsFunc(transfer.QosFlowSetupRequest, func(f libngap.QosFlowSetupRequestItem) bool {
		return uint8(f.QosFlowIdentifier) == c.qfi && f.ERABID != nil && uint8(*f.ERABID) == flowEBI
	}) {
		t.Fatalf("QoS flows %+v, want the voice flow with its E-RAB ID (TS 23.502 §4.11.1.2.2)", transfer.QosFlowSetupRequest)
	}

	if !qfiUplinkOnSessionTEID(lastModify(t, c.upf), c.qfi) {
		t.Fatal("no uplink for the voice QFI on the session TEID at preparation (TS 23.502 §4.11.1.2.2.2 step 6)")
	}

	if _, err := c.s.UpdateSmContextN2HandoverPrepared(ctx, ref, handoverAck(t, admitted...)); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	if err := c.s.UpdateSmContextN2HandoverComplete(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverComplete: %v", err)
	}
}

func TestHandoverFromEPSCarriesTheVoiceFlow(t *testing.T) {
	c := epsCallForArrival(t)

	handOverTo5GS(t, c, models.DefaultQFI, c.qfi)

	if got := c.s.HandoverAdmittedFlowEBIs(c.sc.Ref); len(got) != 0 {
		t.Fatalf("admitted flow EBIs %v after completion, want the handover state cleared", got)
	}

	if access := accessOf(t, c.sc); access != smf.Access5G {
		t.Fatalf("session on %s after the handover", access)
	}

	m := lastModify(t, c.upf)
	if !qfiUplinkOnSessionTEID(m, c.qfi) || bearerTEIDUplinkPDRs(m) != 0 {
		t.Fatalf("modification %+v, want the voice uplink on its QFI and no S-GW bearer TEID", m)
	}

	if r := c.pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none for a flow that survived the move (TS 29.214 §4.4.6.2)", r)
	}
}

func TestPreparedHandoverFromEPSReportsTheAdmittedFlowEBIs(t *testing.T) {
	c := epsCallForArrival(t)
	ctx := context.Background()

	ref, _, _, err := c.s.PrepareSmContextFromEPS(ctx, testSUPI(), arrivingPDUSessionID, epsTestEBI, testDNN, testSnssai)
	if err != nil {
		t.Fatalf("PrepareSmContextFromEPS: %v", err)
	}

	if _, err := c.s.UpdateSmContextN2HandoverPrepared(ctx, ref, handoverAck(t, models.DefaultQFI)); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	if got := c.s.HandoverAdmittedFlowEBIs(ref); len(got) != 0 {
		t.Fatalf("admitted flow EBIs %v, want none: the MME releases the voice E-RAB (TS 36.413 §8.4.1.2)", got)
	}

	if _, err := c.s.UpdateSmContextN2HandoverPrepared(ctx, ref, handoverAck(t, models.DefaultQFI, c.qfi)); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	if got := c.s.HandoverAdmittedFlowEBIs(ref); !slices.Equal(got, []uint8{flowEBI}) {
		t.Fatalf("admitted flow EBIs %v, want %d", got, flowEBI)
	}
}

func TestFlowNotAdmittedAtHandoverFromEPSIsDeletedFromTheUE(t *testing.T) {
	c := epsCallForArrival(t)

	handOverTo5GS(t, c, models.DefaultQFI)

	cmd := flowCommand(t, waitFlowModifications(t, c.amfCb, 1)[0])

	if !slices.ContainsFunc(cmd.QoSFlowDescriptions, func(d fgs.QoSFlowDescription) bool {
		return d.QFI == c.qfi && d.OperationCode == fgs.QoSFlowOpDelete
	}) || !slices.ContainsFunc(cmd.MappedEPSBearerContexts, func(m fgs.MappedEPSBearerContext) bool {
		return m.EPSBearerIdentity == flowEBI && m.Operation == fgs.MappedEPSBearerOpDelete
	}) {
		t.Fatalf("command %+v, want the voice flow and its EPS bearer deleted (TS 23.502 §4.11.1.2.2.3 steps 1-2)", cmd)
	}

	if r := waitReportedRules(t, c.pcf); !slices.Contains(r, voiceRule().ID) {
		t.Fatalf("reported rules %v, want the voice rule lost", r)
	}
}

func TestIdleMoveTo5GSKeepsTheVoiceFlow(t *testing.T) {
	c := epsCallForArrival(t)
	ctx := context.Background()

	ref, flowEBIs, err := c.s.TransferIdleTo5GS(ctx, testSUPI(), arrivingPDUSessionID, epsTestEBI, testDNN, testSnssai)
	if err != nil {
		t.Fatalf("TransferIdleTo5GS: %v", err)
	}

	if !slices.Equal(flowEBIs, []uint8{flowEBI}) {
		t.Fatalf("flow EBIs %v, want %d for the Registration Accept (TS 23.502 §4.11.1.3.3 step 14)", flowEBIs, flowEBI)
	}

	n2, err := c.s.ActivateSmContext(ctx, ref)
	if err != nil {
		t.Fatalf("ActivateSmContext: %v", err)
	}

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(n2)
	if err != nil {
		t.Fatalf("decode the setup transfer: %v", err)
	}

	if !slices.ContainsFunc(transfer.QosFlowSetupRequest, func(f libngap.QosFlowSetupRequestItem) bool {
		return uint8(f.QosFlowIdentifier) == c.qfi && f.ERABID != nil && uint8(*f.ERABID) == flowEBI
	}) {
		t.Fatalf("QoS flows %+v, want the held voice flow at service request", transfer.QosFlowSetupRequest)
	}

	if r := c.pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none", r)
	}
}

func TestEPSBearerReportedInactiveReleasesTheFlow(t *testing.T) {
	c := epsCallForArrival(t)
	ctx := context.Background()

	ref, _, err := c.s.TransferIdleTo5GS(ctx, testSUPI(), arrivingPDUSessionID, epsTestEBI, testDNN, testSnssai)
	if err != nil {
		t.Fatalf("TransferIdleTo5GS: %v", err)
	}

	c.s.ReleaseInactiveEPSBearers(ctx, ref, []uint8{flowEBI})

	if r := waitReportedRules(t, c.pcf); !slices.Contains(r, voiceRule().ID) {
		t.Fatalf("reported rules %v, want the voice rule lost", r)
	}

	if slices.Contains(c.amfCb.released(), flowEBI) {
		t.Fatalf("released EBIs %v, want %d left to the AMF, which already freed it: a second release could free a reassigned EBI", c.amfCb.released(), flowEBI)
	}

	if m := c.amfCb.modifications(); len(m) != 0 {
		t.Fatalf("%d modifications, want the flow released locally (TS 23.502 §4.11.1.3.3 step 14)", len(m))
	}
}

func TestBearerWithoutItsFiveGSQoSStaysInEPS(t *testing.T) {
	c := epsCallForArrival(t)
	ctx := context.Background()

	c.s.DedicatedBearerWithoutFiveGSQoS(ctx, c.sc.Ref, c.sgw)

	_, n2, flowEBIs, err := c.s.PrepareSmContextFromEPS(ctx, testSUPI(), arrivingPDUSessionID, epsTestEBI, testDNN, testSnssai)
	if err != nil {
		t.Fatalf("PrepareSmContextFromEPS: %v", err)
	}

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(n2)
	if err != nil {
		t.Fatalf("parse the Handover Request transfer: %v", err)
	}

	if len(flowEBIs) != 0 || len(transfer.QosFlowSetupRequest) != 1 {
		t.Fatalf("flow EBIs %v, QoS flows %+v, want only the default flow: the UE holds no 5GS QoS for the bearer (TS 24.301 §6.4.2.3)", flowEBIs, transfer.QosFlowSetupRequest)
	}
}

func TestModificationInFlightAtHandoverFromEPSIsRealignedOn5GS(t *testing.T) {
	c := epsCallForArrival(t)

	pushRules(t, c.s, c.sc.Ref, voiceRule(), secondCallRule())

	handOverTo5GS(t, c, models.DefaultQFI, c.qfi)

	cmd := flowCommand(t, waitFlowModifications(t, c.amfCb, 1)[0])

	if !slices.ContainsFunc(cmd.QoSRules, func(r fgs.QoSRule) bool { return r.OperationCode == fgs.QoSRuleOpDelete }) {
		t.Fatalf("command %+v, want the second call's rule the UE may hold deleted to match the network (TS 24.501 §6.3.2.4 case 11)", cmd)
	}

	if r := c.pcf.reportedRules(); slices.Contains(r, voiceRule().ID) {
		t.Fatalf("reported rules %v, want the first call kept", r)
	}
}
