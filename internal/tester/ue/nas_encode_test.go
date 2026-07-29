// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// plainIdentityRequest builds a minimal plain 5GMM message to protect.
func plainIdentityRequest(t *testing.T) []byte {
	t.Helper()

	b, err := (&fgs.IdentityRequest{IdentityType: fgs.IdentitySUCI}).MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain identity request: %v", err)
	}

	return b
}

// TestNASEncodeUnprotectRoundTrip proves the UE's protect path (NASEncode via
// fgs.Protect) round-trips through fgs.Unprotect for both an integrity-only and a
// ciphered security header, across each supported algorithm pair.
func TestNASEncodeUnprotectRoundTrip(t *testing.T) {
	plain := plainIdentityRequest(t)

	for _, tc := range []struct {
		name    string
		nia     uint8
		nea     uint8
		sht     uint8
		ciphers bool
	}{
		{"nia2-nea0-integrity", AlgIntegrity128NIA2, AlgCiphering128NEA0, uint8(fgs.SHTIntegrityProtected), false},
		{"nia2-nea2-ciphered", AlgIntegrity128NIA2, AlgCiphering128NEA2, uint8(fgs.SHTIntegrityProtectedCiphered), true},
		{"nia1-nea1-ciphered", AlgIntegrity128NIA1, AlgCiphering128NEA1, uint8(fgs.SHTIntegrityProtectedCiphered), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ue := &UE{UeSecurity: &UESecurity{
				IntegrityAlg: tc.nia,
				CipheringAlg: tc.nea,
				KnasInt:      [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				KnasEnc:      [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
				ULCount:      nas.MakeCount(0, 3),
			}}

			wire, err := ue.EncodeNasPduWithSecurity(plain, tc.sht)
			if err != nil {
				t.Fatalf("EncodeNasPduWithSecurity: %v", err)
			}

			if fgs.SecurityHeaderType(wire[1]&0x0f) != fgs.SecurityHeaderType(tc.sht) {
				t.Fatalf("wire sht = %#x, want %#x", wire[1]&0x0f, tc.sht)
			}

			// The UE advanced ULCount to 4; recover the count the message carried.
			recovered, err := unprotected(fgs.Unprotect(wire, nas.MakeCount(0, 3), nas.DirectionUplink, mustSecurityContext(t, nas.IntegrityAlgorithm(tc.nia), nas.CipheringAlgorithm(tc.nea), ue.UeSecurity.KnasInt, ue.UeSecurity.KnasEnc)))
			if err != nil {
				t.Fatalf("Unprotect: %v", err)
			}

			if !bytes.Equal(recovered, plain) {
				t.Fatalf("round-trip mismatch:\n got  %x\n want %x", recovered, plain)
			}

			if ue.UeSecurity.ULCount != nas.MakeCount(0, 4) {
				t.Fatalf("ULCount = %d, want 4", ue.UeSecurity.ULCount)
			}
		})
	}
}
