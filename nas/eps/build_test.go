// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"testing"
)

// TestBuildIdentityResponseRoundTrip demonstrates the adversarial builder can
// construct a valid uplink EMM message — one eps only parses — and that it parses
// back with the expected field.
func TestBuildIdentityResponseRoundTrip(t *testing.T) {
	imsi := []byte{0x08, 0x29, 0x43, 0x65, 0x87, 0x09, 0x21, 0x43}

	msg := Build(MsgIdentityResponse).LV(imsi).Bytes()

	resp, err := ParseIdentityResponse(msg)
	if err != nil {
		t.Fatalf("built IDENTITY RESPONSE did not parse: %v", err)
	}

	if !bytes.Equal(resp.MobileIdentity, imsi) {
		t.Errorf("MobileIdentity = % x, want % x", resp.MobileIdentity, imsi)
	}
}

// TestBuildEMMStatusRoundTrip round-trips a header-plus-one-octet EMM message.
func TestBuildEMMStatusRoundTrip(t *testing.T) {
	msg := Build(MsgEMMStatus).U8(111).Bytes() // cause #111 protocol error

	st, err := ParseEMMStatus(msg)
	if err != nil {
		t.Fatalf("built EMM STATUS did not parse: %v", err)
	}

	if st.EMMCause != 111 {
		t.Errorf("EMMCause = %d, want 111", st.EMMCause)
	}
}

// TestBuildMalformedAttachRequestRejected demonstrates the builder producing a
// deliberately malformed ATTACH REQUEST (mandatory EPS mobile identity absent) and
// that the parser rejects it.
func TestBuildMalformedAttachRequestRejected(t *testing.T) {
	// Header + NAS-key-set/attach-type octet, nothing after: the mandatory mobile
	// identity LV cannot be read.
	msg := Build(MsgAttachRequest).U8(0x71).Bytes()

	if _, err := ParseAttachRequest(msg); err == nil {
		t.Fatal("expected an ATTACH REQUEST missing its mobile identity to be rejected")
	}
}

// TestBuildRawInvalidPD demonstrates constructing a message with a wrong protocol
// discriminator, which the parser must reject.
func TestBuildRawInvalidPD(t *testing.T) {
	// Octet 0 low nibble = 0x0F is not the EMM protocol discriminator (0x07).
	msg := BuildRaw(0x0F, uint8(MsgIdentityResponse)).LV([]byte{0x01}).Bytes()

	if _, err := ParseIdentityResponse(msg); err == nil {
		t.Fatal("expected a message with an invalid protocol discriminator to be rejected")
	}
}
