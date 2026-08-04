// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"testing"
)

// Golden PDU SESSION RESOURCE RELEASE PDUs. free5gc/ngap v1.1.3 and pycrate's
// NGAP module, two independent implementations, encode these identically.
const (
	goldenPDUSessionResourceReleaseCommand  = "001c001f000004000a0002000100550002000200264003027e00004f000500000501aa"
	goldenPDUSessionResourceReleaseResponse = "201c0018000003000a400200010055400200020046400500000501bb"
)

func goldPDUSessionResourceReleaseCommand() *PDUSessionResourceReleaseCommand {
	return &PDUSessionResourceReleaseCommand{
		AMFUENGAPID:               1,
		RANUENGAPID:               2,
		NASPDU:                    Ptr(NASPDU{0x7e, 0x00}),
		PDUSessionResourceRelease: PDUSessionResourceToReleaseListRelCmd{{PDUSessionID: 5, Transfer: TransferContainer{0xaa}}},
	}
}

func TestPDUSessionResourceReleaseGoldenEncodings(t *testing.T) {
	tests := []struct {
		name string
		msg  interface{ Marshal() ([]byte, error) }
		want string
	}{
		{"Command", goldPDUSessionResourceReleaseCommand(), goldenPDUSessionResourceReleaseCommand},
		{
			"Response",
			&PDUSessionResourceReleaseResponse{
				AMFUENGAPID:                Ptr(AMFUENGAPID(1)),
				RANUENGAPID:                Ptr(RANUENGAPID(2)),
				PDUSessionResourceReleased: PDUSessionResourceReleasedListRelRes{{PDUSessionID: 5, Transfer: TransferContainer{0xbb}}},
			},
			goldenPDUSessionResourceReleaseResponse,
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

func TestPDUSessionResourceReleaseRoundTrips(t *testing.T) {
	t.Run("Command", func(t *testing.T) {
		in := goldPDUSessionResourceReleaseCommand()

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		im, ok := pdu.(*InitiatingMessage)
		if !ok || im.ProcedureCode != ProcPDUSessionResourceRelease {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParsePDUSessionResourceReleaseCommand(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		if len(out.PDUSessionResourceRelease) != 1 || out.PDUSessionResourceRelease[0].PDUSessionID != 5 {
			t.Fatalf("release list = %+v", out.PDUSessionResourceRelease)
		}
	})

	t.Run("Response", func(t *testing.T) {
		plmn := PLMNIdentity{0x00, 0xf1, 0x10}
		in := &PDUSessionResourceReleaseResponse{
			AMFUENGAPID:                Ptr(AMFUENGAPID(7)),
			RANUENGAPID:                Ptr(RANUENGAPID(9)),
			PDUSessionResourceReleased: PDUSessionResourceReleasedListRelRes{{PDUSessionID: 1, Transfer: TransferContainer{0x01}}},
			UserLocationInformation: &UserLocationInformation{
				Kind: UserLocationNR, PLMNIdentity: plmn, CellIdentity: 0x123456789,
				TAI: TAI{PLMNIdentity: plmn, TAC: 1},
			},
		}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		so, ok := pdu.(*SuccessfulOutcome)
		if !ok || so.ProcedureCode != ProcPDUSessionResourceRelease {
			t.Fatalf("got %T", pdu)
		}

		out, err := ParsePDUSessionResourceReleaseResponse(so.Value)
		if err != nil || deref(out.AMFUENGAPID) != 7 {
			t.Fatalf("got %+v err %v", out, err)
		}

		if out.UserLocationInformation == nil || out.UserLocationInformation.CellIdentity != 0x123456789 {
			t.Fatalf("ULI = %+v", out.UserLocationInformation)
		}
	})
}

// The release list is mandatory and reject criticality: a command without it
// releases nothing, so it is not delivered (§10.3.5).
func TestPDUSessionResourceReleaseCommandMissingList(t *testing.T) {
	value := container(t,
		ieField{id: idAMFUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(AMFUENGAPID(1)))},
		ieField{id: idRANUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(RANUENGAPID(2)))},
	)

	_, err := ParsePDUSessionResourceReleaseCommand(value)
	if err == nil {
		t.Fatal("parse succeeded without the release list")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != idPDUSessionResourceToReleaseListRelCmd ||
		ase.IEs[0].TypeOfError != TypeOfErrorMissing {
		t.Errorf("diagnostics = %+v, want one missing entry for the release list", ase.IEs)
	}
}

// The released list is mandatory but ignore criticality, so a response without
// it is still delivered and the absence reported (§10.3.5).
func TestPDUSessionResourceReleaseResponseMissingList(t *testing.T) {
	value := container(t,
		ieField{id: idAMFUENGAPID, crit: CriticalityIgnore, raw: ieRaw(t, Ptr(AMFUENGAPID(1)))},
		ieField{id: idRANUENGAPID, crit: CriticalityIgnore, raw: ieRaw(t, Ptr(RANUENGAPID(2)))},
	)

	msg, err := ParsePDUSessionResourceReleaseResponse(value)
	if err != nil {
		t.Fatalf("an absent ignore-criticality IE must still deliver the message: %v", err)
	}

	d := msg.Diagnostics()
	if len(d.IEs) != 1 || d.IEs[0].ID != idPDUSessionResourceReleasedListRelRes ||
		d.IEs[0].TypeOfError != TypeOfErrorMissing {
		t.Errorf("diagnostics = %+v, want one missing entry for the released list", d.IEs)
	}
}
