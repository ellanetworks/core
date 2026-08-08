// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"testing"
)

// TS 24.501 §9.11.3.56: EMM registration status is octet 3 bit 1, 5GMM
// registration status bit 2. A UE moving from EPC sets the first.
func TestUEStatusRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value byte
		want  UEStatus
	}{
		{"neither", 0x00, UEStatus{}},
		{"EMM-REGISTERED", 0x01, UEStatus{S1ModeReg: true}},
		{"5GMM-REGISTERED", 0x02, UEStatus{N1ModeReg: true}},
		{"both", 0x03, UEStatus{S1ModeReg: true, N1ModeReg: true}},
		// Spare bits are coded as zero but must not be mistaken for the two that
		// carry meaning.
		{"spare set", 0xFC, UEStatus{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseUEStatus([]byte{tc.value})
			if err != nil {
				t.Fatal(err)
			}

			if got != tc.want {
				t.Fatalf("ParseUEStatus(%#02x) = %+v, want %+v", tc.value, got, tc.want)
			}

			if raw := got.MarshalBinary(); !bytes.Equal(raw, []byte{tc.value &^ 0xFC}) {
				t.Errorf("re-encode = % x, want % x", raw, tc.value&^0xFC)
			}
		})
	}

	if _, err := ParseUEStatus([]byte{0x00, 0x00}); err == nil {
		t.Error("a two-octet UE status was accepted")
	}
}

// The element has to survive a REGISTRATION REQUEST round trip, since that is
// how the AMF learns the registration is an inter-system move.
func TestRegistrationRequestCarriesUEStatus(t *testing.T) {
	m := &RegistrationRequest{
		RegistrationType: RegistrationTypeMobilityUpdating,
		MobileIdentity:   MobileIdentity{},
		UEStatus:         &UEStatus{S1ModeReg: true},
	}

	wire, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseRegistrationRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	if got.UEStatus == nil || !got.UEStatus.S1ModeReg {
		t.Fatalf("UE status did not survive the round trip: %+v", got.UEStatus)
	}
}
