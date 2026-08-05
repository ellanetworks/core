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

const (
	goldenInitialUEMessage     = "000f402900000400550002000100260004037e00410079000f4000f110123456789000f110000001005a400118"
	goldenInitialUEMessageFull = "000f404300000700550005c0ffffffff00260005047e0041010079000f4000f110123456789000f110000001005a400148001a00070010c0deadbeef0003400200400070400100"
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
		!sameLocation(deref(out.UserLocationInformation), deref(in.UserLocationInformation)) ||
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
// alternative. Selecting it must be an explicit error, not a zero location that
// reads as one the peer reported.
func TestUserLocationInformationUnsupportedAlternatives(t *testing.T) {
	for _, alt := range []int64{userLocationInformationChoiceExtensions} {
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

		if !sameLocation(uli, UserLocationInformation{}) {
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
		{UserLocationEUTRA, 1 << EUTRACellIdentityBits},
		{UserLocationNR, 1 << NRCellIdentityBits},
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

func TestDownlinkNASTransportRoundTrip(t *testing.T) {
	in := &DownlinkNASTransport{AMFUENGAPID: 42, RANUENGAPID: 1, NASPDU: NASPDU{0x7e, 0x00, 0x41, 0x01}}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// DownlinkNASTransport is AMF-originated; an initiatingMessage with
	// procedureCode 4 (TS 38.413 §9.2.5.2).
	if b[1] != 0x04 {
		t.Fatalf("procedureCode byte = %#x, want 0x04", b[1])
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseDownlinkNASTransport(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFUENGAPID != in.AMFUENGAPID || out.RANUENGAPID != in.RANUENGAPID ||
		!bytes.Equal(out.NASPDU, in.NASPDU) {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

const (
	goldenDownlinkNASTransport        = "00044017000003000a0002000100550002000100260004037e0056"
	goldenDownlinkNASTransportWideIDs = "00044020000003000a000680ffffffffff00550005c0ffffffff00260006057e01020304"
)

func TestDownlinkNASTransportGolden(t *testing.T) {
	tests := []struct {
		name string
		msg  *DownlinkNASTransport
		want string
	}{
		{
			"minimal",
			&DownlinkNASTransport{AMFUENGAPID: 1, RANUENGAPID: 1, NASPDU: NASPDU{0x7e, 0x00, 0x56}},
			goldenDownlinkNASTransport,
		},
		{
			// The widest ids either field can carry: the AMF's is 40 bits, past
			// what a uint32 holds.
			"wide ids",
			&DownlinkNASTransport{
				AMFUENGAPID: 0xffffffffff, RANUENGAPID: 0xffffffff,
				NASPDU: NASPDU{0x7e, 0x01, 0x02, 0x03, 0x04},
			},
			goldenDownlinkNASTransportWideIDs,
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

			out, err := ParseDownlinkNASTransport(pdu.value())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if out.AMFUENGAPID != tt.msg.AMFUENGAPID || out.RANUENGAPID != tt.msg.RANUENGAPID ||
				!bytes.Equal(out.NASPDU, tt.msg.NASPDU) {
				t.Fatalf("decode mismatch:\n  got  %+v\n  want %+v", out, tt.msg)
			}
		})
	}
}

// Every modeled IE is mandatory-reject, so §9.1.1 refuses to encode any unset
// one and §10.3.5 stops an arriving message that omits one.
func TestDownlinkNASTransportMissingIEs(t *testing.T) {
	if _, err := (&DownlinkNASTransport{}).Marshal(); err == nil {
		t.Error("Marshal() = nil error, want a required-IE error")
	}

	if _, err := (&DownlinkNASTransport{AMFUENGAPID: 1, RANUENGAPID: 1}).Marshal(); err == nil {
		t.Error("Marshal() with no NAS-PDU = nil error, want a required-IE error")
	}

	if _, err := ParseDownlinkNASTransport(container(t,
		ieField{id: idAMFUENGAPID, crit: CriticalityReject, val: AMFUENGAPID(42)},
		ieField{id: idRANUENGAPID, crit: CriticalityReject, val: RANUENGAPID(7)},
	)); err == nil {
		t.Error("decoded a message with no NAS-PDU, want it rejected")
	}
}

// sameLocation compares two locations; UserLocationInformation holds a slice
// for the N3IWF alternative, so it is not comparable with ==.
func sameLocation(a, b UserLocationInformation) bool {
	return a.Kind == b.Kind && a.PLMNIdentity == b.PLMNIdentity &&
		a.CellIdentity == b.CellIdentity && a.TAI == b.TAI &&
		deref(a.TimeStamp) == deref(b.TimeStamp) &&
		bytes.Equal(a.IPAddress, b.IPAddress) && a.PortNumber == b.PortNumber
}

func goldInitialULI() UserLocationInformation {
	return UserLocationInformation{
		Kind: UserLocationNR, PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
		CellIdentity: 0x123456789, TAI: goldULITAI(),
	}
}

func TestInitialUEMessageGolden(t *testing.T) {
	tests := []struct {
		name string
		msg  *InitialUEMessage
		want string
	}{
		{
			"minimal",
			&InitialUEMessage{
				RANUENGAPID: 1, NASPDU: NASPDU{0x7e, 0x00, 0x41},
				UserLocationInformation: goldInitialULI(),
				RRCEstablishmentCause:   Ptr(RRCCauseMOSignalling),
			},
			goldenInitialUEMessage,
		},
		{
			// Every modeled IE, and the RRC cause is one of the five values
			// that exist only in NGAP's root.
			"every modeled IE",
			&InitialUEMessage{
				RANUENGAPID: 0xffffffff, NASPDU: NASPDU{0x7e, 0x00, 0x41, 0x01},
				UserLocationInformation: goldInitialULI(),
				RRCEstablishmentCause:   Ptr(RRCCauseMCSPriorityAccess),
				FiveGSTMSI:              &FiveGSTMSI{AMFSetID: 1, AMFPointer: 3, FiveGTMSI: 0xdeadbeef},
				AMFSetID:                Ptr(AMFSetID(1)),
				UEContextRequest:        Ptr(UEContextRequested),
			},
			goldenInitialUEMessageFull,
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

			out, err := ParseInitialUEMessage(pdu.value())
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if out.RANUENGAPID != tt.msg.RANUENGAPID || !bytes.Equal(out.NASPDU, tt.msg.NASPDU) ||
				!sameLocation(out.UserLocationInformation, tt.msg.UserLocationInformation) ||
				deref(out.RRCEstablishmentCause) != deref(tt.msg.RRCEstablishmentCause) ||
				deref(out.FiveGSTMSI) != deref(tt.msg.FiveGSTMSI) ||
				deref(out.AMFSetID) != deref(tt.msg.AMFSetID) ||
				deref(out.UEContextRequest) != deref(tt.msg.UEContextRequest) {
				t.Fatalf("decode mismatch:\n  got  %+v\n  want %+v", out, tt.msg)
			}
		})
	}
}

// User Location Information is mandatory-reject here but mandatory-ignore in
// UPLINK NAS TRANSPORT, so it is a value type in one message and nil-able in
// the other. §10.3.5 stops this message when it is absent.
func TestInitialUEMessageLocationIsRequired(t *testing.T) {
	if _, err := ParseInitialUEMessage(container(t,
		ieField{id: idRANUENGAPID, crit: CriticalityReject, val: RANUENGAPID(1)},
		ieField{id: idNASPDU, crit: CriticalityReject, val: NASPDU{0x7e}},
	)); err == nil {
		t.Error("decoded a message with no User Location Information, want it rejected")
	}

	// RRC Establishment Cause is mandatory-ignore, so its absence is reported
	// and the message still delivered.
	msg, err := ParseInitialUEMessage(container(t,
		ieField{id: idRANUENGAPID, crit: CriticalityReject, val: RANUENGAPID(1)},
		ieField{id: idNASPDU, crit: CriticalityReject, val: NASPDU{0x7e}},
		ieField{id: idUserLocationInformation, crit: CriticalityReject, val: goldInitialULI()},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.RRCEstablishmentCause != nil {
		t.Errorf("absent RRC cause decoded to %v, want nil", *msg.RRCEstablishmentCause)
	}

	var missing int

	for _, ie := range msg.Diagnostics().IEs {
		if ie.TypeOfError == TypeOfErrorMissing && ie.ID == idRRCEstablishmentCause {
			missing++
		}
	}

	if missing != 1 {
		t.Errorf("reported %d missing IEs, want 1: %+v", missing, msg.Diagnostics().IEs)
	}
}

// The five RRC establishment causes above S1AP's root are NGAP-only, and the
// extension additions past them must not alias onto a root value.
func TestRRCEstablishmentCauseNGAPOnlyValues(t *testing.T) {
	for _, c := range []RRCEstablishmentCause{RRCCauseMOVoiceCall, RRCCauseMOVideoCall, RRCCauseMOSMS, RRCCauseMPSPriorityAccess, RRCCauseMCSPriorityAccess} {
		w := per.NewWriter()
		if err := c.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("encode %d: %v", c, err)
		}

		w.AlignToByte()

		var got RRCEstablishmentCause
		if err := got.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned); err != nil {
			t.Fatalf("decode %d: %v", c, err)
		}

		if got != c {
			t.Errorf("round trip = %d, want %d", got, c)
		}
	}

	// notAvailable and mo-ExceptionData are extension additions.
	raw := extensionEnum(t, rrcEstablishmentCauseRootCount, 0)

	var got RRCEstablishmentCause
	if err := got.UnmarshalPER(per.NewReader(raw), per.Aligned); !errors.Is(err, errNotComprehended) {
		t.Errorf("extension addition: err = %v, want errNotComprehended", err)
	}
}

const (
	goldenInitialUEMessageN3IWF = "000f402100000400550002000100260003027e000079000880f8c0a801011f90005a400118"
	goldenInitialUEMessageNSSAI = "000f403100000500550002000100260003027e000079000f4000f110123456789000f110000001005a40011800000005020100007b"
)

// TS 38.413 §9.3.1.16's N3IWF alternative. Ella Core serves no non-3GPP access
// and does not act on the location, but the IE is mandatory-reject, so it must
// decode rather than reject the UE's first message.
func TestInitialUEMessageN3IWFLocation(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenInitialUEMessageN3IWF))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseInitialUEMessage(pdu.value())
	if err != nil {
		t.Fatalf("an N3IWF location was rejected: %v", err)
	}

	uli := msg.UserLocationInformation
	if uli.Kind != UserLocationN3IWF {
		t.Fatalf("Kind = %d, want UserLocationN3IWF", uli.Kind)
	}

	if !bytes.Equal(uli.IPAddress, []byte{0xc0, 0xa8, 0x01, 0x01}) {
		t.Errorf("IPAddress = %x, want c0a80101", uli.IPAddress)
	}

	if uli.PortNumber != 0x1f90 {
		t.Errorf("PortNumber = %#x, want 0x1f90", uli.PortNumber)
	}

	got, err := msg.Marshal()
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}

	if want := mustHex(t, goldenInitialUEMessageN3IWF); !bytes.Equal(got, want) {
		t.Fatalf("re-encode mismatch:\n  got  %x\n  want %x", got, want)
	}
}

// §8.6.1.2 has the NG-RAN node include the Allowed NSSAI it has stored for the
// UE. Ella Core derives the UE's slices from subscription data and does not act
// on this, but the IE is reject criticality, so a slicing-aware gNB that sends
// it must not have every registration refused.
func TestInitialUEMessageAllowedNSSAI(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenInitialUEMessageNSSAI))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseInitialUEMessage(pdu.value())
	if err != nil {
		t.Fatalf("an Allowed NSSAI was rejected: %v", err)
	}

	if len(msg.AllowedNSSAI) != 1 {
		t.Fatalf("AllowedNSSAI = %+v, want one item", msg.AllowedNSSAI)
	}

	if got := msg.AllowedNSSAI[0].SNSSAI; got.SST != 1 || deref(got.SD) != (SD{0x00, 0x00, 0x7b}) {
		t.Errorf("S-NSSAI = %+v, want SST 1 / SD 00007b", got)
	}

	got, err := msg.Marshal()
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}

	if want := mustHex(t, goldenInitialUEMessageNSSAI); !bytes.Equal(got, want) {
		t.Fatalf("re-encode mismatch:\n  got  %x\n  want %x", got, want)
	}
}
