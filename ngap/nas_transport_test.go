// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/per/ap/apimpl"
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
		AMFUENGAPID:             42,
		RANUENGAPID:             1,
		NASPDU:                  NASPDU{0x7e, 0x00, 0x41, 0x01},
		UserLocationInformation: &UserLocationInformation{NR: &UserLocationInformationNR{NRCGI: NRCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, NRCellIdentity: NRCellIdentity(0x123456789)}, TAI: goldULITAI()}},
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
		!deref(out.UserLocationInformation).Equal(deref(in.UserLocationInformation)) ||
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
				UserLocationInformation: &UserLocationInformation{NR: &UserLocationInformationNR{NRCGI: NRCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, NRCellIdentity: NRCellIdentity(0x123456789)}, TAI: goldULITAI()}},
			},
			goldenUplinkNASTransportNR,
		},
		{
			// The widest of everything: 40-bit AMF id, 32-bit RAN id, the
			// all-ones 36-bit NR cell identity, and the optional timestamp.
			"nr with timestamp",
			&UplinkNASTransport{
				AMFUENGAPID: 0xffffffffff, RANUENGAPID: 0xffffffff, NASPDU: NASPDU{0x7e, 0x01, 0x02},
				UserLocationInformation: &UserLocationInformation{NR: &UserLocationInformationNR{NRCGI: NRCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, NRCellIdentity: NRCellIdentity(0xfffffffff)}, TAI: goldULITAI(), TimeStamp: &TimeStamp{0x01, 0x02, 0x03, 0x04}}},
			},
			goldenUplinkNASTransportNRTime,
		},
		{
			// The E-UTRA alternative: a 28-bit cell identity, which is the only
			// width S1AP has.
			"eutra",
			&UplinkNASTransport{
				AMFUENGAPID: 42, RANUENGAPID: 7, NASPDU: NASPDU{0x7e},
				UserLocationInformation: &UserLocationInformation{EUTRA: &UserLocationInformationEUTRA{EUTRACGI: EUTRACGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, EUTRACellIdentity: EUTRACellIdentity(0xabcde12)}, TAI: goldULITAI()}},
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
			if !gotULI.Equal(*wantULI) {
				t.Fatalf("location mismatch:\n  got  %+v\n  want %+v", gotULI, wantULI)
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
		ieField{ID: IDAMFUENGAPID, Criticality: CriticalityReject, Value: AMFUENGAPID(42)},
		ieField{ID: IDRANUENGAPID, Criticality: CriticalityReject, Value: RANUENGAPID(7)},
	)); err == nil {
		t.Error("decoded a message with no NAS-PDU, want it rejected")
	}

	msg, err := ParseUplinkNASTransport(container(t,
		ieField{ID: IDAMFUENGAPID, Criticality: CriticalityReject, Value: AMFUENGAPID(42)},
		ieField{ID: IDRANUENGAPID, Criticality: CriticalityReject, Value: RANUENGAPID(7)},
		ieField{ID: IDNASPDU, Criticality: CriticalityReject, Value: NASPDU{0x7e}},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.UserLocationInformation != nil {
		t.Errorf("absent location decoded to %+v, want nil", msg.UserLocationInformation)
	}

	var missing int

	for _, ie := range msg.Diagnostics().IEs {
		if typeOfError(ie) == TypeOfErrorMissing && ie.ID == IDUserLocationInformation {
			missing++
		}
	}

	if missing != 1 {
		t.Errorf("reported %d missing IEs, want 1: %+v", missing, msg.Diagnostics().IEs)
	}
}

func TestUserLocationInformationN3IWFWithoutPortNumberIsTyped(t *testing.T) {
	tai := goldULITAI()
	in := UserLocationInformation{N3IWFWithoutPortNumber: &UserLocationInformationN3IWFWithoutPortNumber{
		IPAddress: tlaOf(10, 0, 0, 1), TAI: &tai,
	}}

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	inner, err := per.Marshal(in.N3IWFWithoutPortNumber)
	if err != nil {
		t.Fatal(err)
	}

	want := per.NewWriter()
	if err := per.EncodeConstrainedWholeNumber(want, per.Aligned, 0, 3, 3); err != nil {
		t.Fatal(err)
	}

	if err := encodeContainerField(want, ieField{ID: IDUserLocationInformationN3IWFwithoutPortNumber, Criticality: CriticalityIgnore, Value: apimpl.RawValue(inner)}); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(perBytes(w), perBytes(want)) {
		t.Fatalf("encoding = % x, want % x", perBytes(w), perBytes(want))
	}

	got, err := unmarshalPERValue[UserLocationInformation](perBytes(w))
	if err != nil {
		t.Fatal(err)
	}

	if !got.Equal(in) {
		t.Errorf("round trip = %+v, want %+v", got, in)
	}
}

func TestUserLocationInformationUnknownChoiceExtensionIsPreserved(t *testing.T) {
	for _, crit := range []Criticality{CriticalityReject, CriticalityIgnore} {
		w := per.NewWriter()
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 3, 3); err != nil {
			t.Fatal(err)
		}

		if err := encodeContainerField(w, ieField{ID: 9999, Criticality: crit, Value: apimpl.RawValue([]byte{0x00})}); err != nil {
			t.Fatal(err)
		}

		wire := perBytes(w)

		uli, err := unmarshalPERValue[UserLocationInformation](wire)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}

		if uli.ChoiceExtension == nil || uli.ChoiceExtension.ID != 9999 || uli.NR != nil || uli.EUTRA != nil {
			t.Fatalf("decoded %+v, want only the raw choice-Extensions", uli)
		}

		d := ap.Diagnose[ProtocolIEID](&uli)
		if len(d.IEs) != 1 || d.IEs[0].Criticality != crit {
			t.Errorf("diagnostics = %+v, want one entry on criticality %v", d.IEs, crit)
		}

		w2 := per.NewWriter()
		if err := uli.MarshalPER(w2, per.Aligned); err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(perBytes(w2), wire) {
			t.Errorf("re-encode = % x, want % x", perBytes(w2), wire)
		}
	}
}

