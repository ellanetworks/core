package nas_test

import (
	"encoding/base64"
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
	naslib "github.com/free5gc/nas"
)

func TestDecodeNASMessage_IntegrityProtectedNotCiphered(t *testing.T) {
	// Take a known plain NAS RegistrationRequest and wrap it in an
	// integrity-protected (but NOT ciphered) security header.
	//
	// Security-protected NAS format (TS 24.501 section 9.1):
	//   Byte 0:    EPD (0x7e = 5GMM)
	//   Byte 1:    Security Header Type (0x01 = integrity protected)
	//   Bytes 2-5: Message Authentication Code
	//   Byte 6:    Sequence Number
	//   Bytes 7+:  Plain NAS message
	plainNASB64 := "fgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8A=="

	plainNAS, err := base64.StdEncoding.DecodeString(plainNASB64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	// Build a security-protected wrapper
	securityHeader := []byte{
		0x7e,                   // EPD: 5GMM
		0x01,                   // Security Header Type: Integrity Protected
		0xDE, 0xAD, 0xBE, 0xEF, // MAC
		0x05, // Sequence Number
	}
	raw := append(securityHeader, plainNAS...)

	nasMsg := nas.DecodeNASMessage(raw)
	if nasMsg == nil {
		t.Fatal("decoded NAS message is nil")
	}

	if nasMsg.Encrypted {
		t.Error("message should NOT be marked as encrypted (integrity-only)")
	}

	if nasMsg.Error != "" {
		t.Errorf("unexpected error: %s", nasMsg.Error)
	}

	if nasMsg.SecurityHeader.SecurityHeaderType.Value != naslib.SecurityHeaderTypeIntegrityProtected {
		t.Errorf("expected SecurityHeaderType=1, got %d", nasMsg.SecurityHeader.SecurityHeaderType.Value)
	}

	if nasMsg.SecurityHeader.SecurityHeaderType.Label != "Integrity Protected" {
		t.Errorf("expected label 'Integrity Protected', got %q", nasMsg.SecurityHeader.SecurityHeaderType.Label)
	}

	expectedMAC := uint32(0xDEADBEEF)
	if nasMsg.SecurityHeader.MessageAuthenticationCode != expectedMAC {
		t.Errorf("expected MAC=0x%08X, got 0x%08X", expectedMAC, nasMsg.SecurityHeader.MessageAuthenticationCode)
	}

	if nasMsg.SecurityHeader.SequenceNumber != 0x05 {
		t.Errorf("expected SequenceNumber=5, got %d", nasMsg.SecurityHeader.SequenceNumber)
	}

	// The inner NAS should be decoded as a RegistrationRequest
	if nasMsg.GmmMessage == nil {
		t.Fatal("GmmMessage is nil — inner NAS was not decoded")
	}

	if nasMsg.GmmMessage.GmmHeader.MessageType.Label != "RegistrationRequest" {
		t.Errorf("expected RegistrationRequest, got %s", nasMsg.GmmMessage.GmmHeader.MessageType.Label)
	}

	if nasMsg.GmmMessage.RegistrationRequest == nil {
		t.Fatal("RegistrationRequest is nil")
	}

	if nasMsg.GmmMessage.RegistrationRequest.MobileIdentity5GS.SUCI == nil {
		t.Fatal("SUCI is nil")
	}

	expectedSuci := "suci-0-001-01-0000-0-0-4447867552"
	if *nasMsg.GmmMessage.RegistrationRequest.MobileIdentity5GS.SUCI != expectedSuci {
		t.Errorf("expected SUCI=%s, got %s", expectedSuci, *nasMsg.GmmMessage.RegistrationRequest.MobileIdentity5GS.SUCI)
	}
}

func TestDecodeNASMessage_IntegrityProtectedWithNewContext(t *testing.T) {
	// Same test but with security header type 3 (integrity protected with new 5G NAS security context).
	// This should also be decoded since it's NOT ciphered.
	plainNASB64 := "fgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8A=="

	plainNAS, err := base64.StdEncoding.DecodeString(plainNASB64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	securityHeader := []byte{
		0x7e,                   // EPD: 5GMM
		0x03,                   // Security Header Type: Integrity Protected with New 5G NAS Security Context
		0x11, 0x22, 0x33, 0x44, // MAC
		0x0A, // Sequence Number
	}
	raw := append(securityHeader, plainNAS...)

	nasMsg := nas.DecodeNASMessage(raw)
	if nasMsg == nil {
		t.Fatal("decoded NAS message is nil")
	}

	if nasMsg.Encrypted {
		t.Error("message should NOT be marked as encrypted (integrity-only with new context)")
	}

	if nasMsg.Error != "" {
		t.Errorf("unexpected error: %s", nasMsg.Error)
	}

	if nasMsg.SecurityHeader.SecurityHeaderType.Value != naslib.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext {
		t.Errorf("expected SecurityHeaderType=3, got %d", nasMsg.SecurityHeader.SecurityHeaderType.Value)
	}

	expectedMAC := uint32(0x11223344)
	if nasMsg.SecurityHeader.MessageAuthenticationCode != expectedMAC {
		t.Errorf("expected MAC=0x%08X, got 0x%08X", expectedMAC, nasMsg.SecurityHeader.MessageAuthenticationCode)
	}

	if nasMsg.SecurityHeader.SequenceNumber != 0x0A {
		t.Errorf("expected SequenceNumber=10, got %d", nasMsg.SecurityHeader.SequenceNumber)
	}

	if nasMsg.GmmMessage == nil {
		t.Fatal("GmmMessage is nil — inner NAS was not decoded")
	}

	if nasMsg.GmmMessage.GmmHeader.MessageType.Label != "RegistrationRequest" {
		t.Errorf("expected RegistrationRequest, got %s", nasMsg.GmmMessage.GmmHeader.MessageType.Label)
	}
}

func TestDecodeNASMessage_CipheredNotDecoded(t *testing.T) {
	// Security header type 2 (integrity protected AND ciphered) should NOT be decoded.
	securityHeader := []byte{
		0x7e,                   // EPD: 5GMM
		0x02,                   // Security Header Type: Integrity Protected and Ciphered
		0xAA, 0xBB, 0xCC, 0xDD, // MAC
		0x01,                   // Sequence Number
		0xFF, 0xFF, 0xFF, 0xFF, // Encrypted payload (garbage)
	}

	nasMsg := nas.DecodeNASMessage(securityHeader)
	if nasMsg == nil {
		t.Fatal("decoded NAS message is nil")
	}

	if !nasMsg.Encrypted {
		t.Error("ciphered message should be marked as encrypted")
	}

	if nasMsg.SecurityHeader.SecurityHeaderType.Value != naslib.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Errorf("expected SecurityHeaderType=2, got %d", nasMsg.SecurityHeader.SecurityHeaderType.Value)
	}

	expectedMAC := uint32(0xAABBCCDD)
	if nasMsg.SecurityHeader.MessageAuthenticationCode != expectedMAC {
		t.Errorf("expected MAC=0x%08X, got 0x%08X", expectedMAC, nasMsg.SecurityHeader.MessageAuthenticationCode)
	}

	if nasMsg.SecurityHeader.SequenceNumber != 0x01 {
		t.Errorf("expected SequenceNumber=1, got %d", nasMsg.SecurityHeader.SequenceNumber)
	}

	if nasMsg.GmmMessage != nil {
		t.Error("ciphered message should not have decoded GmmMessage")
	}
}
