// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

// Golden UPLINK NAS TRANSPORT PDUs
const (
	goldenUplinkNASTransportNR     = "002e4029000004000a0002000100550002000100260003027e000079400f4000f110123456789000f110000001"
	goldenUplinkNASTransportNRTime = "002e4035000004000a000680ffffffffff00550005c0ffffffff00260004037e0102007940135000f110fffffffff000f11000000101020304"
	goldenUplinkNASTransportEUTRA  = "002e4027000004000a0002002a00550002000700260002017e0079400e0000f110abcde12000f110000001"
)

func goldULITAI() TAI {
	return TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}
}

func TestUplinkNASTransportRoundTrip(t *testing.T) {
	in := &UplinkNASTransport{
		AMFUENGAPID: 42,
		RANUENGAPID: 1,
		NASPDU:      NASPDU{0x7e, 0x00, 0x41, 0x01},
		UserLocationInformation: &UserLocationInformation{
			Kind: UserLocationNR, PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
			CellIdentity: 0x123456789, TAI: goldULITAI(),
		},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Initiating message, procedureCode 46 (TS 38.413 §9.2.5.3).
	if b[1] != 0x2e {
		t.Fatalf("procedureCode byte = %#x, want 0x2e", b[1])
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseUplinkNASTransport(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFUENGAPID != in.AMFUENGAPID || out.RANUENGAPID != in.RANUENGAPID ||
		deref(out.UserLocationInformation) != deref(in.UserLocationInformation) ||
		!bytes.Equal(out.NASPDU, in.NASPDU) {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

func TestUplinkNASTransportGolden(t *testing.T) {
	tests := []struct {
		name string
		msg  *UplinkNASTransport
		want string
	}{
		{
			"nr",
			&UplinkNASTransport{
				AMFUENGAPID: 1, RANUENGAPID: 1, NASPDU: NASPDU{0x7e, 0x00},
				UserLocationInformation: &UserLocationInformation{
					Kind: UserLocationNR, PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
					CellIdentity: 0x123456789, TAI: goldULITAI(),
				},
			},
			goldenUplinkNASTransportNR,
		},
		{
			// The widest of everything: 40-bit AMF id, 32-bit RAN id, the
			// all-ones 36-bit NR cell identity, and the optional timestamp.
			"nr with timestamp",
			&UplinkNASTransport{
				AMFUENGAPID: 0xffffffffff, RANUENGAPID: 0xffffffff, NASPDU: NASPDU{0x7e, 0x01, 0x02},
				UserLocationInformation: &UserLocationInformation{
					Kind: UserLocationNR, PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
					CellIdentity: 0xfffffffff, TAI: goldULITAI(),
					TimeStamp: &TimeStamp{0x01, 0x02, 0x03, 0x04},
				},
			},
			goldenUplinkNASTransportNRTime,
		},
		{
			// The E-UTRA alternative: a 28-bit cell identity, which is the only
			// width S1AP has.
			"eutra",
			&UplinkNASTransport{
				AMFUENGAPID: 42, RANUENGAPID: 7, NASPDU: NASPDU{0x7e},
				UserLocationInformation: &UserLocationInformation{
					Kind: UserLocationEUTRA, PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
					CellIdentity: 0xabcde12, TAI: goldULITAI(),
				},
			},
			goldenUplinkNASTransportEUTRA,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.msg.Marshal()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			if want := mustHex(t, tt.want); !bytes.Equal(got, want) {
				t.Fatalf("encode mismatch:\n  got  %x\n  want %x", got, want)
			}

			pdu, err := Unmarshal(mustHex(t, tt.want))
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			out, err := ParseUplinkNASTransport(pdu.value())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if out.AMFUENGAPID != tt.msg.AMFUENGAPID || out.RANUENGAPID != tt.msg.RANUENGAPID ||
				!bytes.Equal(out.NASPDU, tt.msg.NASPDU) {
				t.Fatalf("decode mismatch:\n  got  %+v\n  want %+v", out, tt.msg)
			}

			gotULI, wantULI := out.UserLocationInformation, tt.msg.UserLocationInformation
			if gotULI.Kind != wantULI.Kind || gotULI.CellIdentity != wantULI.CellIdentity ||
				gotULI.PLMNIdentity != wantULI.PLMNIdentity || gotULI.TAI != wantULI.TAI {
				t.Fatalf("location mismatch:\n  got  %+v\n  want %+v", gotULI, wantULI)
			}

			if deref(gotULI.TimeStamp) != deref(wantULI.TimeStamp) {
				t.Errorf("TimeStamp = %v, want %v", gotULI.TimeStamp, wantULI.TimeStamp)
			}
		})
	}
}

// The three UE-addressing IEs are mandatory-reject, so §10.3.5 stops the
// message; User Location Information is mandatory-ignore, so its absence is
// reported and the message still delivered.
func TestUplinkNASTransportMissingIEs(t *testing.T) {
	if _, err := (&UplinkNASTransport{}).Marshal(); err == nil {
		t.Error("Marshal() = nil error, want a required-IE error")
	}

	if _, err := ParseUplinkNASTransport(container(t,
		ieField{id: idAMFUENGAPID, crit: CriticalityReject, val: AMFUENGAPID(42)},
		ieField{id: idRANUENGAPID, crit: CriticalityReject, val: RANUENGAPID(7)},
	)); err == nil {
		t.Error("decoded a message with no NAS-PDU, want it rejected")
	}

	msg, err := ParseUplinkNASTransport(container(t,
		ieField{id: idAMFUENGAPID, crit: CriticalityReject, val: AMFUENGAPID(42)},
		ieField{id: idRANUENGAPID, crit: CriticalityReject, val: RANUENGAPID(7)},
		ieField{id: idNASPDU, crit: CriticalityReject, val: NASPDU{0x7e}},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.UserLocationInformation != nil {
		t.Errorf("absent location decoded to %+v, want nil", msg.UserLocationInformation)
	}

	var missing int

	for _, ie := range msg.Diagnostics().IEs {
		if ie.TypeOfError == TypeOfErrorMissing && ie.ID == idUserLocationInformation {
			missing++
		}
	}

	if missing != 1 {
		t.Errorf("reported %d missing IEs, want 1: %+v", missing, msg.Diagnostics().IEs)
	}
}

// TS 38.413 §9.3.1.16 closes UserLocationInformation with a choice-Extensions
// alternative, and its N3IWF alternative is not modeled. Either must be an
// explicit error, not a zero location that reads as one the peer reported.
func TestUserLocationInformationUnsupportedAlternatives(t *testing.T) {
	for _, alt := range []int64{userLocationInformationN3IWF, userLocationInformationChoiceExtensions} {
		w := per.NewWriter()
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, userLocationInformationAlternatives-1, alt); err != nil {
			t.Fatal(err)
		}

		w.AlignToByte()

		var uli UserLocationInformation

		err := uli.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned)
		if err == nil {
			t.Fatalf("alternative %d decoded to %+v, want an error", alt, uli)
		}

		if alt == userLocationInformationN3IWF && !errors.Is(err, errNotComprehended) {
			t.Errorf("alternative %d: err = %v, want errNotComprehended (§10.3.1 case 6)", alt, err)
		}

		if uli != (UserLocationInformation{}) {
			t.Errorf("alternative %d left a partial value %+v", alt, uli)
		}
	}
}

// Each kind's cell identity has its own width; a value too wide for the kind is
// an error rather than a silent truncation to another cell.
func TestUserLocationInformationCellIdentityWidth(t *testing.T) {
	tests := []struct {
		kind UserLocationInformationKind
		cell uint64
	}{
		{UserLocationEUTRA, 1 << eutraCellIdentityBits},
		{UserLocationNR, 1 << nrCellIdentityBits},
	}

	for _, tt := range tests {
		uli := UserLocationInformation{Kind: tt.kind, CellIdentity: tt.cell, TAI: goldULITAI()}
		if _, err := (&UplinkNASTransport{
			AMFUENGAPID: 1, RANUENGAPID: 1, NASPDU: NASPDU{0x7e}, UserLocationInformation: &uli,
		}).Marshal(); err == nil {
			t.Errorf("kind %d encoded cell identity %#x, want a width error", tt.kind, tt.cell)
		}
	}
}
