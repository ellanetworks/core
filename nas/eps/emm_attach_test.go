// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"encoding/hex"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func loadCapture(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := os.ReadFile("../testdata/captures/" + name)
	if err != nil {
		t.Fatalf("read capture: %v", err)
	}

	b, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}

	return b
}

// TestAttachRequestGolden decodes the real captured Attach Request: the outer
// integrity-protected wrapper (Phase 2) and the inner plain ATTACH REQUEST, and
// checks the message round-trips byte-for-byte.
func TestAttachRequestGolden(t *testing.T) {
	wrapped := loadCapture(t, "attach_request_nas.hex")

	sp, err := ParseSecurityProtectedMessage(wrapped)
	if err != nil {
		t.Fatalf("wrapper: %v", err)
	}

	if sp.SecurityHeaderType != SHTIntegrityProtected || sp.SequenceNumber != 0x05 {
		t.Fatalf("wrapper SHT=%d seq=%#x, want 1 / 0x05", sp.SecurityHeaderType, sp.SequenceNumber)
	}

	if mt, err := PeekMessageType(sp.UnverifiedPayload); err != nil || mt != MsgAttachRequest {
		t.Fatalf("PeekMessageType = %#x, %v; want 0x41", mt, err)
	}

	ar, err := ParseAttachRequest(sp.UnverifiedPayload)
	if err != nil {
		t.Fatalf("attach request: %v", err)
	}

	if ar.EPSAttachType != AttachTypeCombined || ar.NASKeySetIdentifier.Value != 0 {
		t.Fatalf("attach type=%d ksi=%d, want 2 / 0", ar.EPSAttachType, ar.NASKeySetIdentifier.Value)
	}

	guti := ar.EPSMobileIdentity.GUTI
	if guti == nil || guti.PLMN.MCC != "001" || guti.PLMN.MNC != "01" ||
		guti.MMEGroupID != 0x0002 || guti.MMECode != 0x01 || guti.TMSI != [4]byte{0x03, 0x00, 0x03, 0xe6} {
		t.Fatalf("GUTI mismatch: %+v", ar.EPSMobileIdentity)
	}

	if !ar.UENetworkCapability.HasUMTS || len(ar.UENetworkCapability.Rest) != 1 || len(ar.ESMMessageContainer) != 5 {
		t.Fatalf("UE network capability = %+v, ESM container %d octets",
			ar.UENetworkCapability, len(ar.ESMMessageContainer))
	}

	// The capture carries an MS network capability after a Last visited TAI and a
	// DRX parameter, so it exercises the optional-IE walk.
	if ar.MSNetworkCapability == nil || !bytes.Equal(ar.MSNetworkCapability.Rest, []byte{0xe5, 0xe0, 0x34}) {
		t.Fatalf("MSNetworkCapability = %+v", ar.MSNetworkCapability)
	}

	// The capture re-encodes byte-for-byte, so the codec loses nothing it decoded.
	again, err := ar.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if !bytes.Equal(again, sp.UnverifiedPayload) {
		t.Fatalf("re-encode of the capture differs\n got % x\nwant % x", again, sp.UnverifiedPayload)
	}
}

// TestAttachRequestMSNetworkCapability exercises the optional-IE walk through the
// public parser: extracting the MS network capability when it leads the optional
// part, when it sits behind the IEs the ATTACH REQUEST orders before it, when it
// is absent (only later IEs present, at which the walk stops), and when a
// malformed length makes the message unparseable.
func TestAttachRequestMSNetworkCapability(t *testing.T) {
	base := &AttachRequest{
		EPSAttachType:       AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 0},
		EPSMobileIdentity:   GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x00, 0x00, 0x00, 0x01}}),
		UENetworkCapability: UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: []byte{0x02, 0x01, 0xd0, 0x11},
	}

	prefix, err := base.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		opt     []byte
		want    []byte
		wantErr bool
	}{
		{"first", []byte{0x31, 0x03, 0xaa, 0xbb, 0xcc}, []byte{0xaa, 0xbb, 0xcc}, false},
		{
			"after TAI, DRX and additional GUTI",
			[]byte{
				0x52, 0, 0xf1, 0x10, 0x30, 0x39, 0x5c, 0x0a, 0x00,
				0x50, 0x0b, 0xf6, 0x00, 0xf1, 0x10, 0x00, 0x02, 0x01, 0x03, 0x00, 0x03, 0xe6,
				0x31, 0x02, 0x11, 0x22, 0x5d, 0x01, 0xe0,
			},
			[]byte{0x11, 0x22},
			false,
		},
		{"absent (only later IEs)", []byte{0x13, 0, 0xf1, 0x10, 0x00, 0x01}, nil, false},
		{"truncated length", []byte{0x31, 0x05, 0xaa, 0xbb}, nil, true},
		{"empty", nil, nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wire := append(append([]byte{}, prefix...), tc.opt...)

			out, err := ParseAttachRequest(wire)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseAttachRequest(%x) = nil error, want error", tc.opt)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			var got []byte
			if out.MSNetworkCapability != nil {
				got = out.MSNetworkCapability.Rest
			}

			if !bytes.Equal(got, tc.want) {
				t.Fatalf("MSNetworkCapability = %x, want %x", got, tc.want)
			}
		})
	}
}

