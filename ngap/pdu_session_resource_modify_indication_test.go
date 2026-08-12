// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"errors"
	"testing"
)

const (
	goldenPDUSessionResourceModifyIndication = "001b0018000003000a00020001005500020002003f000500000501aa"
	goldenPDUSessionResourceModifyConfirm    = "201b0021000004000a40020001005540020002003e400500000501bb0083400500000901cc"
)

func goldPDUSessionResourceModifyIndication() *PDUSessionResourceModifyIndication {
	return &PDUSessionResourceModifyIndication{
		AMFUENGAPID:              1,
		RANUENGAPID:              2,
		PDUSessionResourceModify: PDUSessionResourceModifyListModInd{{PDUSessionID: 5, Transfer: TransferContainer{0xaa}}},
	}
}

func TestPDUSessionResourceModifyIndicationGoldenEncodings(t *testing.T) {
	tests := []struct {
		name string
		msg  interface{ Marshal() ([]byte, error) }
		want string
	}{
		{"Indication", goldPDUSessionResourceModifyIndication(), goldenPDUSessionResourceModifyIndication},
		{
			"Confirm",
			&PDUSessionResourceModifyConfirm{
				AMFUENGAPID:              Ptr(AMFUENGAPID(1)),
				RANUENGAPID:              Ptr(RANUENGAPID(2)),
				PDUSessionResourceModify: PDUSessionResourceModifyListModCfm{{PDUSessionID: 5, Transfer: TransferContainer{0xbb}}},
				PDUSessionResourceFailed: PDUSessionResourceFailedToModifyListModCfm{{PDUSessionID: 9, Transfer: TransferContainer{0xcc}}},
			},
			goldenPDUSessionResourceModifyConfirm,
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

func TestPDUSessionResourceModifyIndicationRoundTrips(t *testing.T) {
	t.Run("Indication", func(t *testing.T) {
		plmn := PLMNIdentity{0x00, 0xf1, 0x10}
		in := goldPDUSessionResourceModifyIndication()
		in.UserLocationInformation = &UserLocationInformation{
			Kind: UserLocationNR, PLMNIdentity: plmn, CellIdentity: 0x123456789,
			TAI: TAI{PLMNIdentity: plmn, TAC: 1},
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
		if !ok || im.ProcedureCode != ProcPDUSessionResourceModifyIndication {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParsePDUSessionResourceModifyIndication(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		if len(out.PDUSessionResourceModify) != 1 || out.PDUSessionResourceModify[0].PDUSessionID != 5 {
			t.Fatalf("modify list = %+v", out.PDUSessionResourceModify)
		}

		if out.UserLocationInformation == nil || out.UserLocationInformation.CellIdentity != 0x123456789 {
			t.Fatalf("ULI = %+v", out.UserLocationInformation)
		}
	})

	t.Run("Confirm", func(t *testing.T) {
		in := &PDUSessionResourceModifyConfirm{
			AMFUENGAPID:              Ptr(AMFUENGAPID(7)),
			RANUENGAPID:              Ptr(RANUENGAPID(9)),
			PDUSessionResourceModify: PDUSessionResourceModifyListModCfm{{PDUSessionID: 1, Transfer: TransferContainer{0x01}}},
		}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		so, ok := pdu.(*SuccessfulOutcome)
		if !ok || so.ProcedureCode != ProcPDUSessionResourceModifyIndication {
			t.Fatalf("got %T", pdu)
		}

		out, err := ParsePDUSessionResourceModifyConfirm(so.Value)
		if err != nil || deref(out.AMFUENGAPID) != 7 || deref(out.RANUENGAPID) != 9 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}

// The modify list is mandatory and reject criticality: an indication without it
// modifies nothing, so it is not delivered (§10.3.5).
func TestPDUSessionResourceModifyIndicationMissingList(t *testing.T) {
	value := container(t,
		ieField{id: IDAMFUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(AMFUENGAPID(1)))},
		ieField{id: IDRANUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(RANUENGAPID(2)))},
	)

	_, err := ParsePDUSessionResourceModifyIndication(value)
	if err == nil {
		t.Fatal("parse succeeded without the modify list")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != IDPDUSessionResourceModifyListModInd ||
		ase.IEs[0].TypeOfError != TypeOfErrorMissing {
		t.Errorf("diagnostics = %+v, want one missing entry for the modify list", ase.IEs)
	}
}

// The transfer the AMF returns for a session it could not hand to the SMF
// round-trips, so a peer's copy decodes to the same cause.
func TestPDUSessionResourceModifyIndicationUnsuccessfulTransferRoundTrip(t *testing.T) {
	in := &PDUSessionResourceModifyIndicationUnsuccessfulTransfer{
		Cause: Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnknownPDUSessionID},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParsePDUSessionResourceModifyIndicationUnsuccessfulTransfer(b)
	if err != nil {
		t.Fatal(err)
	}

	if out.Cause != in.Cause {
		t.Fatalf("cause = %+v, want %+v", out.Cause, in.Cause)
	}
}

const (
	goldenModifyIndicationTransfer = "000f80c0a80101000000010003"
	goldenModifyConfirmTransferMin = "0000203ec0a8010100000001"
	// qosFlowFailedToModifyList present.
	goldenModifyConfirmTransferFull = "2000203ec0a801010000000100040000"
)

func modifyTransferTunnel() UPTransportLayerInformation {
	return UPTransportLayerInformation{GTPTunnel: GTPTunnel{
		TransportLayerAddress: TransportLayerAddress{192, 168, 1, 1},
		GTPTEID:               1,
	}}
}

func TestPDUSessionResourceModifyIndicationTransferGolden(t *testing.T) {
	in := PDUSessionResourceModifyIndicationTransfer{
		DLQosFlowPerTNLInformation: QosFlowPerTNLInformation{
			UPTransportLayerInformation: modifyTransferTunnel(),
			AssociatedQosFlowList:       AssociatedQosFlowList{{QosFlowIdentifier: 3}},
		},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenModifyIndicationTransfer {
		t.Fatalf("encoded %s, want %s", got, goldenModifyIndicationTransfer)
	}

	out, err := ParsePDUSessionResourceModifyIndicationTransfer(b)
	if err != nil {
		t.Fatal(err)
	}

	if len(out.DLQosFlowPerTNLInformation.AssociatedQosFlowList) != 1 ||
		out.DLQosFlowPerTNLInformation.AssociatedQosFlowList[0].QosFlowIdentifier != 3 {
		t.Errorf("AssociatedQosFlowList = %+v, want one flow with identifier 3",
			out.DLQosFlowPerTNLInformation.AssociatedQosFlowList)
	}

	if out.AdditionalDLQosFlowPerTNLInformation != nil {
		t.Errorf("AdditionalDLQosFlowPerTNLInformation = %+v, want nil", out.AdditionalDLQosFlowPerTNLInformation)
	}
}

func TestPDUSessionResourceModifyConfirmTransferGolden(t *testing.T) {
	failed := QosFlowListWithCause{{
		QosFlowIdentifier: 2,
		Cause:             Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified},
	}}

	for _, c := range []struct {
		name   string
		in     PDUSessionResourceModifyConfirmTransfer
		golden string
	}{
		{"Min", PDUSessionResourceModifyConfirmTransfer{
			QosFlowModifyConfirm:  QosFlowModifyConfirmList{{QosFlowIdentifier: 1}},
			ULNGUUPTNLInformation: modifyTransferTunnel(),
		}, goldenModifyConfirmTransferMin},
		{"Full", PDUSessionResourceModifyConfirmTransfer{
			QosFlowModifyConfirm:  QosFlowModifyConfirmList{{QosFlowIdentifier: 1}},
			ULNGUUPTNLInformation: modifyTransferTunnel(),
			QosFlowFailedToModify: failed,
		}, goldenModifyConfirmTransferFull},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := c.in.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			if got := hex.EncodeToString(b); got != c.golden {
				t.Fatalf("encoded %s, want %s", got, c.golden)
			}

			out, err := ParsePDUSessionResourceModifyConfirmTransfer(b)
			if err != nil {
				t.Fatal(err)
			}

			if len(out.QosFlowModifyConfirm) != 1 || out.QosFlowModifyConfirm[0].QosFlowIdentifier != 1 {
				t.Errorf("QosFlowModifyConfirm = %+v, want one flow with identifier 1", out.QosFlowModifyConfirm)
			}

			if len(out.QosFlowFailedToModify) != len(c.in.QosFlowFailedToModify) {
				t.Errorf("QosFlowFailedToModify length %d, want %d",
					len(out.QosFlowFailedToModify), len(c.in.QosFlowFailedToModify))
			}
		})
	}
}
