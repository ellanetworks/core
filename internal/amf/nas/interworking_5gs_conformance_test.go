// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

// Conformance tests for interworking without N26
func TestFiveGSNetworkFeatureSupportEncodesIWKN26(t *testing.T) {
	const iwkN26Octet3 = 0x40 // octet 3, bit 7

	for _, tc := range []struct {
		name string
		nfs  fgs.NetworkFeatureSupport
		want byte
	}{
		{"not supported", fgs.NetworkFeatureSupport{}, 0x00},
		{"supported", fgs.NetworkFeatureSupport{IWKN26: true}, iwkN26Octet3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := tc.nfs.MarshalBinary()
			if err != nil {
				t.Fatalf("encode 5GS network feature support: %v", err)
			}

			if len(raw) == 0 {
				t.Fatal("encoded no octets")
			}

			if raw[0]&iwkN26Octet3 != tc.want {
				t.Errorf("octet 3 = %#02x, want bit 7 = %#02x", raw[0], tc.want)
			}
		})
	}
}

// TS 24.501 §9.11.3.47
func TestFiveGSRequestTypeExistingPDUSessionCodePoint(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   fgs.RequestType
		want byte
	}{
		{"existing PDU session", fgs.RequestTypeExistingPDUSession, 0x02},
		{"initial request", fgs.RequestTypeInitialRequest, 0x01},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requested := tc.in
			msg := &fgs.ULNASTransport{
				PayloadContainerType: fgs.PayloadContainerTypeN1SMInfo,
				PayloadContainer:     []byte{0x2e, 0x01, 0x01},
				RequestType:          &requested,
			}

			raw, err := msg.MarshalBinary()
			if err != nil {
				t.Fatalf("encode UL NAS TRANSPORT: %v", err)
			}

			back, err := fgs.ParseULNASTransport(raw)
			if err != nil {
				t.Fatalf("parse the encoded UL NAS TRANSPORT: %v", err)
			}

			if back.RequestType == nil || *back.RequestType != tc.in {
				t.Fatalf("decoded request type = %v, want %d", back.RequestType, tc.in)
			}

			var found bool

			for i := 0; i < len(raw); i++ {
				if raw[i]&0xf0 == 0x80 && raw[i]&0x0f == tc.want {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("encoded % x carries no request type IE with IEI 0x8 and value %#03b in its low nibble", raw, tc.want)
			}
		})
	}
}

// TS 24.501 §9.11.3.56
func TestFiveGSUEStatusEncodesTheEMMRegistrationBit(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   fgs.UEStatus
		want byte
	}{
		{"neither", fgs.UEStatus{}, 0x00},
		{"EMM-REGISTERED", fgs.UEStatus{S1ModeReg: true}, 0x01},
		{"5GMM-REGISTERED", fgs.UEStatus{N1ModeReg: true}, 0x02},
		{"both", fgs.UEStatus{S1ModeReg: true, N1ModeReg: true}, 0x03},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.in.MarshalBinary()
			if len(raw) == 0 {
				t.Fatal("encoded no octets")
			}

			if raw[0] != tc.want {
				t.Errorf("octet 3 = %#02x, want %#02x", raw[0], tc.want)
			}
		})
	}
}

// TS 23.502 §4.11.2.3
func TestFiveGSMovingFromEPCIsRecognisedFromTheUEStatusIEAlone(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  *fgs.RegistrationRequest
		want bool
	}{
		{"no UE status IE", &fgs.RegistrationRequest{}, false},
		{"EMM-REGISTERED", &fgs.RegistrationRequest{UEStatus: &fgs.UEStatus{S1ModeReg: true}}, true},
		{"5GMM-REGISTERED only", &fgs.RegistrationRequest{UEStatus: &fgs.UEStatus{N1ModeReg: true}}, false},
		{"both", &fgs.RegistrationRequest{UEStatus: &fgs.UEStatus{S1ModeReg: true, N1ModeReg: true}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := movingFromEPC(tc.req); got != tc.want {
				t.Errorf("movingFromEPC = %v, want %v", got, tc.want)
			}
		})
	}
}
