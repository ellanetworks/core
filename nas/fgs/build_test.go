// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs_test

import (
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/nas/nastest"
)

// TestBuildRegistrationRequestRoundTrip demonstrates the adversarial builder can
// construct a valid uplink REGISTRATION REQUEST — a message fgs only parses — and
// that it parses back with the expected fields.
func TestBuildRegistrationRequestRoundTrip(t *testing.T) {
	guti := []byte{0xf2, 0x00, 0xf1, 0x10, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	ueseccap := []byte{0xe0, 0xe0} // NEA0-2 / NIA0-2

	msg := nastest.BuildGMM(fgs.MsgRegistrationRequest).
		U8(0x79).            // ngKSI=7, registration type = initial
		LVE(guti).           // mandatory 5GS mobile identity
		TLV(0x2e, ueseccap). // optional UE security capability
		Bytes()

	req, err := fgs.ParseRegistrationRequest(msg)
	if err != nil {
		t.Fatalf("built REGISTRATION REQUEST did not parse: %v", err)
	}

	if req.RegistrationType != fgs.RegistrationTypeInitial {
		t.Errorf("RegistrationType = %d, want %d", req.RegistrationType, fgs.RegistrationTypeInitial)
	}

	if req.NgKSI != nas.NoKeySet {
		t.Errorf("ngKSI = %v, want no key available", req.NgKSI)
	}

	if req.MobileIdentity.GUTI == nil || req.MobileIdentity.GUTI.PLMN.MCC != "001" {
		t.Errorf("MobileIdentity = %+v, want the 5G-GUTI in % x", req.MobileIdentity, guti)
	}

	want := fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0}
	if req.UESecurityCapability == nil || !req.UESecurityCapability.Equal(want) {
		t.Errorf("UESecurityCapability = %+v, want %+v", req.UESecurityCapability, want)
	}
}

// TestBuildMalformedRegistrationRequestRejected demonstrates the builder producing a
// deliberately malformed message (mandatory 5GS mobile identity cut short) and that
// the parser rejects it rather than mis-decoding — the property compliance tests rely on.
func TestBuildMalformedRegistrationRequestRejected(t *testing.T) {
	// Header + registration-type octet, then truncate before the mandatory identity.
	truncated := nastest.BuildGMM(fgs.MsgRegistrationRequest).U8(0x79).LVE([]byte{0xde, 0xad}).Truncate(4).Bytes()

	if _, err := fgs.ParseRegistrationRequest(truncated); err == nil {
		t.Fatal("expected a truncated REGISTRATION REQUEST to be rejected")
	}
}

// TestBuildRawInvalidHeader demonstrates constructing a message with a wrong extended
// protocol discriminator, which the parser must reject.
func TestBuildRawInvalidHeader(t *testing.T) {
	msg := nastest.BuildGMMRaw(0x99, uint8(fgs.SHTPlain), uint8(fgs.MsgRegistrationRequest)).U8(0x01).Bytes()

	if _, err := fgs.ParseRegistrationRequest(msg); err == nil {
		t.Fatal("expected a message with an invalid EPD to be rejected")
	}
}
