// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func newDecoderTestUE(t *testing.T) *UeContext {
	t.Helper()

	ue := NewUeContext()
	ue.secured = true

	radio := &Radio{
		name: "test-gNB",
		Log:  zap.NewNop(),
	}
	radio.BindAMFForTest(New(nil, nil, nil))

	ueConn := &UeConn{
		conn:        radio.Conn,
		amf:         radio.amf,
		RanUeNgapID: 1,
		AmfUeNgapID: 1,
	}
	ueConn.setRadio("", radio.name)
	ueConn.setLog(zap.NewNop())
	ueConn.amf.AttachUeConn(ue, ueConn)

	return ue
}

func encodePlainServiceRequest(t *testing.T) []byte {
	t.Helper()

	m := &fgs.ServiceRequest{
		ServiceType:    fgs.ServiceTypeSignalling,
		NgKSI:          nas.KeySetIdentifier{Value: 1},
		MobileIdentity: fgs.STMSIIdentity(fgs.STMSI{TMSI: [4]byte{0xDE, 0xAD, 0xBE, 0xEF}}),
	}

	payload, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain ServiceRequest: %v", err)
	}

	return payload
}

func encodePlainULNasTransport(t *testing.T) []byte {
	t.Helper()

	m := &fgs.ULNASTransport{
		PayloadContainerType: fgs.PayloadContainerTypeN1SMInfo,
		PayloadContainer:     []byte{0x00},
	}

	payload, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain ULNasTransport: %v", err)
	}

	return payload
}

func encodePlainDeregistrationRequest(t *testing.T) []byte {
	t.Helper()

	m := &fgs.DeregistrationRequestUEOriginating{
		AccessType:     1,
		MobileIdentity: testMobileIdentity(),
	}

	payload, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain DeregistrationRequest: %v", err)
	}

	return payload
}

func encodePlainRegistrationRequest(t *testing.T) []byte {
	t.Helper()

	m := &fgs.RegistrationRequest{
		RegistrationType:     fgs.RegistrationTypeInitial,
		FOR:                  true,
		MobileIdentity:       testMobileIdentity(),
		UESecurityCapability: &fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0},
	}

	payload, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain RegistrationRequest: %v", err)
	}

	return payload
}

func TestDecodeNASMessage_PlainServiceRequestRejected(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := encodePlainServiceRequest(t)

	result, err := DecodeNASMessage(ue, payload)
	if err == nil {
		t.Fatalf("expected error, got result=%v", result)
	}

	if !strings.Contains(err.Error(), "not permitted by TS 24.501") {
		t.Errorf("expected TS 24.501 rejection, got: %v", err)
	}

	if !ue.secured {
		t.Error("decoder must NOT tear down SecurityContextAvailable on a hostile plain NAS message (DoS amplification)")
	}
}

// TS 24.501 §7.5.1
func TestDecodeNASMessage_MalformedPlain_YieldsStatus96(t *testing.T) {
	ue := newDecoderTestUE(t)
	ue.secured = false

	_, err := DecodeNASMessage(ue, []byte{0x7e, 0x00, uint8(fgs.MsgRegistrationRequest)})
	if err == nil {
		t.Fatal("expected a decode error for a truncated registration request")
	}

	d := DispositionForDecodeError(err)
	if d.Action != nasreply.ActionStatus || d.Domain != nasreply.DomainMM || d.Cause != nasreply.CauseInvalidMandatoryInfo {
		t.Errorf("disposition = %+v, want a 5GMM STATUS #96 (invalid mandatory information)", d)
	}
}

// TS 24.501 §7.4
func TestDecodeNASMessage_UnknownType_YieldsStatus97(t *testing.T) {
	ue := newDecoderTestUE(t)
	ue.secured = false

	_, err := DecodeNASMessage(ue, []byte{0x7e, 0x00, 0xff})
	if err == nil {
		t.Fatal("expected a decode error for an unknown message type")
	}

	d := DispositionForDecodeError(err)
	if d.Action != nasreply.ActionStatus || d.Domain != nasreply.DomainMM || d.Cause != nasreply.CauseMessageTypeNotImplemented {
		t.Errorf("disposition = %+v, want a 5GMM STATUS #97 (message type non-existent or not implemented)", d)
	}
}

func TestGmmDecodeFailureCause(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want uint8
	}{
		{"unknown type 0xff", []byte{0x7e, 0x00, 0xff}, nasreply.CauseMessageTypeNotImplemented},
		{"defined uplink type, malformed body", []byte{0x7e, 0x00, uint8(fgs.MsgRegistrationRequest)}, nasreply.CauseInvalidMandatoryInfo},
		{"downlink-only type on uplink", []byte{0x7e, 0x00, uint8(fgs.MsgRegistrationAccept)}, nasreply.CauseMessageTypeNotImplemented},
		{"too short to carry a type", []byte{0x7e, 0x00}, nasreply.CauseInvalidMandatoryInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GmmDecodeFailureCause(tt.body); got != tt.want {
				t.Errorf("GmmDecodeFailureCause(%x) = %d, want %d", tt.body, got, tt.want)
			}
		})
	}
}

// TS 24.501 §4.4.4.3
func TestDecodeNASMessage_PlainRejected_YieldsSilent(t *testing.T) {
	ue := newDecoderTestUE(t)

	_, err := DecodeNASMessage(ue, encodePlainServiceRequest(t))
	if err == nil {
		t.Fatal("expected a decode error for a plain service request on a secured UE")
	}

	if d := DispositionForDecodeError(err); d.Action != nasreply.ActionSilent {
		t.Errorf("disposition = %+v, want a silent discard", d)
	}
}