func TestUserLocationInformationCellIdentityWidth(t *testing.T) {
	for _, uli := range []UserLocationInformation{
		{EUTRA: &UserLocationInformationEUTRA{EUTRACGI: EUTRACGI{EUTRACellIdentity: 1 << EUTRACellIdentityBits}, TAI: goldULITAI()}},
		{NR: &UserLocationInformationNR{NRCGI: NRCGI{NRCellIdentity: 1 << NRCellIdentityBits}, TAI: goldULITAI()}},
	} {
		if _, err := (&UplinkNASTransport{
			AMFUENGAPID: 1, RANUENGAPID: 1, NASPDU: NASPDU{0x7e}, UserLocationInformation: &uli,
		}).Marshal(); err == nil {
			t.Errorf("%+v encoded an over-wide cell identity, want a width error", uli)
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
		ieField{ID: IDAMFUENGAPID, Criticality: CriticalityReject, Value: AMFUENGAPID(42)},
		ieField{ID: IDRANUENGAPID, Criticality: CriticalityReject, Value: RANUENGAPID(7)},
	)); err == nil {
		t.Error("decoded a message with no NAS-PDU, want it rejected")
	}
}

func goldInitialULI() UserLocationInformation {
	return UserLocationInformation{NR: &UserLocationInformationNR{NRCGI: NRCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, NRCellIdentity: NRCellIdentity(0x123456789)}, TAI: goldULITAI()}}
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
				!out.UserLocationInformation.Equal(tt.msg.UserLocationInformation) ||
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
		ieField{ID: IDRANUENGAPID, Criticality: CriticalityReject, Value: RANUENGAPID(1)},
		ieField{ID: IDNASPDU, Criticality: CriticalityReject, Value: NASPDU{0x7e}},
	)); err == nil {
		t.Error("decoded a message with no User Location Information, want it rejected")
	}

	// RRC Establishment Cause is mandatory-ignore, so its absence is reported
	// and the message still delivered.
	msg, err := ParseInitialUEMessage(container(t,
		ieField{ID: IDRANUENGAPID, Criticality: CriticalityReject, Value: RANUENGAPID(1)},
		ieField{ID: IDNASPDU, Criticality: CriticalityReject, Value: NASPDU{0x7e}},
		ieField{ID: IDUserLocationInformation, Criticality: CriticalityReject, Value: Ptr(goldInitialULI())},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.RRCEstablishmentCause != nil {
		t.Errorf("absent RRC cause decoded to %v, want nil", *msg.RRCEstablishmentCause)
	}

	var missing int

	for _, ie := range msg.Diagnostics().IEs {
		if typeOfError(ie) == TypeOfErrorMissing && ie.ID == IDRRCEstablishmentCause {
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

	for k, want := range map[int64]RRCEstablishmentCause{0: RRCCauseNotAvailable, 1: RRCCauseMOExceptionData} {
		var got RRCEstablishmentCause
		if err := got.UnmarshalPER(per.NewReader(extensionEnum(t, int64(len(enumRRCEstablishmentCause.Root)), k)), per.Aligned); err != nil || got != want || got.Diagnose(nil) {
			t.Errorf("extension addition %d = %d, %v; want %s", k, got, err, want.Name())
		}
	}

	var got RRCEstablishmentCause
	if err := got.UnmarshalPER(per.NewReader(extensionEnum(t, int64(len(enumRRCEstablishmentCause.Root)), 2)), per.Aligned); err != nil || !got.Diagnose(nil) {
		t.Errorf("unknown extension addition = %d, %v; want it kept and reported", got, err)
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

	uli := msg.UserLocationInformation.N3IWF
	if uli == nil {
		t.Fatalf("location = %+v, want the N3IWF alternative", msg.UserLocationInformation)
	}

	if uli.IPAddress != tlaOf(0xc0, 0xa8, 0x01, 0x01) {
		t.Errorf("IPAddress = %v, want 192.168.1.1", uli.IPAddress)
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

func goldRegistrationRequestNAS() NASPDU {
	return NASPDU{
		0x7e, 0x00, 0x41, 0x79, 0x00, 0x0b, 0xf2, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x2e, 0x02, 0xf0, 0xf0, 0x17, 0x07, 0xf0, 0xf0, 0xc0, 0x40, 0x01, 0x80, 0x30,
	}
}

func goldInitialUEMessageFemto() *InitialUEMessage {
	return &InitialUEMessage{
		RANUENGAPID:             1,
		NASPDU:                  goldRegistrationRequestNAS(),
		UserLocationInformation: goldInitialULI(),
		RRCEstablishmentCause:   Ptr(RRCCauseMOSignalling),
		GUAMI:                   &GUAMI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, AMFRegionID: 0xca, AMFSetID: 0x3f8, AMFPointer: 0x01},
		GUAMIType:               Ptr(GUAMITypeMapped),
		RequestedNSSAI: RequestedNSSAI{
			{SNSSAI: SNSSAI{SST: 1}},
			{SNSSAI: SNSSAI{SST: 2, SD: &SD{0x00, 0x00, 0x01}}},
		},
	}
}

func initialUEMessageWireCases(t *testing.T) []wireCase {
	t.Helper()

	return []wireCase{
		{
			name:  "InitialUEMessageGUAMITypeRequestedNSSAI",
			pdu:   marshalT(t, goldInitialUEMessageFemto()),
			parse: func(v []byte) (any, error) { return ParseInitialUEMessage(v) },
			check: func(t *testing.T, a any) {
				t.Helper()

				if !equalByMethod(a, goldInitialUEMessageFemto()) {
					t.Fatalf("decoded %+v", a)
				}
			},
		},
		{
			name: "InitialUEMessageGUAMIAfterSelectedPLMNIdentity",
			pdu: insertRawIEs(t, marshalT(t, goldInitialUEMessageFemto()), 4,
				ieField{ID: IDSelectedPLMNIdentity, Criticality: CriticalityIgnore, Value: apimpl.RawValue([]byte{0x00, 0xf1, 0x10})}),
			parse: func(v []byte) (any, error) { return ParseInitialUEMessage(v) },
			check: func(t *testing.T, a any) {
				t.Helper()

				m := a.(*InitialUEMessage)
				if m.GUAMI == nil || deref(m.GUAMIType) != GUAMITypeMapped || len(m.RequestedNSSAI) != 2 ||
					deref(m.SelectedPLMNIdentity) != (PLMNIdentity{0x00, 0xf1, 0x10}) || len(m.UnknownIEs()) != 0 {
					t.Fatalf("decoded %+v", m)
				}
			},
		},
	}
}
