// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps_test

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/nastest"
)

// TestBuildIdentityResponseRoundTrip demonstrates the adversarial builder can
// construct a valid uplink EMM message — one eps only parses — and that it parses
// back with the expected field.
func TestBuildIdentityResponseRoundTrip(t *testing.T) {
	// IMSI 001010000000001: type 001, odd, then the digits two per octet.
	imsi := []byte{0x09, 0x10, 0x10, 0x00, 0x00, 0x00, 0x00, 0x10}

	msg := nastest.BuildEMM(eps.MsgIdentityResponse).LV(imsi).Bytes()

	resp, err := eps.ParseIdentityResponse(msg)
	if err != nil {
		t.Fatalf("built IDENTITY RESPONSE did not parse: %v", err)
	}

	if resp.MobileIdentity.IMSI == nil || *resp.MobileIdentity.IMSI != "001010000000001" {
		t.Errorf("MobileIdentity = %+v, want the IMSI in % x", resp.MobileIdentity, imsi)
	}
}

// TestBuildEMMStatusRoundTrip round-trips a header-plus-one-octet EMM message.
func TestBuildEMMStatusRoundTrip(t *testing.T) {
	msg := nastest.BuildEMM(eps.MsgEMMStatus).U8(111).Bytes() // cause #111 protocol error

	st, err := eps.ParseEMMStatus(msg)
	if err != nil {
		t.Fatalf("built EMM STATUS did not parse: %v", err)
	}

	if st.Cause != 111 {
		t.Errorf("EMMCause = %d, want 111", st.Cause)
	}
}

// TestBuildMalformedAttachRequestRejected demonstrates the builder producing a
// deliberately malformed ATTACH REQUEST (mandatory EPS mobile identity absent) and
// that the parser rejects it.
func TestBuildMalformedAttachRequestRejected(t *testing.T) {
	// Header + NAS-key-set/attach-type octet, nothing after: the mandatory mobile
	// identity LV cannot be read.
	msg := nastest.BuildEMM(eps.MsgAttachRequest).U8(0x71).Bytes()

	if _, err := eps.ParseAttachRequest(msg); err == nil {
		t.Fatal("expected an ATTACH REQUEST missing its mobile identity to be rejected")
	}
}

// TestBuildRawInvalidPD demonstrates constructing a message with a wrong protocol
// discriminator, which the parser must reject.
func TestBuildRawInvalidPD(t *testing.T) {
	// Octet 0 low nibble = 0x0F is not the EMM protocol discriminator (0x07).
	msg := nastest.BuildEMMRaw(0x0F, uint8(eps.MsgIdentityResponse)).LV([]byte{0x01}).Bytes()

	if _, err := eps.ParseIdentityResponse(msg); err == nil {
		t.Fatal("expected a message with an invalid protocol discriminator to be rejected")
	}
}
