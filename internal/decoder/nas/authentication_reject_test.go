package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
	naslib "github.com/omec-project/nas"
)

func TestDecodeNASMessage_AuthenticationReject(t *testing.T) {
	const message = "fgBY"

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	nas := nas.DecodeNASMessage(raw)

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

	if nas.GmmMessage.GmmHeader.MessageType.Label != "AuthenticationReject" {
		t.Errorf("Unexpected GmmMessage Type: got %v", nas.GmmMessage.GmmHeader.MessageType.Label)
	}

	if nas.GmmMessage.GmmHeader.MessageType.Value != naslib.MsgTypeAuthenticationReject {
		t.Errorf("Unexpected GmmMessage Type value: got %d", nas.GmmMessage.GmmHeader.MessageType.Value)
	}

	if nas.GmmMessage.AuthenticationReject == nil {
		t.Fatal("AuthenticationReject is nil")
	}
}
