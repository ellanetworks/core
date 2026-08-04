// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"errors"
	"testing"
)

// Golden PDU SESSION RESOURCE MODIFY PDUs. free5gc/ngap v1.1.3 and pycrate's
// NGAP module, two independent implementations, encode these identically.
const (
	goldenPDUSessionResourceModifyRequest  = "001a001b000003000a0002000100550002000200400008004005027e0001aa"
	goldenPDUSessionResourceModifyResponse = "201a0021000004000a400200010055400200020041400500000501bb0036400500000901cc"
)

func goldPDUSessionResourceModifyRequest() *PDUSessionResourceModifyRequest {
	return &PDUSessionResourceModifyRequest{
		AMFUENGAPID: 1,
		RANUENGAPID: 2,
		PDUSessionResourceModify: PDUSessionResourceModifyListModReq{{
			PDUSessionID: 5,
			NASPDU:       Ptr(NASPDU{0x7e, 0x00}),
			Transfer:     TransferContainer{0xaa},
		}},
	}
}

func TestPDUSessionResourceModifyGoldenEncodings(t *testing.T) {
	tests := []struct {
		name string
		msg  interface{ Marshal() ([]byte, error) }
		want string
	}{
		{"Request", goldPDUSessionResourceModifyRequest(), goldenPDUSessionResourceModifyRequest},
		{
			"Response",
			&PDUSessionResourceModifyResponse{
				AMFUENGAPID:              Ptr(AMFUENGAPID(1)),
				RANUENGAPID:              Ptr(RANUENGAPID(2)),
				PDUSessionResourceModify: PDUSessionResourceModifyListModRes{{PDUSessionID: 5, Transfer: TransferContainer{0xbb}}},
				PDUSessionResourceFailed: PDUSessionResourceFailedToModifyListModRes{{PDUSessionID: 9, Transfer: TransferContainer{0xcc}}},
			},
			goldenPDUSessionResourceModifyResponse,
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

func TestPDUSessionResourceModifyRoundTrips(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		in := goldPDUSessionResourceModifyRequest()

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		im, ok := pdu.(*InitiatingMessage)
		if !ok || im.ProcedureCode != ProcPDUSessionResourceModify {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParsePDUSessionResourceModifyRequest(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		if len(out.PDUSessionResourceModify) != 1 {
			t.Fatalf("modify list = %+v", out.PDUSessionResourceModify)
		}

		item := out.PDUSessionResourceModify[0]
		if item.PDUSessionID != 5 || item.NASPDU == nil || string(*item.NASPDU) != "\x7e\x00" {
			t.Fatalf("item = %+v", item)
		}
	})

	t.Run("Response", func(t *testing.T) {
		plmn := PLMNIdentity{0x00, 0xf1, 0x10}
		in := &PDUSessionResourceModifyResponse{
			AMFUENGAPID:              Ptr(AMFUENGAPID(7)),
			RANUENGAPID:              Ptr(RANUENGAPID(9)),
			PDUSessionResourceModify: PDUSessionResourceModifyListModRes{{PDUSessionID: 1, Transfer: TransferContainer{0x01}}},
			UserLocationInformation: &UserLocationInformation{
				Kind: UserLocationNR, PLMNIdentity: plmn, CellIdentity: 0x123456789,
				TAI: TAI{PLMNIdentity: plmn, TAC: 1},
			},
		}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		so, ok := pdu.(*SuccessfulOutcome)
		if !ok || so.ProcedureCode != ProcPDUSessionResourceModify {
			t.Fatalf("got %T", pdu)
		}

		out, err := ParsePDUSessionResourceModifyResponse(so.Value)
		if err != nil || deref(out.AMFUENGAPID) != 7 {
			t.Fatalf("got %+v err %v", out, err)
		}

		if out.UserLocationInformation == nil || out.UserLocationInformation.CellIdentity != 0x123456789 {
			t.Fatalf("ULI = %+v", out.UserLocationInformation)
		}
	})
}

// The modify list is mandatory and reject criticality: a request without it
// modifies nothing, so it is not delivered (§10.3.5).
func TestPDUSessionResourceModifyRequestMissingList(t *testing.T) {
	value := container(t,
		ieField{id: idAMFUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(AMFUENGAPID(1)))},
		ieField{id: idRANUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(RANUENGAPID(2)))},
	)

	_, err := ParsePDUSessionResourceModifyRequest(value)
	if err == nil {
		t.Fatal("parse succeeded without the modify list")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != idPDUSessionResourceModifyListModReq ||
		ase.IEs[0].TypeOfError != TypeOfErrorMissing {
		t.Errorf("diagnostics = %+v, want one missing entry for the modify list", ase.IEs)
	}
}

// The modify request transfer is an IE container: 0x8c=140
// UL-NGU-UP-TNLModifyList, 0x87=135 QosFlowAddOrModifyRequestList, 0x89=137
// QosFlowToReleaseList, each reject criticality.
const (
	goldenModifyRequestTransferMin  = "0000010087000701008000090000"
	goldenModifyRequestTransferFull = "000003008c0014001fc0a801010000000101f00a0000010000000200870007010080000900000089000400040000"
)

func TestPDUSessionResourceModifyRequestTransferGolden(t *testing.T) {
	qp := QosFlowLevelQosParameters{
		QosCharacteristics: QosCharacteristics{
			Kind: QosCharacteristicsNonDynamic5QI, NonDynamic5QI: NonDynamic5QIDescriptor{FiveQI: 9},
		},
		AllocationAndRetentionPriority: AllocationAndRetentionPriority{PriorityLevelARP: 1},
	}

	addOrModify := QosFlowAddOrModifyRequestList{{QosFlowIdentifier: 1, QosFlowLevelQosParameters: &qp}}

	full := PDUSessionResourceModifyRequestTransfer{
		ULNGUUPTNLModify: ULNGUUPTNLModifyList{{
			ULNGUUPTNLInformation: UPTransportLayerInformation{GTPTunnel: GTPTunnel{
				TransportLayerAddress: TransportLayerAddress{192, 168, 1, 1}, GTPTEID: 1,
			}},
			DLNGUUPTNLInformation: UPTransportLayerInformation{GTPTunnel: GTPTunnel{
				TransportLayerAddress: TransportLayerAddress{10, 0, 0, 1}, GTPTEID: 2,
			}},
		}},
		QosFlowAddOrModifyRequest: addOrModify,
		QosFlowToRelease: QosFlowListWithCause{{
			QosFlowIdentifier: 2,
			Cause:             Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified},
		}},
	}

	for _, c := range []struct {
		name   string
		in     PDUSessionResourceModifyRequestTransfer
		golden string
	}{
		{"Min", PDUSessionResourceModifyRequestTransfer{QosFlowAddOrModifyRequest: addOrModify}, goldenModifyRequestTransferMin},
		{"Full", full, goldenModifyRequestTransferFull},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := c.in.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			if got := hex.EncodeToString(b); got != c.golden {
				t.Fatalf("encoded %s, want %s", got, c.golden)
			}

			out, err := ParsePDUSessionResourceModifyRequestTransfer(b)
			if err != nil {
				t.Fatal(err)
			}

			if len(out.QosFlowAddOrModifyRequest) != 1 || out.QosFlowAddOrModifyRequest[0].QosFlowIdentifier != 1 {
				t.Errorf("QosFlowAddOrModifyRequest = %+v, want one flow with identifier 1", out.QosFlowAddOrModifyRequest)
			}

			if len(out.ULNGUUPTNLModify) != len(c.in.ULNGUUPTNLModify) {
				t.Errorf("ULNGUUPTNLModify length %d, want %d", len(out.ULNGUUPTNLModify), len(c.in.ULNGUUPTNLModify))
			}

			if len(out.QosFlowToRelease) != len(c.in.QosFlowToRelease) {
				t.Errorf("QosFlowToRelease length %d, want %d", len(out.QosFlowToRelease), len(c.in.QosFlowToRelease))
			}
		})
	}
}

