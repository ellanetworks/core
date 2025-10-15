package decoder_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder"
)

func TestDecodeNASMessage_RegistrationRequest(t *testing.T) {
	const message = "fgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8A=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	nas, err := decoder.DecodeNASMessage(raw, nil)
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

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.IntegrityAlgorithm.EEA0_5G {
		t.Error("UESecurityCapability IntegrityAlgorithm EEA0_5G is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.IntegrityAlgorithm.EEA1_128_5G {
		t.Error("UESecurityCapability IntegrityAlgorithm EEA1_128_5G is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.IntegrityAlgorithm.EEA2_128_5G {
		t.Error("UESecurityCapability IntegrityAlgorithm EEA2_128_5G is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.IntegrityAlgorithm.EEA3_128_5G {
		t.Error("UESecurityCapability IntegrityAlgorithm EEA3_128_5G is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.CipheringAlgorithm.EIA0_5G {
		t.Error("UESecurityCapability CipheringAlgorithm EIA0_5G is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.CipheringAlgorithm.EIA1_128_5G {
		t.Error("UESecurityCapability CipheringAlgorithm EIA1_128_5G is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.CipheringAlgorithm.EIA2_128_5G {
		t.Error("UESecurityCapability CipheringAlgorithm EIA2_128_5G is false, expected true")
	}

	if !nas.GmmMessage.RegistrationRequest.UESecurityCapability.CipheringAlgorithm.EIA3_128_5G {
		t.Error("UESecurityCapability CipheringAlgorithm EIA3_128_5G is false, expected true")
	}
}

func TestDecodeNASMessage_AuthenticationRequest(t *testing.T) {
	const message = "fgBWAAIAACEaBwCjbSa9vkiAkRdky8+5IBBH2jhAU2SAAE2CgCRBSs2H"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	nas, err := decoder.DecodeNASMessage(raw, nil)
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

	if nas.GmmMessage.GmmHeader.MessageType != "AuthenticationRequest (86)" {
		t.Errorf("Unexpected GmmMessage Type: got %v", nas.GmmMessage.GmmHeader.MessageType)
	}

	if nas.GmmMessage.AuthenticationRequest == nil {
		t.Fatal("AuthenticationRequest is nil")
	}

	if nas.GmmMessage.AuthenticationRequest.ABBA == nil {
		t.Fatal("ABBA is nil")
	}

	if len(nas.GmmMessage.AuthenticationRequest.ABBA) != 2 {
		t.Errorf("Unexpected ABBA length: got %d, want 2", len(nas.GmmMessage.AuthenticationRequest.ABBA))
	}

	expectedABBA := []uint8{0x00, 0x00}
	for i, v := range expectedABBA {
		if nas.GmmMessage.AuthenticationRequest.ABBA[i] != v {
			t.Errorf("Unexpected ABBA[%d]: got %x, want %x", i, nas.GmmMessage.AuthenticationRequest.ABBA[i], v)
		}
	}

	if len(nas.GmmMessage.AuthenticationRequest.AuthenticationParameterAUTN) != 16 {
		t.Errorf("Unexpected AuthenticationParameterAUTN length: got %d, want 16", len(nas.GmmMessage.AuthenticationRequest.AuthenticationParameterAUTN))
	}

	if len(nas.GmmMessage.AuthenticationRequest.AuthenticationParameterRAND) != 16 {
		t.Errorf("Unexpected AuthenticationParameterRAND length: got %d, want 16", len(nas.GmmMessage.AuthenticationRequest.AuthenticationParameterRAND))
	}
}

func TestDecodeNASMessage_AuthenticationFailure(t *testing.T) {
	const message = "fgBZFA=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	nas, err := decoder.DecodeNASMessage(raw, nil)
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

	if nas.GmmMessage.GmmHeader.MessageType != "AuthenticationFailure (89)" {
		t.Errorf("Unexpected GmmMessage Type: got %v", nas.GmmMessage.GmmHeader.MessageType)
	}

	if nas.GmmMessage.AuthenticationFailure == nil {
		t.Fatal("AuthenticationFailure is nil")
	}

	if nas.GmmMessage.AuthenticationFailure.Cause5GMM != "MAC failure (20)" {
		t.Errorf("Unexpected Cause5GMM: got %v, want 'MAC failure (20)'", nas.GmmMessage.AuthenticationFailure.Cause5GMM)
	}
}

func TestDecodeNASMessage_AuthenticationReject(t *testing.T) {
	const message = "fgBY"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	nas, err := decoder.DecodeNASMessage(raw, nil)
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

	if nas.GmmMessage.GmmHeader.MessageType != "AuthenticationReject (88)" {
		t.Errorf("Unexpected GmmMessage Type: got %v", nas.GmmMessage.GmmHeader.MessageType)
	}

	if nas.GmmMessage.AuthenticationReject == nil {
		t.Fatal("AuthenticationReject is nil")
	}

	if nas.GmmMessage.AuthenticationReject.AuthenticationRejectMessageIdentity != "AuthenticationReject" {
		t.Errorf("Unexpected AuthenticationRejectMessageIdentity: got %v, want AuthenticationReject", nas.GmmMessage.AuthenticationReject.AuthenticationRejectMessageIdentity)
	}
}

func TestDecodeNASMessage_AuthenticationResponse(t *testing.T) {
	const message = "fgBXLRAr/v+SKtfNKW4evtLp2SKq"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	nas, err := decoder.DecodeNASMessage(raw, nil)
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

	if nas.GmmMessage.GmmHeader.MessageType != "AuthenticationResponse (87)" {
		t.Errorf("Unexpected GmmMessage Type: got %v", nas.GmmMessage.GmmHeader.MessageType)
	}

	if nas.GmmMessage.AuthenticationResponse == nil {
		t.Fatal("AuthenticationResponse is nil")
	}

	if nas.GmmMessage.AuthenticationResponse.AuthenticationResponseMessageIdentity != "AuthenticationResponse" {
		t.Errorf("Unexpected AuthenticationResponseMessageIdentity: got %v, want AuthenticationResponse", nas.GmmMessage.AuthenticationResponse.AuthenticationResponseMessageIdentity)
	}

	if nas.GmmMessage.AuthenticationResponse.AuthenticationResponseParameter == nil {
		t.Fatal("AuthenticationResponseParameter is nil")
	}

	if len(nas.GmmMessage.AuthenticationResponse.AuthenticationResponseParameter.ResStar) != 16 {
		t.Errorf("Unexpected RES* length: got %d, want 16", len(nas.GmmMessage.AuthenticationResponse.AuthenticationResponseParameter.ResStar))
	}
}
