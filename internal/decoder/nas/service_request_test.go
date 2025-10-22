package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
	naslib "github.com/omec-project/nas"
)

func TestDecodeNASMessage_ServiceRequest(t *testing.T) {
	const message = "fgGqIV5THX4ATBAAB/T+AAAAAAJxABV+AEwQAAf0/gAAAAACQAICAFACAgA="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	nas := nas.DecodeNASMessage(raw, nil)

	if nas == nil {
		t.Fatal("Decoded NAS message is nil")
	}

	if nas.SecurityHeader.SecurityHeaderType.Label != "Integrity Protected" {
		t.Errorf("Unexpected SecurityHeaderType: got %v", nas.SecurityHeader.SecurityHeaderType.Label)
	}

	if nas.SecurityHeader.SecurityHeaderType.Value != naslib.SecurityHeaderTypeIntegrityProtected {
		t.Errorf("Unexpected SecurityHeaderType value: got %d", nas.SecurityHeader.SecurityHeaderType.Value)
	}

	if nas.GsmMessage != nil {
		t.Fatal("GsmMessage is not nil")
	}

	if nas.GmmMessage == nil {
		t.Fatal("GmmMessage is nil")
	}

	if nas.GmmMessage.GmmHeader.MessageType.Label != "ServiceRequest" {
		t.Errorf("Unexpected GmmMessage Type: got %v", nas.GmmMessage.GmmHeader.MessageType.Label)
	}

	if nas.GmmMessage.GmmHeader.MessageType.Value != naslib.MsgTypeServiceRequest {
		t.Errorf("Unexpected GmmMessage Type value: got %d", nas.GmmMessage.GmmHeader.MessageType.Value)
	}

	if nas.GmmMessage.ServiceRequest == nil {
		t.Fatal("ServiceRequest is nil")
	}

	if nas.GmmMessage.ServiceRequest.ServiceTypeAndNgksi.ServiceType.Label != "Data" {
		t.Errorf("Unexpected ServiceType: got %v", nas.GmmMessage.ServiceRequest.ServiceTypeAndNgksi.ServiceType.Label)
	}

	if nas.GmmMessage.ServiceRequest.ServiceTypeAndNgksi.ServiceType.Value != 1 {
		t.Errorf("Unexpected ServiceType value: got %d", nas.GmmMessage.ServiceRequest.ServiceTypeAndNgksi.ServiceType.Value)
	}

	if nas.GmmMessage.ServiceRequest.ServiceTypeAndNgksi.TSC.Label != "Native" {
		t.Errorf("Unexpected TSC: got %v", nas.GmmMessage.ServiceRequest.ServiceTypeAndNgksi.TSC.Label)
	}

	if nas.GmmMessage.ServiceRequest.ServiceTypeAndNgksi.TSC.Value != 0 {
		t.Errorf("Unexpected TSC value: got %d", nas.GmmMessage.ServiceRequest.ServiceTypeAndNgksi.TSC.Value)
	}

	if nas.GmmMessage.ServiceRequest.ServiceTypeAndNgksi.NasKeySetIdentifiler != 0 {
		t.Errorf("Unexpected NasKeySetIdentifiler value: got %d", nas.GmmMessage.ServiceRequest.ServiceTypeAndNgksi.NasKeySetIdentifiler)
	}
}
