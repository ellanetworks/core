// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
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
