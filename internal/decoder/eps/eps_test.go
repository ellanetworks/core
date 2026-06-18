// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

func decodeHex(t *testing.T, h string) *NASMessage {
	t.Helper()

	raw, err := hex.DecodeString(h)
	if err != nil {
		t.Fatal(err)
	}

	return DecodeEPSNASMessage(raw)
}

// TestDecodeAttachRequest uses a NAS-PDU captured from a live deployment: an
// integrity-protected combined ATTACH REQUEST carrying a GUTI and a PDN
// Connectivity Request in its ESM container.
func TestDecodeAttachRequest(t *testing.T) {
	msg := decodeHex(t, "17d74e9de8050741020bf699f9100001010000000207f070000018008000330236d011272d8080211001000010810600000000830600000000000d00000a00000500001000001100001a01010023000024005299f91000015c0a009011034f1886f15d0106e0c1")

	if msg.SecurityHeader.SecurityHeaderType.Label != "Integrity protected" {
		t.Fatalf("SHT = %q", msg.SecurityHeader.SecurityHeaderType.Label)
	}

	if msg.EMMMessage == nil || msg.EMMMessage.EMMHeader.MessageType.Label != "Attach Request" {
		t.Fatalf("EMM = %+v", msg.EMMMessage)
	}

	ar := msg.EMMMessage.AttachRequest
	if ar == nil {
		t.Fatal("attach request not decoded")
	}

	if ar.AttachType.Label != "combined EPS/IMSI attach" {
		t.Fatalf("attach type = %q", ar.AttachType.Label)
	}

	if ar.MobileIdentity.Type != "guti" || ar.MobileIdentity.GUTI == nil ||
		ar.MobileIdentity.GUTI.MTMSI != 2 || ar.MobileIdentity.GUTI.MCC != "999" {
		t.Fatalf("mobile identity = %+v", ar.MobileIdentity)
	}

	if ar.ESMContainer == nil || ar.ESMContainer.ESMHeader.MessageType.Label != "PDN Connectivity Request" {
		t.Fatalf("ESM container = %+v", ar.ESMContainer)
	}
}

// TestDecodeTrackingAreaUpdateRequest uses a live NAS-PDU: an integrity-protected
// TRACKING AREA UPDATE REQUEST.
func TestDecodeTrackingAreaUpdateRequest(t *testing.T) {
	msg := decodeHex(t, "17659a6d010d0748000bf699f910000101000000015807f07000001800805299f9100001570220005d0106e0c1")

	if msg.EMMMessage == nil || msg.EMMMessage.EMMHeader.MessageType.Label != "Tracking Area Update Request" {
		t.Fatalf("EMM = %+v", msg.EMMMessage)
	}

	if msg.EMMMessage.TrackingAreaUpdateRequest == nil {
		t.Fatal("TAU request not decoded")
	}
}

// TestDecodeEncrypted uses a live ciphered NAS-PDU (SHT=2): the decoder reports
// the security header and the encrypted flag, without an inner message.
func TestDecodeEncrypted(t *testing.T) {
	msg := decodeHex(t, "2774a88ff701128f7ddc4907f3")

	if !msg.Encrypted {
		t.Fatal("expected encrypted")
	}

	if msg.EMMMessage != nil {
		t.Fatal("must not decode an inner message when ciphered")
	}

	if msg.SecurityHeader.SecurityHeaderType.Label != "Integrity protected and ciphered" {
		t.Fatalf("SHT = %q", msg.SecurityHeader.SecurityHeaderType.Label)
	}
}

