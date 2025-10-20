package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
)

func TestDecodeNASMessage_AuthenticationFailure(t *testing.T) {
	const message = "fgBZFA=="

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
