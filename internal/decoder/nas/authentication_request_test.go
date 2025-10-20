package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
	naslib "github.com/omec-project/nas"
)

func TestDecodeNASMessage_AuthenticationRequest(t *testing.T) {
	const message = "fgBWAAIAACEaBwCjbSa9vkiAkRdky8+5IBBH2jhAU2SAAE2CgCRBSs2H"

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

	if nas.GmmMessage.GmmHeader.MessageType.Label != "AuthenticationRequest" {
		t.Errorf("Unexpected GmmMessage Type: got %v", nas.GmmMessage.GmmHeader.MessageType.Label)
	}

	if nas.GmmMessage.GmmHeader.MessageType.Value != naslib.MsgTypeAuthenticationRequest {
		t.Errorf("Unexpected GmmMessage Type value: got %d", nas.GmmMessage.GmmHeader.MessageType.Value)
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
