// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"encoding/hex"
	"os"
	"strings"
	"testing"
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

	if mt, err := PeekMessageType(sp.Payload); err != nil || mt != MsgAttachRequest {
		t.Fatalf("PeekMessageType = %#x, %v; want 0x41", mt, err)
	}

	ar, err := ParseAttachRequest(sp.Payload)
	if err != nil {
		t.Fatalf("attach request: %v", err)
	}

	if ar.EPSAttachType != AttachTypeCombined || ar.NASKeySetIdentifier != 0 {
		t.Fatalf("attach type=%d ksi=%d, want 2 / 0", ar.EPSAttachType, ar.NASKeySetIdentifier)
	}

	id := ar.EPSMobileIdentity
	if id.Type != IdentityGUTI || id.MCC != "001" || id.MNC != "01" ||
		id.MMEGroupID != 0x0002 || id.MMECode != 0x01 || id.MTMSI != 0x030003e6 {
		t.Fatalf("GUTI mismatch: %+v", id)
	}

	if len(ar.UENetworkCapability) != 5 || len(ar.ESMMessageContainer) != 5 {
		t.Fatalf("IE lengths: uenc=%d esm=%d",
			len(ar.UENetworkCapability), len(ar.ESMMessageContainer))
	}

	// The capture carries an MS network capability after a Last visited TAI and a
	// DRX parameter, so it exercises the optional-IE walk.
	if want := []byte{0xe5, 0xe0, 0x34}; !bytes.Equal(ar.MSNetworkCapability, want) {
		t.Fatalf("MSNetworkCapability = %x, want %x", ar.MSNetworkCapability, want)
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
		NASKeySetIdentifier: 0,
		EPSMobileIdentity:   EPSMobileIdentity{Type: IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1},
		UENetworkCapability: []byte{0xf0, 0x70, 0xc0},
		ESMMessageContainer: []byte{0x02, 0x01, 0xd0, 0x11},
	}

	prefix, err := base.Marshal()
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
			[]byte{0x52, 0, 0xf1, 0x10, 0x30, 0x39, 0x5c, 0x0a, 0x00, 0x50, 0x02, 0xde, 0xad, 0x31, 0x02, 0x11, 0x22, 0x5d, 0x01, 0xe0},
			[]byte{0x11, 0x22},
			false,
		},
		{"absent (only later IEs)", []byte{0x13, 0x05, 0, 0xf1, 0x10, 0x00, 0x01}, nil, false},
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

			if !bytes.Equal(out.MSNetworkCapability, tc.want) {
				t.Fatalf("MSNetworkCapability = %x, want %x", out.MSNetworkCapability, tc.want)
			}
		})
	}
}

