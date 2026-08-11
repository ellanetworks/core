// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// handoverSetupTransfer is the PDU session transfer a HANDOVER REQUEST carries
// per session: the same PDUSessionResourceSetupRequestTransfer the setup
// procedure uses.
func handoverSetupTransfer(t *testing.T) TransferContainer {
	t.Helper()

	b, err := (&PDUSessionResourceSetupRequestTransfer{
		ULNGUUPTNLInformation: UPTransportLayerInformation{GTPTunnel: GTPTunnel{
			TransportLayerAddress: TransportLayerAddress{192, 168, 1, 1}, GTPTEID: 1,
		}},
		PDUSessionType: PDUSessionTypeIPv4,
		QosFlowSetupRequest: QosFlowSetupRequestList{{
			QosFlowIdentifier: 1,
			QosFlowLevelQosParameters: QosFlowLevelQosParameters{
				QosCharacteristics:             QosCharacteristics{Kind: QosCharacteristicsNonDynamic5QI, NonDynamic5QI: NonDynamic5QIDescriptor{FiveQI: 9}},
				AllocationAndRetentionPriority: AllocationAndRetentionPriority{PriorityLevelARP: 1},
			},
		}},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

const goldenHandoverRequest = "000d00809700000a000a00020001001d000100000f40020400006e000a0c3b9a" +
	"ca00303b9aca0000770009100008000000000000005d0021080000000000000000000000000000000000000000" +
	"00000000000000000000000000490027000005002021000003008b000a01f0c0a80101000000010086000100008800" +
	"07000100000900000000000200010065000302aabb001c00070002f839010040"

func TestHandoverRequestGolden(t *testing.T) {
	in := HandoverRequest{
		AMFUENGAPID: 1, HandoverType: HandoverTypeIntra5GS,
		Cause:                     &Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHandoverDesirableForRadio},
		UEAggregateMaximumBitRate: UEAggregateMaximumBitRate{DL: 1000000000, UL: 1000000000},
		UESecurityCapabilities:    UESecurityCapabilities{NREncryptionAlgorithms: 0x8000, NRIntegrityProtectionAlgorithms: 0x8000},
		SecurityContext:           SecurityContext{NextHopChainingCount: 1},
		PDUSessionResourceSetupListHOReq: PDUSessionResourceSetupListHOReq{{
			PDUSessionID: 5, SNSSAI: SNSSAI{SST: 1}, Transfer: handoverSetupTransfer(t),
		}},
		AllowedNSSAI:                       AllowedNSSAI{{SNSSAI: SNSSAI{SST: 1}}},
		SourceToTargetTransparentContainer: SourceToTargetTransparentContainer{0xaa, 0xbb},
		GUAMI:                              GUAMI{PLMNIdentity: PLMNIdentity{0x02, 0xf8, 0x39}, AMFRegionID: 1, AMFSetID: 1},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenHandoverRequest {
		t.Fatalf("encoded %s, want %s", got, goldenHandoverRequest)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcHandoverResourceAllocation {
		t.Fatalf("decoded %T, want an initiating HandoverResourceAllocation", pdu)
	}

	out, err := ParseHandoverRequest(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.SecurityContext.NextHopChainingCount != 1 || out.UEAggregateMaximumBitRate.DL != 1000000000 ||
		len(out.PDUSessionResourceSetupListHOReq) != 1 || out.PDUSessionResourceSetupListHOReq[0].PDUSessionID != 5 ||
		out.PDUSessionResourceSetupListHOReq[0].SNSSAI.SST != 1 ||
		len(out.AllowedNSSAI) != 1 || out.GUAMI.AMFSetID != 1 {
		t.Fatalf("round trip %+v", out)
	}

	if _, err := ParsePDUSessionResourceSetupRequestTransfer(out.PDUSessionResourceSetupListHOReq[0].Transfer); err != nil {
		t.Fatalf("nested transfer: %v", err)
	}
}

const goldenHandoverRequestAcknowledge = "200d0035000005000a40020001005540020002003540110000050d0007c0c0a8" +
	"010200000002000100384006000006020038006a000302ccdd"

func TestHandoverRequestAcknowledgeGolden(t *testing.T) {
	admitted, err := (&HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: UPTransportLayerInformation{GTPTunnel: GTPTunnel{
			TransportLayerAddress: TransportLayerAddress{192, 168, 1, 2}, GTPTEID: 2,
		}},
		QosFlowSetupResponse: QosFlowListWithDataForwarding{{QosFlowIdentifier: 1}},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	refused, err := (&HandoverResourceAllocationUnsuccessfulTransfer{
		Cause: Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHOFailureInTarget},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	amfID, ranID := AMFUENGAPID(1), RANUENGAPID(2)

	in := HandoverRequestAcknowledge{
		AMFUENGAPID: &amfID, RANUENGAPID: &ranID,
		PDUSessionResourceAdmittedList:     PDUSessionResourceAdmittedList{{PDUSessionID: 5, Transfer: admitted}},
		PDUSessionResourceFailedToSetup:    PDUSessionResourceFailedToSetupListHOAck{{PDUSessionID: 6, Transfer: refused}},
		TargetToSourceTransparentContainer: TargetToSourceTransparentContainer{0xcc, 0xdd},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenHandoverRequestAcknowledge {
		t.Fatalf("encoded %s, want %s", got, goldenHandoverRequestAcknowledge)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcHandoverResourceAllocation {
		t.Fatalf("decoded %T, want a successful HandoverResourceAllocation outcome", pdu)
	}

	out, err := ParseHandoverRequestAcknowledge(so.Value)
	if err != nil {
		t.Fatal(err)
	}

	if len(out.PDUSessionResourceAdmittedList) != 1 || out.PDUSessionResourceAdmittedList[0].PDUSessionID != 5 ||
		len(out.PDUSessionResourceFailedToSetup) != 1 || out.PDUSessionResourceFailedToSetup[0].PDUSessionID != 6 ||
		!bytes.Equal(out.TargetToSourceTransparentContainer, []byte{0xcc, 0xdd}) {
		t.Fatalf("round trip %+v", out)
	}

	unsuccessful, err := ParseHandoverResourceAllocationUnsuccessfulTransfer(out.PDUSessionResourceFailedToSetup[0].Transfer)
	if err != nil {
		t.Fatal(err)
	}

	if unsuccessful.Cause.Group != CauseGroupRadioNetwork || unsuccessful.Cause.Value != CauseRadioNetworkHOFailureInTarget {
		t.Errorf("nested cause %+v", unsuccessful.Cause)
	}
}

const goldenHandoverFailure = "400d0016000003000a40020001000f400201c00106400302eeff"

func TestHandoverFailureGolden(t *testing.T) {
	amfID := AMFUENGAPID(1)

	in := HandoverFailure{
		AMFUENGAPID: &amfID,
		Cause:       &Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHOFailureInTarget},
		TargettoSourceFailureTransparentContainer: TargettoSourceFailureTransparentContainer{0xee, 0xff},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenHandoverFailure {
		t.Fatalf("encoded %s, want %s", got, goldenHandoverFailure)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	uo, ok := pdu.(*UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ProcHandoverResourceAllocation {
		t.Fatalf("decoded %T, want an unsuccessful HandoverResourceAllocation outcome", pdu)
	}

	out, err := ParseHandoverFailure(uo.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 1 || out.Cause == nil ||
		out.Cause.Value != CauseRadioNetworkHOFailureInTarget ||
		!bytes.Equal(out.TargettoSourceFailureTransparentContainer, []byte{0xee, 0xff}) {
		t.Fatalf("round trip %+v", out)
	}
}

// Every IE of HANDOVER FAILURE is ignore criticality, including the mandatory
// ones, so §10.3.5 never rejects the procedure — the AMF sees the absence in
// diagnostics instead.
func TestHandoverFailureMissingMandatoryIEsAreIgnored(t *testing.T) {
	out, err := ParseHandoverFailure(container(t))
	if err != nil {
		t.Fatalf("an all-ignore message must not reject the procedure: %v", err)
	}

	if out.AMFUENGAPID != nil || out.Cause != nil {
		t.Errorf("decoded %+v, want every field absent", out)
	}

	ies := out.meta().diagnostics.IEs
	if len(ies) != 2 {
		t.Fatalf("diagnostics = %+v, want two missing entries", ies)
	}

	for _, ie := range ies {
		if ie.TypeOfError != TypeOfErrorMissing {
			t.Errorf("IE %v reported %v, want missing", ie.ID, ie.TypeOfError)
		}
	}
}

// The AMF must not send a HANDOVER REQUEST that omits an IE §9.2.3.4 makes
// mandatory: §9.1.1 binds the sender even where §10.3.5 lets a receiver carry on.
func TestHandoverRequestRefusesMissingMandatoryIEs(t *testing.T) {
	full := func() HandoverRequest {
		return HandoverRequest{
			AMFUENGAPID: 1, HandoverType: HandoverTypeIntra5GS,
			Cause:                     &Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHandoverDesirableForRadio},
			UEAggregateMaximumBitRate: UEAggregateMaximumBitRate{DL: 1, UL: 1},
			PDUSessionResourceSetupListHOReq: PDUSessionResourceSetupListHOReq{{
				PDUSessionID: 5, SNSSAI: SNSSAI{SST: 1}, Transfer: handoverSetupTransfer(t),
			}},
			AllowedNSSAI:                       AllowedNSSAI{{SNSSAI: SNSSAI{SST: 1}}},
			SourceToTargetTransparentContainer: SourceToTargetTransparentContainer{0xaa},
		}
	}

	complete := full()
	if _, err := complete.Marshal(); err != nil {
		t.Fatalf("the complete message must encode: %v", err)
	}

	for _, tt := range []struct {
		name string
		omit func(*HandoverRequest)
	}{
		{"PDUSessionResourceSetupListHOReq", func(m *HandoverRequest) { m.PDUSessionResourceSetupListHOReq = nil }},
		{"AllowedNSSAI", func(m *HandoverRequest) { m.AllowedNSSAI = nil }},
		{"SourceToTargetTransparentContainer", func(m *HandoverRequest) { m.SourceToTargetTransparentContainer = nil }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := full()
			tt.omit(&m)

			if _, err := m.Marshal(); err == nil {
				t.Errorf("encoded a HANDOVER REQUEST with no %s", tt.name)
			}
		})
	}
}

// TestHandoverRequestCarriesNASC covers the two IEs an EPS to 5GS handover adds
// to HANDOVER REQUEST (TS 38.413 §9.2.3.4). The NASC is the S1 mode to N1 mode
// NAS transparent container of TS 24.501 §9.11.2.9, relayed to the UE inside the
// target's Target-to-Source container; without it the UE cannot take the mapped
// 5G security context into use.
func TestHandoverRequestCarriesNASC(t *testing.T) {
	nasc := NASPDU{0x11, 0x22, 0x33, 0x44, 0x21, 0x5b, 0x00, 0x00}

	in := HandoverRequest{
		AMFUENGAPID: 1, HandoverType: HandoverTypeEPSToFiveGS,
		Cause:                     &Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHandoverDesirableForRadio},
		UEAggregateMaximumBitRate: UEAggregateMaximumBitRate{DL: 1000000000, UL: 1000000000},
		UESecurityCapabilities:    UESecurityCapabilities{NREncryptionAlgorithms: 0x8000, NRIntegrityProtectionAlgorithms: 0x8000},
		SecurityContext:           SecurityContext{NextHopChainingCount: 0},
		NewSecurityContextInd:     Ptr(NewSecurityContextIndTrue),
		NASC:                      nasc,
		PDUSessionResourceSetupListHOReq: PDUSessionResourceSetupListHOReq{{
			PDUSessionID: 5, SNSSAI: SNSSAI{SST: 1}, Transfer: handoverSetupTransfer(t),
		}},
		AllowedNSSAI:                       AllowedNSSAI{{SNSSAI: SNSSAI{SST: 1}}},
		SourceToTargetTransparentContainer: SourceToTargetTransparentContainer{0xaa, 0xbb},
		GUAMI:                              GUAMI{PLMNIdentity: PLMNIdentity{0x02, 0xf8, 0x39}, AMFRegionID: 1, AMFSetID: 1},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcHandoverResourceAllocation {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseHandoverRequest(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.HandoverType != HandoverTypeEPSToFiveGS {
		t.Errorf("handover type = %d, want eps-to-5gs", out.HandoverType)
	}

	if out.NewSecurityContextInd == nil || *out.NewSecurityContextInd != NewSecurityContextIndTrue {
		t.Errorf("new security context indicator = %v, want true", out.NewSecurityContextInd)
	}

	if !bytes.Equal(out.NASC, nasc) {
		t.Errorf("NASC = % x, want % x", out.NASC, nasc)
	}

	// Both are optional, and an intra-5GS handover carries neither.
	in.NewSecurityContextInd, in.NASC, in.HandoverType = nil, nil, HandoverTypeIntra5GS

	b, err = in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err = Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err = ParseHandoverRequest(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.NewSecurityContextInd != nil || out.NASC != nil {
		t.Errorf("intra-5GS request carried %v / % x, want neither", out.NewSecurityContextInd, out.NASC)
	}
}
