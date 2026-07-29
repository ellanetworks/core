// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestParseRegistrationRequest(t *testing.T) {
	// Header, ngKSI/registration-type octet 0x51 (regtype 1, FOR 0, ngKSI 5, TSC 0),
	// SUCI mobile identity (null scheme, MSIN 0000000001), then UE security
	// capability, uplink data status, MICO (RAAI 1), 5GS update type (NG-RAN RCU 1),
	// and requested DRX (value 3).
	b, _ := hex.DecodeString("7e004151000d0100f110ffff000000000000102e02800040020200b1530102510103")

	req, err := ParseRegistrationRequest(b)
	if err != nil {
		t.Fatalf("ParseRegistrationRequest: %v", err)
	}

	if req.RegistrationType != 1 || req.FOR || req.NgKSI != (nas.KeySetIdentifier{Value: 5}) {
		t.Errorf("scalars = type %d FOR %t ngKSI %v", req.RegistrationType, req.FOR, req.NgKSI)
	}

	suci := req.MobileIdentity.SUCI
	if suci == nil || suci.PLMN.MCC != "001" || suci.PLMN.MNC != "01" {
		t.Fatalf("MobileIdentity = %+v", req.MobileIdentity)
	}

	if supi, ok := suci.SUPI(); !ok || supi != "imsi-001010000000001" {
		t.Errorf("SUPI = %q (recovered %v)", supi, ok)
	}

	if req.UESecurityCapability == nil || !req.UESecurityCapability.SupportsEA(0) || req.UESecurityCapability.IA != 0 {
		t.Errorf("UESecurityCapability = %+v", req.UESecurityCapability)
	}

	if req.UplinkDataStatus == nil || !req.UplinkDataStatus.PSI[1] {
		t.Errorf("UplinkDataStatus = %v, want PSI 1 active", req.UplinkDataStatus)
	}

	if req.MICOIndication == nil || !req.MICOIndication.RAAI {
		t.Errorf("MICO indication = %v, want RAAI set", req.MICOIndication)
	}

	if req.UpdateType5GS == nil || !req.UpdateType5GS.NGRANRCU {
		t.Errorf("5GS update type = %v, want NG-RAN-RCU set", req.UpdateType5GS)
	}

	if req.RequestedDRXParameters == nil || req.RequestedDRXParameters.Value != DRXCycleParameterT128 {
		t.Errorf("requested DRX = %v, want T = 128", req.RequestedDRXParameters)
	}

	// Unset optional IEs remain nil.
	if req.PDUSessionStatus != nil || req.NASMessageContainer != nil || req.GMMCapability != nil {
		t.Errorf("unexpected optional IE set")
	}
}

func TestParseRegistrationRequestMandatoryOnly(t *testing.T) {
	b, _ := hex.DecodeString("7e0041010004ffffffff")

	req, err := ParseRegistrationRequest(b)
	if err != nil {
		t.Fatalf("ParseRegistrationRequest: %v", err)
	}

	// Type-of-identity 111 is an EUI-64, which this package keeps verbatim.
	if req.RegistrationType != 1 || req.MobileIdentity.Type() != IdentityEUI64 ||
		!bytes.Equal(req.MobileIdentity.Raw, []byte{0xff, 0xff, 0xff, 0xff}) {
		t.Errorf("type %d id %+v", req.RegistrationType, req.MobileIdentity)
	}

	if req.MICOIndication != nil || req.UpdateType5GS != nil || req.RequestedDRXParameters != nil {
		t.Errorf("absent optional elements should be nil")
	}
}

// TestParseRegistrationRequestSkipsUnknownIE guards TS 24.007 §11.2.4 skipping:
// an IE Ella does not table (here MS classmark 2, 0x41 TLV) placed before a
// later IE Ella reads (the NAS message container, 0x71 TLV-E) must be stepped
// over, not stop the walk. A stop would silently drop the container and defeat
// initial-NAS-message protection (TS 24.501 §4.4.6).
func TestParseRegistrationRequestSkipsUnknownIE(t *testing.T) {
	b := []byte{
		uint8(EPD5GMM), 0x00, uint8(MsgRegistrationRequest),
		0x01,                                                 // registration type initial / ngKSI
		0x00, 0x07, 0xf4, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, // mobile identity (LV-E): 5G-S-TMSI
		0x41, 0x03, 0xAA, 0xBB, 0xCC, // unknown IE: MS classmark 2 (0x41), TLV, 3 octets
		0x71, 0x00, 0x02, 0xDE, 0xAD, // NAS message container (0x71), TLV-E, 2 octets
	}

	req, err := ParseRegistrationRequest(b)
	if err != nil {
		t.Fatalf("ParseRegistrationRequest: %v", err)
	}

	if !bytes.Equal(req.NASMessageContainer, []byte{0xDE, 0xAD}) {
		t.Fatalf("NAS message container after an unknown IE = % x, want dead (the unknown IE stopped the walk)", req.NASMessageContainer)
	}
}
