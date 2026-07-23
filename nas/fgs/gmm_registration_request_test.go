// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestParseRegistrationRequest(t *testing.T) {
	// Header, ngKSI/registration-type octet 0x51 (regtype 1, FOR 0, ngKSI 5, TSC 0),
	// SUCI mobile identity, then UE security capability, uplink data status, MICO
	// (RAAI 1), 5GS update type (NG-RAN RCU 1), and requested DRX (value 3).
	b, _ := hex.DecodeString("7e00415100040100f1102e02800040020200b1530102510103")

	req, err := ParseRegistrationRequest(b)
	if err != nil {
		t.Fatalf("ParseRegistrationRequest: %v", err)
	}

	if req.RegistrationType != 1 || req.FOR != 0 || req.NgKSI != 5 || req.TSC != 0 {
		t.Errorf("scalars = type %d FOR %d ngKSI %d TSC %d", req.RegistrationType, req.FOR, req.NgKSI, req.TSC)
	}

	if !bytes.Equal(req.MobileIdentity, []byte{0x01, 0x00, 0xf1, 0x10}) {
		t.Errorf("MobileIdentity = %x", req.MobileIdentity)
	}

	if !bytes.Equal(req.UESecurityCapability, []byte{0x80, 0x00}) {
		t.Errorf("UESecurityCapability = %x", req.UESecurityCapability)
	}

	if !bytes.Equal(req.UplinkDataStatus, []byte{0x02, 0x00}) {
		t.Errorf("UplinkDataStatus = %x", req.UplinkDataStatus)
	}

	if req.MICOIndication == nil || req.RAAI() != 1 {
		t.Errorf("MICO/RAAI = %v/%d", req.MICOIndication, req.RAAI())
	}

	if req.UpdateType5GS == nil || req.NGRanRcu() != 1 {
		t.Errorf("UpdateType5GS/NGRanRcu = %v/%d", req.UpdateType5GS, req.NGRanRcu())
	}

	if req.RequestedDRXParameters == nil || req.DRXValue() != 3 {
		t.Errorf("RequestedDRXParameters/DRXValue = %v/%d", req.RequestedDRXParameters, req.DRXValue())
	}

	// Unset optional IEs remain nil.
	if req.PDUSessionStatus != nil || req.NASMessageContainer != nil || req.Capability5GMM != nil {
		t.Errorf("unexpected optional IE set")
	}
}

func TestParseRegistrationRequestMandatoryOnly(t *testing.T) {
	b, _ := hex.DecodeString("7e0041010004ffffffff")

	req, err := ParseRegistrationRequest(b)
	if err != nil {
		t.Fatalf("ParseRegistrationRequest: %v", err)
	}

	if req.RegistrationType != 1 || !bytes.Equal(req.MobileIdentity, []byte{0xff, 0xff, 0xff, 0xff}) {
		t.Errorf("type %d id %x", req.RegistrationType, req.MobileIdentity)
	}

	if req.RAAI() != 0 || req.NGRanRcu() != 0 || req.DRXValue() != 0 {
		t.Errorf("absent-IE accessors should be 0")
	}
}
