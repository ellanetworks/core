package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
)

func TestDecodeNASMessage_AuthenticationReject(t *testing.T) {
	const message = "fgBY"

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