// TestAttachRequestOptionalIEsRoundTrip drives the optional-IE walk through
// every format the ATTACH REQUEST uses, and checks that the elements Ella does
// not model are preserved rather than dropped: they arrive as unrecognized
// elements and the message re-encodes byte-for-byte.
func TestAttachRequestOptionalIEsRoundTrip(t *testing.T) {
	// The elements this message no longer names, in the order the ATTACH REQUEST
	// definition lists them (TS 24.301 §8.2.4).
	unmodelled := []nas.RawIE{
		{IEI: 0x19, Format: nas.IETV3, Value: []byte{0x01, 0x02, 0x03}},             // old P-TMSI signature
		{IEI: 0x13, Format: nas.IETV3, Value: []byte{0x00, 0xf1, 0x10, 0x00, 0x01}}, // old location area ID
		{IEI: 0x11, Format: nas.IETLV, Value: []byte{0x33, 0x19, 0xa2}},             // MS classmark 2
		{IEI: 0x20, Format: nas.IETLV, Value: []byte{0x60, 0x14}},                   // MS classmark 3
		{IEI: 0x40, Format: nas.IETLV, Value: []byte{0x04, 0x02, 0x60, 0x04}},       // supported codecs
		{IEI: 0x5d, Format: nas.IETLV, Value: []byte{0x00, 0x04}},                   // voice domain preference
	}

	in := &AttachRequest{
		EPSAttachType:       AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 3},
		EPSMobileIdentity: GUTIIdentity(GUTI{
			PLMN:       nas.PLMN{MCC: "001", MNC: "01"},
			MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x00, 0x00, 0x00, 0x01},
		}),
		UENetworkCapability: UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: []byte{0x02, 0x01, 0xd0, 0x11},

		AdditionalGUTI:          ptr(GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 2, MMECode: 1, TMSI: [4]byte{0x03, 0x00, 0x03, 0xe6}})),
		MSNetworkCapability:     mustMSNetCap(0xe5, 0xe0, 0x34),
		TMSIStatus:              ptr(true),
		AdditionalUpdateType:    ptr(AdditionalUpdateType{SAF: true}),
		DeviceProperties:        ptr(true),
		OldGUTIType:             ptr(GUTITypeNative),
		MSNetworkFeatureSupport: ptr(true),

		Unrecognized: unmodelled,
	}

	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseAttachRequest(b)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(in.MSNetworkCapability, out.MSNetworkCapability) {
		t.Errorf("MSNetworkCapability = %+v, want %+v", out.MSNetworkCapability, in.MSNetworkCapability)
	}

	if !reflect.DeepEqual(in.AdditionalGUTI, out.AdditionalGUTI) {
		t.Errorf("AdditionalGUTI = %+v, want %+v", out.AdditionalGUTI, in.AdditionalGUTI)
	}

	tv1Fields := []struct {
		name     string
		in, want any
	}{
		{"TMSIStatus", in.TMSIStatus, out.TMSIStatus},
		{"AdditionalUpdateType", in.AdditionalUpdateType, out.AdditionalUpdateType},
		{"DeviceProperties", in.DeviceProperties, out.DeviceProperties},
		{"OldGUTIType", in.OldGUTIType, out.OldGUTIType},
		{"MSNetworkFeatureSupport", in.MSNetworkFeatureSupport, out.MSNetworkFeatureSupport},
	}
	for _, f := range tv1Fields {
		if !reflect.DeepEqual(f.in, f.want) {
			t.Errorf("%s = %v, want %v", f.name, f.want, f.in)
		}
	}

	if len(out.Unrecognized) != len(unmodelled) {
		t.Fatalf("preserved %d unmodelled elements, want %d: %+v", len(out.Unrecognized), len(unmodelled), out.Unrecognized)
	}

	again, err := out.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(again, b) {
		t.Fatalf("re-encode dropped something\n got % x\nwant % x", again, b)
	}
}

func TestAttachRequestRoundTrip(t *testing.T) {
	in := &AttachRequest{
		EPSAttachType:       AttachTypeEPS,
		NASKeySetIdentifier: nas.NoKeySet,
		EPSMobileIdentity: GUTIIdentity(GUTI{
			PLMN:       nas.PLMN{MCC: "302", MNC: "720"},
			MMEGroupID: 0x1234, MMECode: 0x56, TMSI: [4]byte{0xde, 0xad, 0xbe, 0xef},
		}),
		UENetworkCapability: UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: []byte{0x02, 0x01, 0xd0, 0x11},

		// The DRX parameter is not modelled, so it travels as a preserved element.
		// Parsing names it from the message's element table.
		Unrecognized: []nas.RawIE{
			{IEI: 0x5C, Format: nas.IETV3, Value: []byte{0x00, 0x08}, Name: "DRX parameter"},
		},
	}

	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseAttachRequest(b)
	if err != nil {
		t.Fatal(err)
	}

	if out.EPSAttachType != in.EPSAttachType || out.NASKeySetIdentifier != in.NASKeySetIdentifier ||
		!reflect.DeepEqual(out.EPSMobileIdentity, in.EPSMobileIdentity) ||
		!out.UENetworkCapability.Equal(in.UENetworkCapability) ||
		!bytes.Equal(out.ESMMessageContainer, in.ESMMessageContainer) ||
		!reflect.DeepEqual(out.Unrecognized, in.Unrecognized) {
		t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
	}
}

// mustMSNetCap decodes an MS network capability fixture that must be well-formed.
func mustMSNetCap(b ...byte) *MSNetworkCapability {
	c, err := ParseMSNetworkCapability(b)
	if err != nil {
		panic(err)
	}

	return &c
}
