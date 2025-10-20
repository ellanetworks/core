package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
)

func TestDecodeNASMessage_RegistrationRequest(t *testing.T) {
	const message = "fgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8A=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	nas, err := nas.DecodeNASMessage(raw, nil)
	if err != nil {
		t.Fatalf("NAS message decode failed: %v", err)
	}

	if nas == nil {
		t.Fatal("Decoded NAS message is nil")
	}

	if nas.SecurityHeader.SecurityHeaderType != "Plain NAS" {
		t.Errorf("Unexpected SecurityHeaderType: got %v", nas.SecurityHeader.SecurityHeaderType)
	}

	if nas.GsmMessage != nil {
		t.Fatal("GsmMessage is not nil")
	}

	if nas.GmmMessage == nil {
		t.Fatal("GmmMessage is nil")
	}

	if nas.GmmMessage.GmmHeader.MessageType != "RegistrationRequest (65)" {
		t.Errorf("Unexpected GmmMessage Type: got %v", nas.GmmMessage.GmmHeader.MessageType)
	}

	if nas.GmmMessage.RegistrationRequest == nil {
		t.Fatal("RegistrationRequest is nil")
	}

	if nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.Identity != "SUCI" {
		t.Errorf("Unexpected MobileIdentity5GS Identity: got %v", nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.Identity)
	}

	if nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.SUCI == nil {
		t.Fatal("SUCI is nil")
	}

	expectedSuci := "suci-0-001-01-0000-0-0-4447867552"
	if *nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.SUCI != expectedSuci {
		t.Errorf("Unexpected SUCI: got %v, want %v", *nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.SUCI, expectedSuci)
	}

	if nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.PLMNID == nil {
		t.Fatal("PLMNID is nil")
	}

	if nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.PLMNID.Mcc != "001" {
		t.Errorf("Unexpected MCC: got %v", nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.PLMNID.Mcc)
	}

	if nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.PLMNID.Mnc != "01" {
		t.Errorf("Unexpected MNC: got %v", nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.PLMNID.Mnc)
	}

	// check ue security capability
	if nas.GmmMessage.RegistrationRequest.UESecurityCapability == nil {
		t.Fatal("UESecurityCapability is nil")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.IntegrityAlgorithm.NIA0 {
		t.Error("UESecurityCapability IntegrityAlgorithm NIA0 is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.IntegrityAlgorithm.NIA1 {
		t.Error("UESecurityCapability IntegrityAlgorithm NIA1 is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.IntegrityAlgorithm.NIA2 {
		t.Error("UESecurityCapability IntegrityAlgorithm NIA2 is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.IntegrityAlgorithm.NIA3 {
		t.Error("UESecurityCapability IntegrityAlgorithm NIA3 is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.CipheringAlgorithm.NEA0 {
		t.Error("UESecurityCapability CipheringAlgorithm NEA0 is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.CipheringAlgorithm.NEA1 {
		t.Error("UESecurityCapability CipheringAlgorithm NEA1 is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.CipheringAlgorithm.NEA2 {
		t.Error("UESecurityCapability CipheringAlgorithm NEA2 is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.CipheringAlgorithm.NEA3 {
		t.Error("UESecurityCapability CipheringAlgorithm NEA3 is false, expected true")
	}
}
