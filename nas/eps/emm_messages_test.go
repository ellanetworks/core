// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestAuthenticationRoundTrips(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		in := &AuthenticationRequest{NASKeySetIdentifier: nas.NoKeySet}
		for i := range in.AUTN {
			in.AUTN[i] = 0xab
		}

		for i := range in.RAND {
			in.RAND[i] = byte(i)
		}

		b, err := in.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseAuthenticationRequest(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.NASKeySetIdentifier != in.NASKeySetIdentifier || out.RAND != in.RAND || out.AUTN != in.AUTN {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("Response", func(t *testing.T) {
		in := &AuthenticationResponse{RES: []byte{0x11, 0x22, 0x33, 0x44}}

		b, _ := in.MarshalBinary()

		out, err := ParseAuthenticationResponse(b)
		if err != nil || !bytes.Equal(out.RES, in.RES) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Reject", func(t *testing.T) {
		b, _ := (&AuthenticationReject{}).MarshalBinary()
		if _, err := ParseAuthenticationReject(b); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Failure no AUTS", func(t *testing.T) {
		in := &AuthenticationFailure{Cause: 21}

		b, _ := in.MarshalBinary()

		out, err := ParseAuthenticationFailure(b)
		if err != nil || out.Cause != 21 || out.AUTS != nil {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Failure with AUTS", func(t *testing.T) {
		in := &AuthenticationFailure{Cause: 21, AUTS: bytes.Repeat([]byte{0xcd}, 14)}

		b, _ := in.MarshalBinary()

		out, err := ParseAuthenticationFailure(b)
		if err != nil || out.Cause != 21 || !bytes.Equal(out.AUTS, in.AUTS) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}

func TestIdentityRoundTrips(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		b, _ := (&IdentityRequest{IdentityType: 1}).MarshalBinary()

		out, err := ParseIdentityRequest(b)
		if err != nil || out.IdentityType != 1 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Response", func(t *testing.T) {
		in := &IdentityResponse{MobileIdentity: MobileIMSI("001010000000001")}

		b, _ := in.MarshalBinary()

		out, err := ParseIdentityResponse(b)
		if err != nil || !reflect.DeepEqual(out.MobileIdentity, in.MobileIdentity) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}

// mustBytes returns the octets of a MarshalBinary call that must succeed, so encode
// calls stay usable as expressions in test fixtures.
func mustBytes(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}

	return b
}

// testTAIList is a one-entry TAI list fixture: PLMN 001-01, TAC 1.
func testTAIList() TAIList {
	return TAIList{{Type: PartialTAIListNonConsecutive, TAIs: []TAI{{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 1}}}}
}