// TestAttachRequestOptionalIEsRoundTrip fills every optional IE the message
// defines (TS 24.301 Table 8.2.4.1) with a distinct value, then checks the whole
// set survives Marshal → ParseAttachRequest byte-for-byte.
func TestAttachRequestOptionalIEsRoundTrip(t *testing.T) {
	u8 := func(v uint8) *uint8 { return &v }

	in := &AttachRequest{
		EPSAttachType:       AttachTypeEPS,
		NASKeySetIdentifier: 3,
		EPSMobileIdentity: EPSMobileIdentity{
			Type: IdentityGUTI, MCC: "001", MNC: "01",
			MMEGroupID: 1, MMECode: 1, MTMSI: 1,
		},
		UENetworkCapability: []byte{0xf0, 0x70, 0xc0},
		ESMMessageContainer: []byte{0x02, 0x01, 0xd0, 0x11},

		OldPTMSISignature:               []byte{0x01, 0x02, 0x03},
		AdditionalGUTI:                  []byte{0xf1, 0x10, 0x00, 0x02, 0x01, 0x03, 0x00, 0x03, 0xe6},
		LastVisitedRegisteredTAI:        []byte{0x00, 0xf1, 0x10, 0x30, 0x39},
		DRXParameter:                    []byte{0x00, 0x08},
		MSNetworkCapability:             []byte{0xe5, 0xe0, 0x34},
		OldLocationAreaID:               []byte{0x00, 0xf1, 0x10, 0x00, 0x01},
		TMSIStatus:                      u8(0x01),
		MobileStationClassmark2:         []byte{0x33, 0x19, 0xa2},
		MobileStationClassmark3:         []byte{0x60, 0x14},
		SupportedCodecs:                 []byte{0x04, 0x02, 0x60, 0x04},
		AdditionalUpdateType:            u8(0x02),
		VoiceDomainPreference:           []byte{0x00, 0x04},
		DeviceProperties:                u8(0x01),
		OldGUTIType:                     u8(0x00),
		MSNetworkFeatureSupport:         u8(0x01),
		TMSIBasedNRIContainer:           []byte{0x00, 0x00},
		T3324Value:                      []byte{0x21},
		T3412ExtendedValue:              []byte{0x0a},
		ExtendedDRXParameters:           []byte{0x2b},
		UEAdditionalSecurityCapability:  []byte{0x00, 0x00, 0x00, 0x00},
		UEStatus:                        []byte{0x01},
		AdditionalInformationRequested:  []byte{0x01},
		N1UENetworkCapability:           []byte{0x00},
		UERadioCapabilityIDAvailability: []byte{0x01},
		RequestedWUSAssistance:          []byte{0x01},
		DRXParameterNBS1Mode:            []byte{0x00},
		RequestedIMSIOffset:             []byte{0x00, 0x01},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseAttachRequest(b)
	if err != nil {
		t.Fatal(err)
	}

	byteFields := []struct {
		name     string
		in, want []byte
	}{
		{"OldPTMSISignature", in.OldPTMSISignature, out.OldPTMSISignature},
		{"AdditionalGUTI", in.AdditionalGUTI, out.AdditionalGUTI},
		{"LastVisitedRegisteredTAI", in.LastVisitedRegisteredTAI, out.LastVisitedRegisteredTAI},
		{"DRXParameter", in.DRXParameter, out.DRXParameter},
		{"MSNetworkCapability", in.MSNetworkCapability, out.MSNetworkCapability},
		{"OldLocationAreaID", in.OldLocationAreaID, out.OldLocationAreaID},
		{"MobileStationClassmark2", in.MobileStationClassmark2, out.MobileStationClassmark2},
		{"MobileStationClassmark3", in.MobileStationClassmark3, out.MobileStationClassmark3},
		{"SupportedCodecs", in.SupportedCodecs, out.SupportedCodecs},
		{"VoiceDomainPreference", in.VoiceDomainPreference, out.VoiceDomainPreference},
		{"TMSIBasedNRIContainer", in.TMSIBasedNRIContainer, out.TMSIBasedNRIContainer},
		{"T3324Value", in.T3324Value, out.T3324Value},
		{"T3412ExtendedValue", in.T3412ExtendedValue, out.T3412ExtendedValue},
		{"ExtendedDRXParameters", in.ExtendedDRXParameters, out.ExtendedDRXParameters},
		{"UEAdditionalSecurityCapability", in.UEAdditionalSecurityCapability, out.UEAdditionalSecurityCapability},
		{"UEStatus", in.UEStatus, out.UEStatus},
		{"AdditionalInformationRequested", in.AdditionalInformationRequested, out.AdditionalInformationRequested},
		{"N1UENetworkCapability", in.N1UENetworkCapability, out.N1UENetworkCapability},
		{"UERadioCapabilityIDAvailability", in.UERadioCapabilityIDAvailability, out.UERadioCapabilityIDAvailability},
		{"RequestedWUSAssistance", in.RequestedWUSAssistance, out.RequestedWUSAssistance},
		{"DRXParameterNBS1Mode", in.DRXParameterNBS1Mode, out.DRXParameterNBS1Mode},
		{"RequestedIMSIOffset", in.RequestedIMSIOffset, out.RequestedIMSIOffset},
	}
	for _, f := range byteFields {
		if !bytes.Equal(f.in, f.want) {
			t.Errorf("%s = %x, want %x", f.name, f.want, f.in)
		}
	}

	tv1Fields := []struct {
		name     string
		in, want *uint8
	}{
		{"TMSIStatus", in.TMSIStatus, out.TMSIStatus},
		{"AdditionalUpdateType", in.AdditionalUpdateType, out.AdditionalUpdateType},
		{"DeviceProperties", in.DeviceProperties, out.DeviceProperties},
		{"OldGUTIType", in.OldGUTIType, out.OldGUTIType},
		{"MSNetworkFeatureSupport", in.MSNetworkFeatureSupport, out.MSNetworkFeatureSupport},
	}
	for _, f := range tv1Fields {
		if f.want == nil || *f.want != *f.in {
			t.Errorf("%s = %v, want %d", f.name, f.want, *f.in)
		}
	}
}

func TestAttachRequestRoundTrip(t *testing.T) {
	in := &AttachRequest{
		EPSAttachType:       AttachTypeEPS,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity: EPSMobileIdentity{
			Type: IdentityGUTI, MCC: "302", MNC: "720",
			MMEGroupID: 0x1234, MMECode: 0x56, MTMSI: 0xdeadbeef,
		},
		UENetworkCapability: []byte{0xf0, 0x70, 0xc0},
		ESMMessageContainer: []byte{0x02, 0x01, 0xd0, 0x11},
		DRXParameter:        []byte{0x00, 0x08}, // SPLIT PG CYCLE + CN-specific DRX coefficient
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseAttachRequest(b)
	if err != nil {
		t.Fatal(err)
	}

	if out.EPSAttachType != in.EPSAttachType || out.NASKeySetIdentifier != in.NASKeySetIdentifier ||
		out.EPSMobileIdentity != in.EPSMobileIdentity ||
		!bytes.Equal(out.UENetworkCapability, in.UENetworkCapability) ||
		!bytes.Equal(out.ESMMessageContainer, in.ESMMessageContainer) ||
		!bytes.Equal(out.DRXParameter, in.DRXParameter) {
		t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
	}
}
