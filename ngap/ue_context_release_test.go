// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"errors"
	"testing"
)

// Golden UE CONTEXT RELEASE PDUs. free5gc/ngap v1.1.3 and pycrate's NGAP
// module, two independent implementations, encode these identically.
const (
	goldenUEContextReleaseRequest     = "002a401c000004000a0002000100550002000200850003000005000f40020500"
	goldenUEContextReleaseRequestMin  = "002a4015000003000a00020001005500020002000f40020500"
	goldenUEContextReleaseCommandPair = "002900100000020072000400010002000f400140"
	goldenUEContextReleaseCommandAMF  = "0029000e000002007200024001000f400140"
	goldenUEContextReleaseComplete    = "20290029000004000a400200010055400200020079400f4000f110123456789000f110000001003c0003000005"
	goldenUEContextReleaseCompleteMin = "2029000f000002000a40020001005540020002"
)

func mustMarshalHex(t *testing.T, m interface{ Marshal() ([]byte, error) }) string {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return hex.EncodeToString(b)
}

func TestUEContextReleaseGoldenEncodings(t *testing.T) {
	plmn := PLMNIdentity{0x00, 0xf1, 0x10}
	inactivity := Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUserInactivity}
	normalRelease := Cause{Group: CauseGroupNAS, Value: CauseNASNormalRelease}

	tests := []struct {
		name string
		msg  interface{ Marshal() ([]byte, error) }
		want string
	}{
		{
			"Request",
			&UEContextReleaseRequest{
				AMFUENGAPID:            1,
				RANUENGAPID:            2,
				PDUSessionResourceList: PDUSessionResourceListCxtRelReq{{PDUSessionID: 5}},
				Cause:                  &inactivity,
			},
			goldenUEContextReleaseRequest,
		},
		{
			"RequestWithoutSessions",
			&UEContextReleaseRequest{AMFUENGAPID: 1, RANUENGAPID: 2, Cause: &inactivity},
			goldenUEContextReleaseRequestMin,
		},
		{
			"CommandPair",
			&UEContextReleaseCommand{
				UENGAPIDs: UENGAPIDs{AMFUENGAPID: 1, RANUENGAPID: 2, Pair: true},
				Cause:     &normalRelease,
			},
			goldenUEContextReleaseCommandPair,
		},
		{
			"CommandAMFIDOnly",
			&UEContextReleaseCommand{UENGAPIDs: UENGAPIDs{AMFUENGAPID: 1}, Cause: &normalRelease},
			goldenUEContextReleaseCommandAMF,
		},
		{
			"Complete",
			&UEContextReleaseComplete{
				AMFUENGAPID: Ptr(AMFUENGAPID(1)),
				RANUENGAPID: Ptr(RANUENGAPID(2)),
				UserLocationInformation: &UserLocationInformation{
					Kind: UserLocationNR, PLMNIdentity: plmn, CellIdentity: 0x123456789,
					TAI: TAI{PLMNIdentity: plmn, TAC: 1},
				},
				PDUSessionResourceList: PDUSessionResourceListCxtRelCpl{{PDUSessionID: 5}},
			},
			goldenUEContextReleaseComplete,
		},
		{
			"CompleteMinimal",
			&UEContextReleaseComplete{AMFUENGAPID: Ptr(AMFUENGAPID(1)), RANUENGAPID: Ptr(RANUENGAPID(2))},
			goldenUEContextReleaseCompleteMin,
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

func TestUEContextReleaseRoundTrips(t *testing.T) {
	plmn := PLMNIdentity{0x00, 0xf1, 0x10}
	cause := Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUserInactivity}

	t.Run("Command pair", func(t *testing.T) {
		in := &UEContextReleaseCommand{
			UENGAPIDs: UENGAPIDs{AMFUENGAPID: 1, RANUENGAPID: 7, Pair: true},
			Cause:     &cause,
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
		if !ok || im.ProcedureCode != ProcUEContextRelease {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParseUEContextReleaseCommand(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		if !out.UENGAPIDs.Pair || out.UENGAPIDs.AMFUENGAPID != 1 || out.UENGAPIDs.RANUENGAPID != 7 ||
			deref(out.Cause) != cause {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("Command bare AMF id", func(t *testing.T) {
		in := &UEContextReleaseCommand{UENGAPIDs: UENGAPIDs{AMFUENGAPID: 42}, Cause: &cause}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		out, err := ParseUEContextReleaseCommand(pdu.(*InitiatingMessage).Value)
		if err != nil {
			t.Fatal(err)
		}

		if out.UENGAPIDs.Pair || out.UENGAPIDs.AMFUENGAPID != 42 || out.UENGAPIDs.RANUENGAPID != 0 {
			t.Fatalf("mismatch: %+v", out.UENGAPIDs)
		}
	})

	t.Run("Complete", func(t *testing.T) {
		in := &UEContextReleaseComplete{AMFUENGAPID: Ptr(AMFUENGAPID(1)), RANUENGAPID: Ptr(RANUENGAPID(7))}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		so, ok := pdu.(*SuccessfulOutcome)
		if !ok || so.ProcedureCode != ProcUEContextRelease {
			t.Fatalf("got %T", pdu)
		}

		out, err := ParseUEContextReleaseComplete(so.Value)
		if err != nil || deref(out.AMFUENGAPID) != 1 || deref(out.RANUENGAPID) != 7 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("CompleteWithUserLocation", func(t *testing.T) {
		in := &UEContextReleaseComplete{
			AMFUENGAPID: Ptr(AMFUENGAPID(1)),
			RANUENGAPID: Ptr(RANUENGAPID(7)),
			UserLocationInformation: &UserLocationInformation{
				Kind: UserLocationNR, PLMNIdentity: plmn, CellIdentity: 0x123456789,
				TAI: TAI{PLMNIdentity: plmn, TAC: 9},
			},
		}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		out, err := ParseUEContextReleaseComplete(pdu.(*SuccessfulOutcome).Value)
		if err != nil || out.UserLocationInformation == nil {
			t.Fatalf("got %+v err %v", out, err)
		}

		if out.UserLocationInformation.CellIdentity != 0x123456789 || out.UserLocationInformation.TAI.TAC != 9 {
			t.Fatalf("ULI mismatch: %+v", out.UserLocationInformation)
		}
	})

	// The list is reject criticality, so a Complete from a node with active
	// sessions must decode rather than be refused (§10.3.4.2).
	t.Run("CompleteWithSessions", func(t *testing.T) {
		in := &UEContextReleaseComplete{
			AMFUENGAPID:            Ptr(AMFUENGAPID(1)),
			RANUENGAPID:            Ptr(RANUENGAPID(7)),
			PDUSessionResourceList: PDUSessionResourceListCxtRelCpl{{PDUSessionID: 5}, {PDUSessionID: 9}},
		}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		out, err := ParseUEContextReleaseComplete(pdu.(*SuccessfulOutcome).Value)
		if err != nil {
			t.Fatal(err)
		}

		if len(out.PDUSessionResourceList) != 2 ||
			out.PDUSessionResourceList[0].PDUSessionID != 5 || out.PDUSessionResourceList[1].PDUSessionID != 9 {
			t.Fatalf("session list mismatch: %+v", out.PDUSessionResourceList)
		}
	})

	t.Run("Request", func(t *testing.T) {
		in := &UEContextReleaseRequest{
			AMFUENGAPID:            1,
			RANUENGAPID:            7,
			PDUSessionResourceList: PDUSessionResourceListCxtRelReq{{PDUSessionID: 3}},
			Cause:                  &cause,
		}

		b, _ := in.Marshal()

		pdu, _ := Unmarshal(b)

		im, ok := pdu.(*InitiatingMessage)
		if !ok || im.ProcedureCode != ProcUEContextReleaseRequest {
			t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
		}

		out, err := ParseUEContextReleaseRequest(im.Value)
		if err != nil || out.AMFUENGAPID != 1 || out.RANUENGAPID != 7 || deref(out.Cause) != cause {
			t.Fatalf("got %+v err %v", out, err)
		}

		if len(out.PDUSessionResourceList) != 1 || out.PDUSessionResourceList[0].PDUSessionID != 3 {
			t.Fatalf("session list mismatch: %+v", out.PDUSessionResourceList)
		}
	})
}

// The CHOICE is closed by a choice-Extensions alternative rather than an
// extension marker, so an unmodeled alternative is §10.3.1 case 6 and lands on
// the IE's criticality: reject, so no message is delivered.
func TestUEContextReleaseCommandChoiceExtension(t *testing.T) {
	value := container(t, ieField{
		id:   idUENGAPIDs,
		crit: CriticalityReject,
		raw:  choiceExtensionValue(t, ueNGAPIDsAlternatives, ueNGAPIDsChoiceExtensions, idUENGAPIDs),
	})

	_, err := ParseUEContextReleaseCommand(value)
	if err == nil {
		t.Fatal("parse succeeded, want a rejection")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
	}

	if ase.Cause.Value != CauseProtocolAbstractSyntaxErrorReject {
		t.Errorf("cause = %s, want abstract-syntax-error-reject", ase.Cause)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != idUENGAPIDs ||
		ase.IEs[0].TypeOfError != TypeOfErrorNotUnderstood {
		t.Errorf("diagnostics = %+v, want one not-understood entry for UE-NGAP-IDs", ase.IEs)
	}
}