// TestDecodePlainIdentityRequest decodes a plain (unprotected) message built with
// the codec, exercising the non-wrapped path.
func TestDecodePlainIdentityRequest(t *testing.T) {
	b, err := (&eps.IdentityRequest{IdentityType: 1}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeEPSNASMessage(b)

	if msg.SecurityHeader.SecurityHeaderType.Label != "Plain NAS" {
		t.Fatalf("SHT = %q", msg.SecurityHeader.SecurityHeaderType.Label)
	}

	if msg.EMMMessage == nil || msg.EMMMessage.IdentityRequest == nil ||
		msg.EMMMessage.IdentityRequest.IdentityType != 1 {
		t.Fatalf("identity request = %+v", msg.EMMMessage)
	}
}

// The following NAS-PDUs are captured from a live deployment (the 999/01 test
// PLMN), one per message type the MME exchanges in the clear.

func TestDecodeAuthenticationRequest(t *testing.T) {
	msg := decodeHex(t, "075200ea3d8ec68864b3dce98a956efffb8adf103787800bf66780007f99d3a08c910b95")

	if msg.EMMMessage == nil || msg.EMMMessage.AuthenticationRequest == nil ||
		len(msg.EMMMessage.AuthenticationRequest.RAND) != 32 {
		t.Fatalf("authentication request = %+v", msg.EMMMessage)
	}
}

func TestDecodeAuthenticationResponse(t *testing.T) {
	msg := decodeHex(t, "075308333f7d3146a2c189")

	if msg.EMMMessage == nil || msg.EMMMessage.AuthenticationResponse == nil ||
		msg.EMMMessage.AuthenticationResponse.RES == "" {
		t.Fatalf("authentication response = %+v", msg.EMMMessage)
	}
}

func TestDecodeSecurityModeCommand(t *testing.T) {
	msg := decodeHex(t, "37d71eeb1400075d220004f0700000c1")

	if msg.SecurityHeader.SecurityHeaderType.Label != "Integrity protected with new EPS security context" {
		t.Fatalf("SHT = %q", msg.SecurityHeader.SecurityHeaderType.Label)
	}

	smc := msg.EMMMessage.SecurityModeCommand
	if smc == nil || smc.CipheringAlgorithm.Label != "EEA2" || smc.IntegrityAlgorithm.Label != "EIA2" {
		t.Fatalf("security mode command = %+v", smc)
	}
}

func TestDecodeServiceRequest(t *testing.T) {
	msg := decodeHex(t, "c7038a84")

	if msg.EMMMessage == nil || msg.EMMMessage.ServiceRequest == nil {
		t.Fatalf("service request = %+v", msg.EMMMessage)
	}
}

func TestDecodeServiceReject(t *testing.T) {
	msg := decodeHex(t, "074e09")

	if msg.EMMMessage == nil || msg.EMMMessage.EMMHeader.MessageType.Label != "Service Reject" {
		t.Fatalf("EMM = %+v", msg.EMMMessage)
	}
}

func TestDecodePlainAttachRequest(t *testing.T) {
	msg := decodeHex(t, "07417108999910480000464407f070000018008000330265d011272d8080211001000010810600000000830600000000000d00000a00000500001000001100001a01010023000024005c0a005d0106c1")

	ar := msg.EMMMessage.AttachRequest
	if ar == nil || ar.MobileIdentity.Type != "imsi" {
		t.Fatalf("attach request = %+v", ar)
	}

	if ar.ESMContainer == nil || ar.ESMContainer.ESMHeader.MessageType.Label != "PDN Connectivity Request" {
		t.Fatalf("ESM container = %+v", ar.ESMContainer)
	}
}

// The PDN address is only ever sent ciphered on the wire (in the Activate
// Default Bearer of the Attach Accept), so it is exercised with codec-built
// values rather than a capture.
func TestPDNAddressIPv4(t *testing.T) {
	a := pdnAddress((&eps.PDNAddress{PDNType: 1, IPv4: [4]byte{10, 45, 0, 7}}).Marshal())
	if a == nil || a.Type.Label != "IPv4" || a.IPv4 != "10.45.0.7" || a.IPv6InterfaceID != "" {
		t.Fatalf("pdn address = %+v", a)
	}
}

func TestPDNAddressIPv4v6(t *testing.T) {
	a := pdnAddress((&eps.PDNAddress{
		PDNType: 3,
		IPv4:    [4]byte{10, 45, 0, 7},
		IPv6IID: [8]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77},
	}).Marshal())
	if a == nil || a.Type.Label != "IPv4v6" || a.IPv4 != "10.45.0.7" || a.IPv6InterfaceID != "0011:2233:4455:6677" {
		t.Fatalf("pdn address = %+v", a)
	}
}

func TestDecodeInvalid(t *testing.T) {
	if msg := DecodeEPSNASMessage([]byte{0x07}); msg.Error == "" {
		t.Fatal("expected an error for a too-short message")
	}
}
