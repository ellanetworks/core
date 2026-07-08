// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"go.uber.org/zap"
)

// newDecoderTestUE returns a UE in the "registered with valid security
// context" state, attached to a fresh UeConn.
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
		radioName:   radio.name,
		amf:         radio.amf,
		RanUeNgapID: 1,
		AmfUeNgapID: 1,
		Log:         zap.NewNop(),
	}
	ueConn.amf.AttachUeConn(ue, ueConn)

	return ue
}

func encodePlainServiceRequest(t *testing.T) []byte {
	t.Helper()

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeServiceRequest)

	sr := nasMessage.NewServiceRequest(0)
	sr.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	sr.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	sr.SetSpareHalfOctet(0)
	sr.SetMessageType(nas.MsgTypeServiceRequest)
	sr.SetServiceTypeValue(nasMessage.ServiceTypeSignalling)
	sr.SetNasKeySetIdentifiler(1)
	sr.TMSI5GS.SetLen(7)
	sr.SetTypeOfIdentity(4) // 5G-S-TMSI
	sr.SetAMFPointer(0)
	sr.SetAMFSetID(0)
	sr.SetTMSI5G([4]uint8{0xDE, 0xAD, 0xBE, 0xEF})

	m.ServiceRequest = sr

	payload, err := m.PlainNasEncode()
	if err != nil {
		t.Fatalf("encode plain ServiceRequest: %v", err)
	}

	return payload
}

func encodePlainULNasTransport(t *testing.T) []byte {
	t.Helper()

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeULNASTransport)

	ul := nasMessage.NewULNASTransport(0)
	ul.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	ul.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	ul.SetSpareHalfOctet(0)
	ul.SetMessageType(nas.MsgTypeULNASTransport)
	ul.SetPayloadContainerType(nasMessage.PayloadContainerTypeN1SMInfo)
	ul.PayloadContainer.SetLen(1)
	ul.SetPayloadContainerContents([]byte{0x00})

	m.ULNASTransport = ul

	payload, err := m.PlainNasEncode()
	if err != nil {
		t.Fatalf("encode plain ULNasTransport: %v", err)
	}

	return payload
}

func encodePlainDeregistrationRequest(t *testing.T) []byte {
	t.Helper()

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration)

	dr := nasMessage.NewDeregistrationRequestUEOriginatingDeregistration(0)
	dr.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	dr.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	dr.SetSpareHalfOctet(0)
	dr.SetMessageType(nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration)
	dr.SetSwitchOff(0)
	dr.SetReRegistrationRequired(0)
	dr.SetAccessType(nasMessage.AccessType3GPP)
	dr.SetNasKeySetIdentifiler(0)
	dr.MobileIdentity5GS = nasType.MobileIdentity5GS{
		Iei:    0,
		Len:    11,
		Buffer: make([]uint8, 11),
	}

	m.DeregistrationRequestUEOriginatingDeregistration = dr

	payload, err := m.PlainNasEncode()
	if err != nil {
		t.Fatalf("encode plain DeregistrationRequest: %v", err)
	}

	return payload
}

func encodePlainRegistrationRequest(t *testing.T) []byte {
	t.Helper()

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeRegistrationRequest)

	rr := nasMessage.NewRegistrationRequest(0)
	rr.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	rr.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	rr.SetSpareHalfOctet(0)
	rr.SetMessageType(nas.MsgTypeRegistrationRequest)
	rr.NgksiAndRegistrationType5GS.SetNasKeySetIdentifiler(0)
	rr.SetRegistrationType5GS(nasMessage.RegistrationType5GSInitialRegistration)
	rr.SetFOR(1)
	rr.MobileIdentity5GS = nasType.MobileIdentity5GS{
		Iei:    nasMessage.MobileIdentity5GSType5gGuti,
		Len:    11,
		Buffer: make([]uint8, 11),
	}
	rr.UESecurityCapability = &nasType.UESecurityCapability{}

	m.RegistrationRequest = rr

	payload, err := m.PlainNasEncode()
	if err != nil {
		t.Fatalf("encode plain RegistrationRequest: %v", err)
	}

	return payload
}

// TestDecodeNASMessage_PlainServiceRequestRejected verifies a plain
// ServiceRequest is rejected by the decoder (TS 24.501).
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