func TestDecodeNASMessage_PlainULNasTransportRejected(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := encodePlainULNasTransport(t)

	result, err := DecodeNASMessage(ue, payload)
	if err == nil {
		t.Fatalf("expected error, got result=%v", result)
	}

	if !strings.Contains(err.Error(), "not permitted by TS 24.501") {
		t.Errorf("expected TS 24.501 rejection, got: %v", err)
	}

	if !ue.secured {
		t.Error("decoder must NOT tear down SecurityContextAvailable on a hostile plain NAS message")
	}
}

func TestDecodeNASMessage_PlainRegistrationRequest_Bootstrap(t *testing.T) {
	ue := newDecoderTestUE(t)
	ue.secured = false
	payload := encodePlainRegistrationRequest(t)

	result, err := DecodeNASMessage(ue, payload)
	if err != nil {
		t.Fatalf("plain RegistrationRequest must be accepted during bootstrap: %v", err)
	}

	if result == nil || !result.IsGMM || result.MessageType != uint8(fgs.MsgRegistrationRequest) {
		t.Fatalf("expected RegistrationRequest, got %+v", result)
	}

	if result.IntegrityVerified {
		t.Errorf("expected a plain NAS message to be not integrity-verified")
	}

	if ue.secured {
		t.Error("a fresh UE must still have SecurityContextAvailable=false after the decoder runs")
	}
}

func TestDecodeNASMessage_PlainRegistrationRequest_WithExistingContext(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := encodePlainRegistrationRequest(t)

	result, err := DecodeNASMessage(ue, payload)
	if err != nil {
		t.Fatalf("plain RegistrationRequest must be accepted: %v", err)
	}

	if result.MessageType != uint8(fgs.MsgRegistrationRequest) {
		t.Fatalf("expected RegistrationRequest, got %d", result.MessageType)
	}

	if result.IntegrityVerified {
		t.Errorf("expected a plain NAS message to be not integrity-verified")
	}

	if !ue.secured {
		t.Error("decoder must NOT clear SecurityContextAvailable; that is the handler's job")
	}
}

func TestDecodeNASMessage_PlainDeregistrationRequest_PassesDecoder(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := encodePlainDeregistrationRequest(t)

	result, err := DecodeNASMessage(ue, payload)
	if err != nil {
		t.Fatalf("plain DeregistrationRequest is on the whitelist; decoder must return it: %v", err)
	}

	if result.MessageType != uint8(fgs.MsgDeregistrationRequestUEOrig) {
		t.Fatalf("expected DeregistrationRequest, got %d", result.MessageType)
	}

	if result.IntegrityVerified {
		t.Errorf("expected a plain NAS message to be not integrity-verified")
	}

	if !ue.secured {
		t.Error("decoder must NOT clear SecurityContextAvailable")
	}
}

func wrapProtectedNAS(t *testing.T, sht fgs.SecurityHeaderType, inner []byte) []byte {
	t.Helper()

	pdu := []byte{0x7e, byte(sht), 0xde, 0xad, 0xbe, 0xef, 0x01}

	return append(pdu, inner...)
}

func TestDecodeNASMessage_ProtectedRegistrationRequest_NoSecurityContext_Admitted(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := wrapProtectedNAS(t, fgs.SHTIntegrityProtected, encodePlainRegistrationRequest(t))

	result, err := DecodeNASMessage(ue, payload)
	if err != nil {
		t.Fatalf("TS 24.501 §4.4.4.3 requires a REGISTRATION REQUEST to be processed when the security context is not available in the network: %v", err)
	}

	if !result.IsGMM || result.MessageType != uint8(fgs.MsgRegistrationRequest) {
		t.Fatalf("expected RegistrationRequest, got %+v", result)
	}

	if result.IntegrityVerified {
		t.Error("a message whose MAC could not be verified must not be reported integrity-verified")
	}

	if ue.Conn().SecureExchangeEstablished() {
		t.Error("an unverified message must not establish secure exchange")
	}
}

func TestDecodeNASMessage_ProtectedULNasTransport_NoSecurityContext_Dropped(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := wrapProtectedNAS(t, fgs.SHTIntegrityProtected, encodePlainULNasTransport(t))

	result, err := DecodeNASMessage(ue, payload)
	if err == nil {
		t.Fatalf("a message outside the TS 24.501 §4.4.4.3 list must not be admitted without a security context, got %+v", result)
	}

	if d := DispositionForDecodeError(err); d.Action != nasreply.ActionSilent {
		t.Errorf("disposition = %+v, want a silent discard", d)
	}
}

func TestDecodeNASMessage_ProtectedCipheredRegistrationRequest_NoSecurityContext_Dropped(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := wrapProtectedNAS(t, fgs.SHTIntegrityProtectedCiphered, encodePlainRegistrationRequest(t))

	result, err := DecodeNASMessage(ue, payload)
	if err == nil {
		t.Fatalf("a ciphered body cannot be read without a security context, got %+v", result)
	}

	if d := DispositionForDecodeError(err); d.Action != nasreply.ActionSilent {
		t.Errorf("disposition = %+v, want a silent discard", d)
	}
}
