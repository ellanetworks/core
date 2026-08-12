// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"errors"
	"testing"
)

// Golden PDU SESSION RESOURCE SETUP PDUs.
const (
	goldenPDUSessionResourceSetupRequest  = "001d0030000005000a0002000100550002000200260003027e00004a0008000005002002aabb006e400a0c3b9aca00301dcd6500"
	goldenPDUSessionResourceSetupResponse = "201d0022000004000a40020001005540020002004b400600000502ccdd003a400500000901ee"
)

func goldPDUSessionResourceSetupRequest() *PDUSessionResourceSetupRequest {
	return &PDUSessionResourceSetupRequest{
		AMFUENGAPID: 1,
		RANUENGAPID: 2,
		NASPDU:      Ptr(NASPDU{0x7e, 0x00}),
		PDUSessionResourceSetup: PDUSessionResourceSetupListSUReq{{
			PDUSessionID: 5,
			SNSSAI:       SNSSAI{SST: 1},
			Transfer:     TransferContainer{0xaa, 0xbb},
		}},
		UEAggregateMaximumBitRate: &UEAggregateMaximumBitRate{DL: 1000000000, UL: 500000000},
	}
}

func TestPDUSessionResourceSetupGoldenEncodings(t *testing.T) {
	tests := []struct {
		name string
		msg  interface{ Marshal() ([]byte, error) }
		want string
	}{
		{"Request", goldPDUSessionResourceSetupRequest(), goldenPDUSessionResourceSetupRequest},
		{
			"Response",
			&PDUSessionResourceSetupResponse{
				AMFUENGAPID: Ptr(AMFUENGAPID(1)),
				RANUENGAPID: Ptr(RANUENGAPID(2)),
				PDUSessionResourceSetup: PDUSessionResourceSetupListSURes{{
					PDUSessionID: 5, Transfer: TransferContainer{0xcc, 0xdd},
				}},
				PDUSessionResourceFailed: PDUSessionResourceFailedToSetupListSURes{{
					PDUSessionID: 9, Transfer: TransferContainer{0xee},
				}},
			},
			goldenPDUSessionResourceSetupResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mustMarshalHex(t, tt.msg); got != tt.want {
				t.Errorf("encoded\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestPDUSessionResourceSetupRoundTrips(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		in := goldPDUSessionResourceSetupRequest()

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		im, ok := pdu.(*InitiatingMessage)
		if !ok || im.ProcedureCode != ProcPDUSessionResourceSetup {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParsePDUSessionResourceSetupRequest(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.AMFUENGAPID != 1 || out.RANUENGAPID != 2 {
			t.Fatalf("ids = %d/%d", out.AMFUENGAPID, out.RANUENGAPID)
		}

		if len(out.PDUSessionResourceSetup) != 1 {
			t.Fatalf("session list = %+v", out.PDUSessionResourceSetup)
		}

		item := out.PDUSessionResourceSetup[0]
		if item.PDUSessionID != 5 || item.SNSSAI.SST != 1 || string(item.Transfer) != "\xaa\xbb" {
			t.Fatalf("session item = %+v", item)
		}

		if out.NASPDU == nil || string(*out.NASPDU) != "\x7e\x00" {
			t.Fatalf("nas pdu = %+v", out.NASPDU)
		}
	})

	t.Run("Response", func(t *testing.T) {
		plmn := PLMNIdentity{0x00, 0xf1, 0x10}
		in := &PDUSessionResourceSetupResponse{
			AMFUENGAPID: Ptr(AMFUENGAPID(7)),
			RANUENGAPID: Ptr(RANUENGAPID(9)),
			PDUSessionResourceSetup: PDUSessionResourceSetupListSURes{
				{PDUSessionID: 1, Transfer: TransferContainer{0x01}},
			},
			UserLocationInformation: &UserLocationInformation{
				Kind: UserLocationNR, PLMNIdentity: plmn, CellIdentity: 0x123456789,
				TAI: TAI{PLMNIdentity: plmn, TAC: 1},
			},
		}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		so, ok := pdu.(*SuccessfulOutcome)
		if !ok || so.ProcedureCode != ProcPDUSessionResourceSetup {
			t.Fatalf("got %T", pdu)
		}

		out, err := ParsePDUSessionResourceSetupResponse(so.Value)
		if err != nil || deref(out.AMFUENGAPID) != 7 || deref(out.RANUENGAPID) != 9 {
			t.Fatalf("got %+v err %v", out, err)
		}

		if out.UserLocationInformation == nil || out.UserLocationInformation.CellIdentity != 0x123456789 {
			t.Fatalf("ULI = %+v", out.UserLocationInformation)
		}
	})
}

// The session list is mandatory and reject criticality: a request without it
// sets up nothing, so it is not delivered (§10.3.5).
func TestPDUSessionResourceSetupRequestMissingSessionList(t *testing.T) {
	value := container(t,
		ieField{id: IDAMFUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(AMFUENGAPID(1)))},
		ieField{id: IDRANUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(RANUENGAPID(2)))},
	)

	_, err := ParsePDUSessionResourceSetupRequest(value)
	if err == nil {
		t.Fatal("parse succeeded without the session list")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != IDPDUSessionResourceSetupListSUReq ||
		ase.IEs[0].TypeOfError != TypeOfErrorMissing {
		t.Errorf("diagnostics = %+v, want one missing entry for the session list", ase.IEs)
	}
}

// The encoder refuses a request whose mandatory session list is unset, so an
// invalid PDU never reaches the NG-RAN node.
func TestPDUSessionResourceSetupRequestRejectsEmptySessionList(t *testing.T) {
	m := goldPDUSessionResourceSetupRequest()
	m.PDUSessionResourceSetup = nil

	if _, err := m.Marshal(); err == nil {
		t.Fatal("encoded a request with no PDU session")
	}
}

// Golden PDU SESSION RESOURCE SETUP UNSUCCESSFUL TRANSFER (TS 38.413 §9.3.4.16).
const goldenSetupUnsuccessfulTransfer = "0000"

func TestPDUSessionResourceSetupUnsuccessfulTransferRoundTrip(t *testing.T) {
	in := &PDUSessionResourceSetupUnsuccessfulTransfer{
		Cause: Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenSetupUnsuccessfulTransfer {
		t.Errorf("encoded %s, want %s", got, goldenSetupUnsuccessfulTransfer)
	}

	out, err := ParsePDUSessionResourceSetupUnsuccessfulTransfer(b)
	if err != nil {
		t.Fatal(err)
	}

	if out.Cause != in.Cause {
		t.Fatalf("cause = %+v, want %+v", out.Cause, in.Cause)
	}
}

// The transfer travels inside an OCTET STRING, so a truncated one must be
// refused rather than yielding a zero Cause the AMF would act on.
func TestPDUSessionResourceSetupUnsuccessfulTransferRejectsTruncated(t *testing.T) {
	if _, err := ParsePDUSessionResourceSetupUnsuccessfulTransfer(TransferContainer{}); err == nil {
		t.Fatal("parsed an empty transfer")
	}
}

// The request transfer is an IE container, so its golden carries IE ids and
// criticality: 0x8b=139 UL-NGU-UP-TNLInformation, 0x86=134 PDUSessionType,
// 0x88=136 QosFlowSetupRequestList, each reject (criticality 0). The response
// transfer is a plain SEQUENCE.
const (
	goldenSetupRequestTransferMin  = "000003008b000a01f0c0a801010000000100860001000088000700010000090000"
	goldenSetupResponseTransferMin = "0003e0c0a80101000000010003"
	// Two optionals present: securityResult and qosFlowFailedToSetupList.
	goldenSetupResponseTransferFull = "3003e0c0a8010100000001000304001000"
)

func setupTransferTunnel() UPTransportLayerInformation {
	return UPTransportLayerInformation{GTPTunnel: GTPTunnel{
		TransportLayerAddress: TransportLayerAddress{192, 168, 1, 1},
		GTPTEID:               1,
	}}
}

func TestPDUSessionResourceSetupRequestTransferGolden(t *testing.T) {
	in := PDUSessionResourceSetupRequestTransfer{
		ULNGUUPTNLInformation: setupTransferTunnel(),
		PDUSessionType:        PDUSessionTypeIPv4,
		QosFlowSetupRequest: QosFlowSetupRequestList{{
			QosFlowIdentifier: 1,
			QosFlowLevelQosParameters: QosFlowLevelQosParameters{
				QosCharacteristics: QosCharacteristics{
					Kind: QosCharacteristicsNonDynamic5QI, NonDynamic5QI: NonDynamic5QIDescriptor{FiveQI: 9},
				},
				AllocationAndRetentionPriority: AllocationAndRetentionPriority{PriorityLevelARP: 1},
			},
		}},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenSetupRequestTransferMin {
		t.Fatalf("encoded %s, want %s", got, goldenSetupRequestTransferMin)
	}

	out, err := ParsePDUSessionResourceSetupRequestTransfer(b)
	if err != nil {
		t.Fatal(err)
	}

	if out.PDUSessionType != PDUSessionTypeIPv4 {
		t.Errorf("PDUSessionType = %d, want %d", out.PDUSessionType, PDUSessionTypeIPv4)
	}

	if len(out.QosFlowSetupRequest) != 1 || out.QosFlowSetupRequest[0].QosFlowIdentifier != 1 {
		t.Errorf("QosFlowSetupRequest = %+v, want one flow with identifier 1", out.QosFlowSetupRequest)
	}

	ipv4, _ := out.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.IPs()
	if ipv4.String() != "192.168.1.1" || out.ULNGUUPTNLInformation.GTPTunnel.GTPTEID != 1 {
		t.Errorf("tunnel = %s/%d, want 192.168.1.1/1", ipv4, out.ULNGUUPTNLInformation.GTPTunnel.GTPTEID)
	}
}

// UL-NGU-UP-TNLInformation, PDUSessionType and QosFlowSetupRequestList are all
// mandatory with reject criticality, so a transfer missing one is rejected
// rather than delivered half-built (§10.3.5).
func TestPDUSessionResourceSetupRequestTransferRequiresMandatoryIEs(t *testing.T) {
	in := PDUSessionResourceSetupRequestTransfer{
		ULNGUUPTNLInformation: setupTransferTunnel(),
		PDUSessionType:        PDUSessionTypeIPv4,
	}

	if _, err := in.Marshal(); err == nil {
		t.Error("encoded a request transfer with no QosFlowSetupRequestList")
	}
}

func TestPDUSessionResourceSetupResponseTransferGolden(t *testing.T) {
	tnl := QosFlowPerTNLInformation{
		UPTransportLayerInformation: setupTransferTunnel(),
		AssociatedQosFlowList:       AssociatedQosFlowList{{QosFlowIdentifier: 3}},
	}

	integrity := IntegrityProtectionPerformed
	confidentiality := ConfidentialityProtectionNotPerformed

	for _, c := range []struct {
		name   string
		in     PDUSessionResourceSetupResponseTransfer
		golden string
	}{
		{"Min", PDUSessionResourceSetupResponseTransfer{
			DLQosFlowPerTNLInformation: tnl,
		}, goldenSetupResponseTransferMin},
		{"Full", PDUSessionResourceSetupResponseTransfer{
			DLQosFlowPerTNLInformation: tnl,
			SecurityResult: &SecurityResult{
				IntegrityProtectionResult:       integrity,
				ConfidentialityProtectionResult: confidentiality,
			},
			QosFlowFailedToSetup: QosFlowListWithCause{{
				QosFlowIdentifier: 2,
				Cause:             Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified},
			}},
		}, goldenSetupResponseTransferFull},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := c.in.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			if got := hex.EncodeToString(b); got != c.golden {
				t.Fatalf("encoded %s, want %s", got, c.golden)
			}

			out, err := ParsePDUSessionResourceSetupResponseTransfer(b)
			if err != nil {
				t.Fatal(err)
			}

			if len(out.DLQosFlowPerTNLInformation.AssociatedQosFlowList) != 1 ||
				out.DLQosFlowPerTNLInformation.AssociatedQosFlowList[0].QosFlowIdentifier != 3 {
				t.Errorf("AssociatedQosFlowList = %+v, want one flow with identifier 3",
					out.DLQosFlowPerTNLInformation.AssociatedQosFlowList)
			}

			if (c.in.SecurityResult == nil) != (out.SecurityResult == nil) {
				t.Errorf("SecurityResult presence not round-tripped")
			}

			if out.SecurityResult != nil && out.SecurityResult.ConfidentialityProtectionResult != confidentiality {
				t.Errorf("ConfidentialityProtectionResult = %d, want %d",
					out.SecurityResult.ConfidentialityProtectionResult, confidentiality)
			}

			if len(out.QosFlowFailedToSetup) != len(c.in.QosFlowFailedToSetup) {
				t.Errorf("QosFlowFailedToSetup length %d, want %d",
					len(out.QosFlowFailedToSetup), len(c.in.QosFlowFailedToSetup))
			}
		})
	}
}

// UL-NGU-UP-TNLInformation is mandatory with reject criticality, and its Go
// type is a struct that is never nil, so the encoder cannot catch its absence.
// Only the decode path can, and §10.3.5 requires the procedure be rejected with
// the missing IE named in Criticality Diagnostics.
func TestPDUSessionResourceSetupRequestTransferMissingTunnel(t *testing.T) {
	value := container(t,
		ieField{id: IDPDUSessionType, crit: CriticalityReject, raw: ieRaw(t, Ptr(PDUSessionTypeIPv4))},
		ieField{id: IDQosFlowSetupRequestList, crit: CriticalityReject, raw: ieRaw(t, QosFlowSetupRequestList{{
			QosFlowIdentifier: 1,
			QosFlowLevelQosParameters: QosFlowLevelQosParameters{
				QosCharacteristics: QosCharacteristics{
					Kind: QosCharacteristicsNonDynamic5QI, NonDynamic5QI: NonDynamic5QIDescriptor{FiveQI: 9},
				},
				AllocationAndRetentionPriority: AllocationAndRetentionPriority{PriorityLevelARP: 1},
			},
		}})},
	)

	_, err := ParsePDUSessionResourceSetupRequestTransfer(value)
	if err == nil {
		t.Fatal("parsed a request transfer with no UL-NGU-UP-TNLInformation")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != IDULNGUUPTNLInformation ||
		ase.IEs[0].TypeOfError != TypeOfErrorMissing {
		t.Errorf("diagnostics = %+v, want one missing entry for UL-NGU-UP-TNLInformation", ase.IEs)
	}
}
