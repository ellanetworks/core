// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"testing"
)

func TestAuthenticationRoundTrips(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		in := &AuthenticationRequest{NASKeySetIdentifier: 0x07, AUTN: bytes.Repeat([]byte{0xab}, 16)}
		for i := range in.RAND {
			in.RAND[i] = byte(i)
		}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseAuthenticationRequest(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.NASKeySetIdentifier != in.NASKeySetIdentifier || out.RAND != in.RAND || !bytes.Equal(out.AUTN, in.AUTN) {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("Response", func(t *testing.T) {
		in := &AuthenticationResponse{RES: []byte{0x11, 0x22, 0x33, 0x44}}

		b, _ := in.Marshal()

		out, err := ParseAuthenticationResponse(b)
		if err != nil || !bytes.Equal(out.RES, in.RES) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Reject", func(t *testing.T) {
		b, _ := (&AuthenticationReject{}).Marshal()
		if _, err := ParseAuthenticationReject(b); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Failure no AUTS", func(t *testing.T) {
		in := &AuthenticationFailure{Cause: 21}

		b, _ := in.Marshal()

		out, err := ParseAuthenticationFailure(b)
		if err != nil || out.Cause != 21 || out.AUTS != nil {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Failure with AUTS", func(t *testing.T) {
		in := &AuthenticationFailure{Cause: 21, AUTS: bytes.Repeat([]byte{0xcd}, 14)}

		b, _ := in.Marshal()

		out, err := ParseAuthenticationFailure(b)
		if err != nil || out.Cause != 21 || !bytes.Equal(out.AUTS, in.AUTS) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}

func TestIdentityRoundTrips(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		b, _ := (&IdentityRequest{IdentityType: 1}).Marshal()

		out, err := ParseIdentityRequest(b)
		if err != nil || out.IdentityType != 1 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Response", func(t *testing.T) {
		in := &IdentityResponse{MobileIdentity: []byte{0x19, 0x00, 0x01, 0x10, 0x10, 0x32, 0x54, 0x76}}

		b, _ := in.Marshal()

		out, err := ParseIdentityResponse(b)
		if err != nil || !bytes.Equal(out.MobileIdentity, in.MobileIdentity) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}
