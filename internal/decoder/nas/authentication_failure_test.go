package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
	naslib "github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
)

func TestDecodeNASMessage_AuthenticationFailure(t *testing.T) {
	const message = "fgBZFA=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	nasMsg := nas.DecodeNASMessage(raw)

	if nasMsg == nil {
		t.Fatal("Decoded NAS message is nil")
	}

	if nasMsg.SecurityHeader.SecurityHeaderType.Label != "Plain NAS" {
		t.Errorf("Unexpected SecurityHeaderType: got %v", nasMsg.SecurityHeader.SecurityHeaderType.Label)
	}

	if nasMsg.SecurityHeader.SecurityHeaderType.Value != naslib.SecurityHeaderTypePlainNas {
		t.Errorf("Unexpected SecurityHeaderType value: got %d", nasMsg.SecurityHeader.SecurityHeaderType.Value)
	}

	if nasMsg.GsmMessage != nil {
		t.Fatal("GsmMessage is not nil")
	}

	if nasMsg.GmmMessage == nil {
		t.Fatal("GmmMessage is nil")
	}

	if nasMsg.GmmMessage.GmmHeader.MessageType.Label != "AuthenticationFailure" {
		t.Errorf("Unexpected GmmMessage Type: got %v", nasMsg.GmmMessage.GmmHeader.MessageType.Label)
	}

	if nasMsg.GmmMessage.GmmHeader.MessageType.Value != naslib.MsgTypeAuthenticationFailure {
		t.Errorf("Unexpected GmmMessage Type value: got %d", nasMsg.GmmMessage.GmmHeader.MessageType.Value)
	}

	if nasMsg.GmmMessage.AuthenticationFailure == nil {
		t.Fatal("AuthenticationFailure is nil")
	}

	if nasMsg.GmmMessage.AuthenticationFailure.Cause5GMM.Label != "MAC failure" {
		t.Errorf("Unexpected Cause5GMM: got %v, want 'MAC failure'", nasMsg.GmmMessage.AuthenticationFailure.Cause5GMM)
	}

	if nasMsg.GmmMessage.AuthenticationFailure.Cause5GMM.Value != nasMessage.Cause5GMMMACFailure {
		t.Errorf("Unexpected Cause5GMM value: got %d, want %d", nasMsg.GmmMessage.AuthenticationFailure.Cause5GMM.Value, nasMessage.Cause5GMMMACFailure)
	}
}
