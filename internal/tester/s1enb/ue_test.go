// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"bytes"
	"testing"

	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

func TestBuildAttachRequest(t *testing.T) {
	ue := &UE{IMSI: "001010000000001"}

	b, err := ue.buildAttachRequest()
	if err != nil {
		t.Fatal(err)
	}

	req, err := eps.ParseAttachRequest(b)
	if err != nil {
		t.Fatalf("parse attach request: %v", err)
	}

	if req.EPSAttachType != eps.AttachTypeEPS {
		t.Fatalf("attach type = %d, want %d", req.EPSAttachType, eps.AttachTypeEPS)
	}

	if req.EPSMobileIdentity.Type != eps.IdentityIMSI || req.EPSMobileIdentity.Digits != ue.IMSI {
		t.Fatalf("identity = %+v, want IMSI %s", req.EPSMobileIdentity, ue.IMSI)
	}

	if len(req.UENetworkCapability) == 0 || len(req.ESMMessageContainer) == 0 {
		t.Fatalf("missing UE network capability or ESM container")
	}
}

// TestUEKeyDerivationRoundTrip checks the UE's NAS-key derivation and algorithm
// mapping are self-consistent: a message protected with the derived keys
// unprotects back to the original.
func TestUEKeyDerivationRoundTrip(t *testing.T) {
	for _, alg := range []uint8{0, 1, 2} { // null, SNOW3G, AES
		ue := &UE{kasme: make([]byte, 32), eea: alg, eia: alg}
		for i := range ue.kasme {
			ue.kasme[i] = byte(i + 1)
		}

		if err := ue.deriveNASKeys(); err != nil {
			t.Fatalf("alg %d: derive NAS keys: %v", alg, err)
		}

		plain := []byte{0x07, 0x42, 0x01, 0x02, 0x03}

		wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nascommon.NASCount(0, 0),
			nascommon.DirectionUplink, ue.knasInt, ue.knasEnc, ue.integrityAlg(), ue.cipherAlg())
		if err != nil {
			t.Fatalf("alg %d: protect: %v", alg, err)
		}

		back, err := eps.Unprotect(wire, nascommon.NASCount(0, wire[5]), nascommon.DirectionUplink,
			ue.knasInt, ue.knasEnc, ue.integrityAlg(), ue.cipherAlg())
		if err != nil {
			t.Fatalf("alg %d: unprotect: %v", alg, err)
		}

		if !bytes.Equal(back, plain) {
			t.Fatalf("alg %d: round-trip = % x, want % x", alg, back, plain)
		}
	}
}

// TestHandleAuthenticationRequest checks the UE computes a RES and derives a
// 256-bit K_ASME from a challenge (the Milenage + KDF wiring).
func TestHandleAuthenticationRequest(t *testing.T) {
	ue := &UE{plmn: []byte{0x00, 0xf1, 0x10}}
	for i := range ue.K {
		ue.K[i] = byte(i)
		ue.OPc[i] = byte(0xff - i)
	}

	var rand [16]byte
	for i := range rand {
		rand[i] = byte(i + 1)
	}

	req, err := (&eps.AuthenticationRequest{RAND: rand, AUTN: make([]byte, 16)}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	respNAS, err := ue.handleAuthenticationRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	if len(ue.kasme) != 32 {
		t.Fatalf("K_ASME length = %d, want 32", len(ue.kasme))
	}

	resp, err := eps.ParseAuthenticationResponse(respNAS)
	if err != nil {
		t.Fatalf("parse authentication response: %v", err)
	}

	if len(resp.RES) != 8 {
		t.Fatalf("RES length = %d, want 8", len(resp.RES))
	}
}