// A plain message whose type octet is readable but whose body cannot be decoded is a protocol
// error: the decoder resolves it to a 5GMM STATUS #96 disposition (TS 24.501 §7.5.1), so the
// finalizer answers rather than dropping it.
func TestDecodeNASMessage_MalformedPlain_YieldsStatus96(t *testing.T) {
	ue := newDecoderTestUE(t)
	ue.secured = false // fresh UE: the plain path is taken

	// EPD, plain security header, REGISTRATION REQUEST type — then truncated (no mandatory IEs).
	_, err := DecodeNASMessage(ue, []byte{0x7e, 0x00, nas.MsgTypeRegistrationRequest})
	if err == nil {
		t.Fatal("expected a decode error for a truncated registration request")
	}

	d := DispositionForDecodeError(err)
	if d.Action != nasreply.ActionStatus || d.Domain != nasreply.DomainMM || d.Cause != nasreply.CauseInvalidMandatoryInfo {
		t.Errorf("disposition = %+v, want a 5GMM STATUS #96 (invalid mandatory information)", d)
	}
}

// A message the decoder discards for a security reason resolves to a silent-discard
// disposition — the network must never answer forged or non-exempt plain NAS
// (TS 24.501 §4.4.4.3).
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

// TestDecodeNASMessage_PlainULNasTransportRejected verifies a plain
// ULNasTransport is rejected by the decoder.
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

// TestDecodeNASMessage_PlainRegistrationRequest_Bootstrap verifies the
// decoder admits a plain REGISTRATION REQUEST for a fresh UE and does not
// mutate security state.
func TestDecodeNASMessage_PlainRegistrationRequest_Bootstrap(t *testing.T) {
	ue := newDecoderTestUE(t)
	ue.secured = false // fresh UE
	payload := encodePlainRegistrationRequest(t)

	result, err := DecodeNASMessage(ue, payload)
	if err != nil {
		t.Fatalf("plain RegistrationRequest must be accepted during bootstrap: %v", err)
	}

	if result == nil || result.Message == nil || result.Message.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationRequest {
		t.Fatalf("expected RegistrationRequest, got %+v", result)
	}

	if result.IntegrityVerified {
		t.Errorf("expected a plain NAS message to be not integrity-verified")
	}

	if ue.secured {
		t.Error("a fresh UE must still have SecurityContextAvailable=false after the decoder runs")
	}
}

// TestDecodeNASMessage_PlainRegistrationRequest_WithExistingContext
// verifies the decoder admits a plain REGISTRATION REQUEST for a UE that still
// has a stored security context and leaves all security state untouched.
func TestDecodeNASMessage_PlainRegistrationRequest_WithExistingContext(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := encodePlainRegistrationRequest(t)

	result, err := DecodeNASMessage(ue, payload)
	if err != nil {
		t.Fatalf("plain RegistrationRequest must be accepted: %v", err)
	}

	if result.Message.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationRequest {
		t.Fatalf("expected RegistrationRequest, got %d", result.Message.GmmHeader.GetMessageType())
	}

	if result.IntegrityVerified {
		t.Errorf("expected a plain NAS message to be not integrity-verified")
	}

	if !ue.secured {
		t.Error("decoder must NOT clear SecurityContextAvailable; that is the handler's job")
	}
}

// TestDecodeNASMessage_PlainDeregistrationRequest_PassesDecoder verifies
// a plain DeregistrationRequest is accepted by the decoder (it is on the
// whitelist).
func TestDecodeNASMessage_PlainDeregistrationRequest_PassesDecoder(t *testing.T) {
	ue := newDecoderTestUE(t)
	payload := encodePlainDeregistrationRequest(t)

	result, err := DecodeNASMessage(ue, payload)
	if err != nil {
		t.Fatalf("plain DeregistrationRequest is on the whitelist; decoder must return it: %v", err)
	}

	if result.Message.GmmHeader.GetMessageType() != nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration {
		t.Fatalf("expected DeregistrationRequest, got %d", result.Message.GmmHeader.GetMessageType())
	}

	if result.IntegrityVerified {
		t.Errorf("expected a plain NAS message to be not integrity-verified")
	}

	if !ue.secured {
		t.Error("decoder must NOT clear SecurityContextAvailable")
	}
}
