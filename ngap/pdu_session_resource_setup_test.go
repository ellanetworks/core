// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"testing"
)

// Golden PDU SESSION RESOURCE SETUP PDUs. free5gc/ngap v1.1.3 and pycrate's
// NGAP module, two independent implementations, encode these identically.
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
		ieField{id: idAMFUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(AMFUENGAPID(1)))},
		ieField{id: idRANUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(RANUENGAPID(2)))},
	)

	_, err := ParsePDUSessionResourceSetupRequest(value)
	if err == nil {
		t.Fatal("parse succeeded without the session list")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != idPDUSessionResourceSetupListSUReq ||
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
