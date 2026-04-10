// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"strings"
	"testing"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"go.uber.org/zap"
)

// newDecoderTestUE returns a UE in the "registered with valid security
// context" state, attached to a fresh RanUe carrying the given
// RRCEstablishmentCause.
func newDecoderTestUE(t *testing.T, rrcCause string) *AmfUe {
	t.Helper()

	ue := NewAmfUe()
	ue.Log = zap.NewNop()
	ue.SecurityContextAvailable = true
	ue.MacFailed = false

	radio := &Radio{
		Name:   "test-gNB",
		RanUEs: make(map[int64]*RanUe),
		Log:    zap.NewNop(),
	}
	ranUe := &RanUe{
		Radio:                 radio,
		RanUeNgapID:           1,
		AmfUeNgapID:           1,
		Log:                   zap.NewNop(),
		RRCEstablishmentCause: rrcCause,
	}
	ue.AttachRanUe(ranUe)

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
// ServiceRequest is rejected for every value of RRCEstablishmentCause
// (TS 24.501 §4.4.4.3).
func TestDecodeNASMessage_PlainServiceRequestRejected(t *testing.T) {
	for _, cause := range []string{"", "0", "1", "2", "3"} {
		t.Run("RRCEstablishmentCause="+cause, func(t *testing.T) {
			ue := newDecoderTestUE(t, cause)
			payload := encodePlainServiceRequest(t)

			msg, err := ue.DecodeNASMessage(payload)
			if err == nil {
				t.Fatalf("expected error, got msg=%v", msg)
			}

			if !strings.Contains(err.Error(), "not permitted by TS 24.501") {
				t.Errorf("expected TS 24.501 §4.4.4.3 rejection, got: %v", err)
			}

			if !ue.SecurityContextAvailable {
				t.Error("decoder must NOT tear down SecurityContextAvailable on a hostile plain NAS message (DoS amplification)")
			}
		})
	}
}

// TestDecodeNASMessage_PlainULNasTransportRejected verifies a plain
// ULNasTransport is rejected by the decoder.
func TestDecodeNASMessage_PlainULNasTransportRejected(t *testing.T) {
	ue := newDecoderTestUE(t, "0")
	payload := encodePlainULNasTransport(t)

	msg, err := ue.DecodeNASMessage(payload)
	if err == nil {
		t.Fatalf("expected error, got msg=%v", msg)
	}

	if !strings.Contains(err.Error(), "not permitted by TS 24.501") {
		t.Errorf("expected TS 24.501 §4.4.4.3 rejection, got: %v", err)
	}

	if !ue.SecurityContextAvailable {
		t.Error("decoder must NOT tear down SecurityContextAvailable on a hostile plain NAS message")
	}
}

// TestDecodeNASMessage_PlainRegistrationRequest_Bootstrap verifies the
// decoder accepts a plain RegistrationRequest from a fresh UE and marks
// it MacFailed.
func TestDecodeNASMessage_PlainRegistrationRequest_Bootstrap(t *testing.T) {
	ue := newDecoderTestUE(t, "1")
	ue.SecurityContextAvailable = false // fresh UE
	payload := encodePlainRegistrationRequest(t)

	msg, err := ue.DecodeNASMessage(payload)
	if err != nil {
		t.Fatalf("plain RegistrationRequest must be accepted during bootstrap: %v", err)
	}

	if msg == nil || msg.GmmMessage == nil || msg.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationRequest {
		t.Fatalf("expected RegistrationRequest, got %+v", msg)
	}

	if !ue.MacFailed {
		t.Error("decoder must mark plain NAS as MacFailed=true so handlers know it is unauthenticated")
	}

	if ue.SecurityContextAvailable {
		t.Error("a fresh UE must still have SecurityContextAvailable=false after the decoder runs")
	}
}

// TestDecodeNASMessage_PlainRegistrationRequest_WithExistingContext
// verifies the decoder accepts a plain RegistrationRequest for a UE that
// still has a stored security context, marks it MacFailed, and leaves
// the security context for the handler to clear.
func TestDecodeNASMessage_PlainRegistrationRequest_WithExistingContext(t *testing.T) {
	ue := newDecoderTestUE(t, "1")
	payload := encodePlainRegistrationRequest(t)

	msg, err := ue.DecodeNASMessage(payload)
	if err != nil {
		t.Fatalf("plain RegistrationRequest must be accepted: %v", err)
	}

	if msg.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationRequest {
		t.Fatalf("expected RegistrationRequest, got %d", msg.GmmHeader.GetMessageType())
	}

	if !ue.MacFailed {
		t.Error("decoder must mark plain NAS as MacFailed=true")
	}

	if !ue.SecurityContextAvailable {
		t.Error("decoder must NOT clear SecurityContextAvailable; that is the handler's job")
	}
}

// TestDecodeNASMessage_PlainDeregistrationRequest_PassesDecoder verifies
// a plain DeregistrationRequest is accepted by the decoder (it is on the
// §4.4.4.3 whitelist).
func TestDecodeNASMessage_PlainDeregistrationRequest_PassesDecoder(t *testing.T) {
	ue := newDecoderTestUE(t, "0")
	payload := encodePlainDeregistrationRequest(t)

	msg, err := ue.DecodeNASMessage(payload)
	if err != nil {
		t.Fatalf("plain DeregistrationRequest is on the §4.4.4.3 whitelist; decoder must return it: %v", err)
	}

	if msg.GmmHeader.GetMessageType() != nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration {
		t.Fatalf("expected DeregistrationRequest, got %d", msg.GmmHeader.GetMessageType())
	}

	if !ue.MacFailed {
		t.Error("decoder must mark plain NAS as MacFailed=true")
	}

	if !ue.SecurityContextAvailable {
		t.Error("decoder must NOT clear SecurityContextAvailable")
	}
}

// TestDecodeNASMessage_PlainNasIgnoresRRCEstablishmentCause verifies the
// decoder produces identical behaviour for every value of
// RRCEstablishmentCause.
func TestDecodeNASMessage_PlainNasIgnoresRRCEstablishmentCause(t *testing.T) {
	results := make(map[string]string)

	for _, cause := range []string{"", "0", "1", "2", "3", "4", "5", "6", "7"} {
		ue := newDecoderTestUE(t, cause)

		_, err := ue.DecodeNASMessage(encodePlainServiceRequest(t))
		if err == nil {
			t.Errorf("RRCEstablishmentCause=%q: expected error, got nil", cause)
			continue
		}

		results[cause] = err.Error()
	}

	first := ""
	for _, msg := range results {
		if first == "" {
			first = msg
			continue
		}

		if msg != first {
			t.Fatalf("decoder behaviour varies with RRCEstablishmentCause: %v", results)
		}
	}
}