// Every IE is optional, so an empty transfer is well formed: a modify that
// changes nothing is still syntactically valid.
func TestPDUSessionResourceModifyRequestTransferEmpty(t *testing.T) {
	var in PDUSessionResourceModifyRequestTransfer

	b, err := in.Marshal()
	if err != nil {
		t.Fatalf("empty transfer did not encode: %v", err)
	}

	out, err := ParsePDUSessionResourceModifyRequestTransfer(b)
	if err != nil {
		t.Fatalf("empty transfer did not decode: %v", err)
	}

	if out.QosFlowAddOrModifyRequest != nil || out.ULNGUUPTNLModify != nil {
		t.Errorf("decoded %+v, want all fields absent", out)
	}
}

// QosFlowAddOrModifyRequestItem makes the QoS parameters optional where
// QosFlowSetupRequestItem has them mandatory, so a flow carrying only an
// identifier must round-trip.
func TestQosFlowAddOrModifyRequestItemWithoutQosParameters(t *testing.T) {
	in := PDUSessionResourceModifyRequestTransfer{
		QosFlowAddOrModifyRequest: QosFlowAddOrModifyRequestList{{QosFlowIdentifier: 7}},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParsePDUSessionResourceModifyRequestTransfer(b)
	if err != nil {
		t.Fatal(err)
	}

	if len(out.QosFlowAddOrModifyRequest) != 1 ||
		out.QosFlowAddOrModifyRequest[0].QosFlowIdentifier != 7 ||
		out.QosFlowAddOrModifyRequest[0].QosFlowLevelQosParameters != nil {
		t.Fatalf("round trip %+v, want one flow with identifier 7 and no QoS parameters", out.QosFlowAddOrModifyRequest)
	}
}

// ULNGUUPTNLModifyItem and UPTransportLayerInformationPairItem are
// structurally identical, so the only thing distinguishing their lists is the
// bound: maxnoofMultiConnectivity (4) against maxnoofMultiConnectivityMinusOne
// (3). Both are two-bit length determinants for short lists, so no golden
// separates them — only a list of exactly four does.
func TestULNGUUPTNLModifyListAcceptsFourLegs(t *testing.T) {
	item := ULNGUUPTNLModifyItem{
		ULNGUUPTNLInformation: UPTransportLayerInformation{GTPTunnel: GTPTunnel{
			TransportLayerAddress: TransportLayerAddress{192, 168, 1, 1}, GTPTEID: 1,
		}},
		DLNGUUPTNLInformation: UPTransportLayerInformation{GTPTunnel: GTPTunnel{
			TransportLayerAddress: TransportLayerAddress{10, 0, 0, 1}, GTPTEID: 2,
		}},
	}

	in := PDUSessionResourceModifyRequestTransfer{
		ULNGUUPTNLModify: ULNGUUPTNLModifyList{item, item, item, item},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatalf("four legs refused, bound is maxnoofMultiConnectivity (%d): %v", maxnoofMultiConnectivity, err)
	}

	out, err := ParsePDUSessionResourceModifyRequestTransfer(b)
	if err != nil {
		t.Fatal(err)
	}

	if len(out.ULNGUUPTNLModify) != 4 {
		t.Fatalf("decoded %d legs, want 4", len(out.ULNGUUPTNLModify))
	}

	over := in
	over.ULNGUUPTNLModify = append(ULNGUUPTNLModifyList{}, item, item, item, item, item)

	if _, err := over.Marshal(); err == nil {
		t.Errorf("encoded %d legs, bound is %d", len(over.ULNGUUPTNLModify), maxnoofMultiConnectivity)
	}
}
