// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"testing"
)

// TestBuildRegistrationRequestRoundTrip demonstrates the adversarial builder can
// construct a valid uplink REGISTRATION REQUEST — a message fgs only parses — and
// that it parses back with the expected fields.
func TestBuildRegistrationRequestRoundTrip(t *testing.T) {
	guti := []byte{0xf2, 0x00, 0xf1, 0x10, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	ueseccap := []byte{0xe0, 0xe0} // NEA0-2 / NIA0-2

	msg := Build(MsgRegistrationRequest).
		U8(0x79).            // ngKSI=7, registration type = initial
		LVE(guti).           // mandatory 5GS mobile identity
		TLV(0x2e, ueseccap). // optional UE security capability
		Bytes()

	req, err := ParseRegistrationRequest(msg)
	if err != nil {
		t.Fatalf("built REGISTRATION REQUEST did not parse: %v", err)
	}

	if req.RegistrationType != RegistrationTypeInitial {
		t.Errorf("RegistrationType = %d, want %d", req.RegistrationType, RegistrationTypeInitial)
	}

	if req.NgKSI != 7 {
		t.Errorf("NgKSI = %d, want 7", req.NgKSI)
	}

	if !bytes.Equal(req.MobileIdentity, guti) {
		t.Errorf("MobileIdentity = % x, want % x", req.MobileIdentity, guti)
	}

	if !bytes.Equal(req.UESecurityCapability, ueseccap) {
		t.Errorf("UESecurityCapability = % x, want % x", req.UESecurityCapability, ueseccap)
	}
}

// TestBuildMalformedRegistrationRequestRejected demonstrates the builder producing a
// deliberately malformed message (mandatory 5GS mobile identity cut short) and that
// the parser rejects it rather than mis-decoding — the property compliance tests rely on.
func TestBuildMalformedRegistrationRequestRejected(t *testing.T) {
	// Header + registration-type octet, then truncate before the mandatory identity.
	truncated := Build(MsgRegistrationRequest).U8(0x79).LVE([]byte{0xde, 0xad}).Truncate(4).Bytes()

	if _, err := ParseRegistrationRequest(truncated); err == nil {
		t.Fatal("expected a truncated REGISTRATION REQUEST to be rejected")
	}
}

// TestBuildRawInvalidHeader demonstrates constructing a message with a wrong extended
// protocol discriminator, which the parser must reject.
func TestBuildRawInvalidHeader(t *testing.T) {
	msg := BuildRaw(0x99, uint8(SHTPlain), uint8(MsgRegistrationRequest)).U8(0x01).Bytes()

	if _, err := ParseRegistrationRequest(msg); err == nil {
		t.Fatal("expected a message with an invalid EPD to be rejected")
	}
}
