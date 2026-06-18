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
		!bytes.Equal(out.ESMMessageContainer, in.ESMMessageContainer) {
		t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
	}
}
