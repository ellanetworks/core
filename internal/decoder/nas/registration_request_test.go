package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
	naslib "github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
)

func TestDecodeNASMessage_RegistrationRequest(t *testing.T) {
	const message = "fgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8A=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	nas := nas.DecodeNASMessage(raw, nil)

	if nas == nil {
		t.Fatal("Decoded NAS message is nil")
	}

	if nas.SecurityHeader.SecurityHeaderType.Label != "Plain NAS" {
		t.Errorf("Unexpected SecurityHeaderType: got %v", nas.SecurityHeader.SecurityHeaderType.Label)
	}

	if nas.SecurityHeader.SecurityHeaderType.Value != naslib.SecurityHeaderTypePlainNas {
		t.Errorf("Unexpected SecurityHeaderType value: got %d", nas.SecurityHeader.SecurityHeaderType.Value)
	}

	if nas.GsmMessage != nil {
		t.Fatal("GsmMessage is not nil")
	}

	if nas.GmmMessage == nil {
		t.Fatal("GmmMessage is nil")
	}

	if nas.GmmMessage.GmmHeader.MessageType.Label != "RegistrationRequest" {
		t.Errorf("Unexpected GmmMessage Type: got %v", nas.GmmMessage.GmmHeader.MessageType.Label)
	}

	if nas.GmmMessage.GmmHeader.MessageType.Value != naslib.MsgTypeRegistrationRequest {
		t.Errorf("Unexpected GmmMessage Type value: got %d", nas.GmmMessage.GmmHeader.MessageType.Value)
	}

	if nas.GmmMessage.RegistrationRequest == nil {
		t.Fatal("RegistrationRequest is nil")
	}

	if nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.Identity.Label != "SUCI" {
		t.Errorf("Unexpected MobileIdentity5GS Identity: got %v", nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.Identity)
	}

	if nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.Identity.Value != nasMessage.MobileIdentity5GSTypeSuci {
		t.Errorf("Unexpected MobileIdentity5GS Identity value: got %d", nas.GmmMessage.RegistrationRequest.MobileIdentity5GS.Identity.Value)
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

func TestDecodeNASMessage_RegistrationRequest_PeriodicRegistrationUpdating(t *testing.T) {
	const message = "fgHdM93FA34AQQMAC/Ji8ELK/gAAAAACcQAlfgBBAwAL8mLwQsr+AAAAAAJSYvBCAABkUAICABgBAXQAAFMBAQ=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	nasMsg := nas.DecodeNASMessage(raw, nil)
	if nasMsg == nil {
		t.Fatal("Decoded NAS message is nil")
	}

	if nasMsg.SecurityHeader.SecurityHeaderType.Label != "Integrity Protected" {
		t.Errorf("Unexpected SecurityHeaderType: got %v", nasMsg.SecurityHeader.SecurityHeaderType.Label)
	}

	if nasMsg.SecurityHeader.SecurityHeaderType.Value != naslib.SecurityHeaderTypeIntegrityProtected {
		t.Errorf("Unexpected SecurityHeaderType value: got %d", nasMsg.SecurityHeader.SecurityHeaderType.Value)
	}

	if nasMsg.GmmMessage == nil {
		t.Fatal("GmmMessage is nil")
	}

	if nasMsg.GmmMessage.RegistrationRequest == nil {
		t.Fatal("RegistrationRequest is nil")
	}

	regReq := nasMsg.GmmMessage.RegistrationRequest
	if regReq.RegistrationType5GS.Label != "Periodic Registration Updating" {
		t.Errorf("Unexpected RegistrationType5GS: got %v", regReq.RegistrationType5GS.Label)
	}

	if regReq.RegistrationType5GS.Value != nasMessage.RegistrationType5GSPeriodicRegistrationUpdating {
		t.Errorf("Unexpected RegistrationType5GS value: got %d", regReq.RegistrationType5GS.Value)
	}

	if regReq.MobileIdentity5GS.Identity.Label != "5G-GUTI" {
		t.Errorf("Unexpected MobileIdentity5GS Identity: got %v", regReq.MobileIdentity5GS.Identity.Label)
	}

	expectedGUTI := "26024cafe0000000002"
	if *regReq.MobileIdentity5GS.GUTI != expectedGUTI {
		t.Errorf("Unexpected GUTI: got %v", *regReq.MobileIdentity5GS.GUTI)
	}

	if regReq.NASMessageContainer == nil {
		t.Fatal("NASMessageContainer is nil")
	}
}

func TestDecodeNASMessage_RegistrationRequest_NASMsgContainer(t *testing.T) {
	const message = "fgHgQT9xBn4AQSkAC/IA8RDK/gAAAAACLgTwcPBwcQA7fgBBKQAL8gDxEMr+AAAAAAIQAQMuBPBw8HAvBQQBECAwUgDxEAAAARcH8HDAQBmAsBgBAXQAAJBTAQE="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	nasMsg := nas.DecodeNASMessage(raw, nil)

	if nasMsg == nil {
		t.Fatal("Decoded NAS message is nil")
	}

	if nasMsg.GmmMessage == nil {
		t.Fatal("GmmMessage is nil")
	}

	if nasMsg.GmmMessage.RegistrationRequest == nil {
		t.Fatal("RegistrationRequest is nil")
	}

	regReq := nasMsg.GmmMessage.RegistrationRequest

	if regReq.NASMessageContainer == nil {
		t.Fatal("NASMessageContainer is nil")
	}

	expectedContainer := "fgBBKQAL8gDxEMr+AAAAAAIQAQMuBPBw8HAvBQQBECAwUgDxEAAAARcH8HDAQBmAsBgBAXQAAJBTAQE="

	receivedNASMsg := encodeB64(regReq.NASMessageContainer)
	if receivedNASMsg != expectedContainer {
		t.Fatalf("Unexpected NASMessageContainer: got %v, want %v", receivedNASMsg, expectedContainer)
	}
}
