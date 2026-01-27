package gmm

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
	"go.uber.org/zap"
)

func TestGetRegistrationType5GSName(t *testing.T) {
	inputs := []uint8{
		0,
		nasMessage.RegistrationType5GSInitialRegistration,
		nasMessage.RegistrationType5GSMobilityRegistrationUpdating,
		nasMessage.RegistrationType5GSPeriodicRegistrationUpdating,
		nasMessage.RegistrationType5GSEmergencyRegistration,
		nasMessage.RegistrationType5GSReserved,
		127,
	}
	expected := []string{
		"Unknown",
		"Initial Registration",
		"Mobility Registration Updating",
		"Periodic Registration Updating",
		"Emergency Registration",
		"Reserved",
		"Unknown",
	}

	for i, input := range inputs {
		r := getRegistrationType5GSName(input)
		if r != expected[i] {
			t.Errorf("expected '%s' for code '%d' but got '%s", expected[i], input, r)
		}
	}
}

// TestHandleRegistrationRequest_NilRanUE validates the graceful
// handling of the degenerate case where RanUE is nil
func TestHandleRegistrationRequest_NilRanUE(t *testing.T) {
	ctx := context.TODO()
	amf := amfContext.AMF{}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	ue := amfContext.NewAmfUe()
	ue.RanUe = nil

	err = handleRegistrationRequest(ctx, &amf, ue, m)
	if err == nil {
		t.Fatalf("nil RanUe should error out")
	}
}

// TestHandleRegistrationRequest_ErrorMissingIdentity validates the graceful
// handling of the case where a UE sends a Registration Request missing
// Mobile Identity 5GS field
func TestHandleRegistrationRequest_ErrorMissingIdentity(t *testing.T) {
	ctx := context.TODO()
	amf := amfContext.AMF{}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	m.RegistrationRequest.MobileIdentity5GS = nasType.MobileIdentity5GS{}

	expected := "mobile identity 5GS is empty"

	err = handleRegistrationRequest(ctx, &amf, ue, m)
	if err == nil {
		t.Fatalf("registration request should be rejected")
	}

	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("expected '%s', got '%v'", expected, err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

// TestHandleRegistrationRequest_ErrorMissingOperatorInfo validates the graceful
// handling of the case where an operator is not configured.
func TestHandleRegistrationRequest_ErrorMissingOperatorInfo(t *testing.T) {
	ctx := context.TODO()
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: nil,
		},
	}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	expected := "error getting operator info"

	err = handleRegistrationRequest(ctx, amf, ue, m)
	if err == nil {
		t.Fatalf("registration request should be rejected")
	}

	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("expected '%s', got '%v'", expected, err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

// TestHandleRegistrationRequest_RejectTrackingAreaNotAllowed validates that a
// registration request for a non-allowed tracking area is rejected.
func TestHandleRegistrationRequest_RejectTrackingAreaNotAllowed(t *testing.T) {
	ctx := context.TODO()
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
	}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.RanUe.Tai = models.Tai{
		PlmnID: &models.PlmnID{
			Mcc: "999",
			Mnc: "99",
		},
		Tac: "42",
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	expected := "registration Reject [Tracking area not allowed]"

	err = handleRegistrationRequest(ctx, amf, ue, m)
	if err == nil {
		t.Fatalf("registration request should be rejected")
	}

	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("expected '%s', got '%v'", expected, err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message")
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationReject {
		t.Fatalf("expected a registration reject message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

// TestHandleRegistrationRequest_RejectMissingSecurityCapability validates that a
// registration request with missing UE Security Capability is rejected.
func TestHandleRegistrationRequest_RejectMissingSecurityCapability(t *testing.T) {
	ctx := context.TODO()
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
	}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	m.UESecurityCapability = nil

	expected := "registration request does not contain UE security capability for initial registration"

	err = handleRegistrationRequest(ctx, amf, ue, m)
	if err == nil {
		t.Fatalf("registration request should be rejected")
	}

	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("expected '%s', got '%v'", expected, err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message")
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationReject {
		t.Fatalf("expected a registration reject message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

// TestHandleRegistrationRequest_Timers_Stopped validates that the timers
// T3513 and T3565 are stopped when receiving a registration request.
func TestHandleRegistrationRequest_Timers_Stopped(t *testing.T) {
	ctx := context.TODO()
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
	}

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.T3513 = amfContext.NewTimer(10*time.Minute, 10, func(e int32) {}, func() {})
	ue.T3565 = amfContext.NewTimer(10*time.Minute, 10, func(e int32) {}, func() {})

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	err = handleRegistrationRequest(ctx, amf, ue, m)
	if err != nil {
		t.Fatalf("registration request should be accepted, got: %v", err)
	}

	if ue.T3513 != nil {
		t.Fatalf("timer T3513 should have been stopped")
	}

	if ue.T3565 != nil {
		t.Fatalf("timer T3565 should have been stopped")
	}
}

// TestHandleRegistrationRequest_IdentityRequest_MissingSUCI_SUPI validates that
// a registration request for a UE missing SUCI and SUPI triggers an
// Identity Request message.
func TestHandleRegistrationRequest_IdentityRequest_MissingSUCI_SUPI(t *testing.T) {
	ctx := context.TODO()
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
	}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	err = handleRegistrationRequest(ctx, amf, ue, m)
	if err != nil {
		t.Fatalf("registration request should be accepted, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message")
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeIdentityRequest {
		t.Fatalf("expected an identity request message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

// TestHandleRegistrationRequest_AuthenticationRequest validates that a
// Registration Request for a UE with proper identity but without a valid
// security context triggers an Authentication Request message.
func TestHandleRegistrationRequest_AuthenticationRequest(t *testing.T) {
	ctx := context.TODO()
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		Ausf: &FakeAusf{
			AvKgAka: &models.Av5gAka{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  "imsi-001019756139935",
			Kseaf: "testkey",
		},
	}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.Supi = "imsi-001019756139935"

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	err = handleRegistrationRequest(ctx, amf, ue, m)
	if err != nil {
		t.Fatalf("registration request should be accepted, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message")
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeAuthenticationRequest {
		t.Fatalf("expected an authentication request message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

// TestHandleRegistrationRequest_RegistrationAccepted validates that a
// Registration Request for a UE with valid security context is accepted
// with a properly ciphered message.
func TestHandleRegistrationRequest_RegistrationAccepted(t *testing.T) {
	ctx := context.TODO()
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		Ausf: &FakeAusf{
			AvKgAka: &models.Av5gAka{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  "imsi-001019756139935",
			Kseaf: "testkey",
		},
		UEs: make(map[string]*amfContext.AmfUe),
	}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.Supi = "imsi-001019756139935"
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	ue.MacFailed = false

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	err = handleRegistrationRequest(ctx, amf, ue, m)
	if err != nil {
		t.Fatalf("registration request should be accepted, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a protected and ciphered NAS message")
	}

	decodedMessage, err := ue.DecodeNASMessage(resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message")
	}

	if decodedMessage.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected a registration accept message, got '%v'", decodedMessage.GmmHeader.GetMessageType())
	}
}

// TestHandleRegistrationRequest_UEStateContextSetup_ResetToDeregistered validates
// that a Registration Request from a UE in the Context Setup state resets the
// UE's state to deregister, ignoring the request.
func TestHandleRegistrationRequest_UEStateContextSetup_ResetToDeregistered(t *testing.T) {
	ctx := context.TODO()
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
	}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.State = amfContext.ContextSetup

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	err = handleRegistrationRequest(ctx, amf, ue, m)
	if err != nil {
		t.Fatalf("registration request should be accepted, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}

	if ue.State != amfContext.Deregistered {
		t.Fatalf("state should be deregistered, got: %v", ue.State)
	}
}

// TestHandleRegistrationRequest_UEStateAuthentication_Error validates that
// a registration request for a UE in the middle of an Authentication procedure
// triggers an error.
func TestHandleRegistrationRequest_UEStateAuthentication_Error(t *testing.T) {
	ctx := context.TODO()
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
	}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.State = amfContext.Authentication

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	err = handleRegistrationRequest(ctx, amf, ue, m)
	if err == nil {
		t.Fatalf("registration request in state Authentication should return an error")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

// TestHandleRegistrationRequest_SecurityMode_AuthenticationRequest validates
// that a registration request coming in while the SecurityMode procedure is
// on-going resets the state of the UE to deregistered, triggering a new
// AuthenticationRequest.
func TestHandleRegistrationRequest_SecurityMode_AuthenticationRequest(t *testing.T) {
	ctx := context.TODO()
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		Ausf: &FakeAusf{
			AvKgAka: &models.Av5gAka{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  "imsi-001019756139935",
			Kseaf: "testkey",
		},
		UEs: make(map[string]*amfContext.AmfUe),
	}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.Supi = "imsi-001019756139935"
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	ue.MacFailed = false
	ue.State = amfContext.SecurityMode
	ue.T3560 = amfContext.NewTimer(10*time.Minute, 10, func(e int32) {}, func() {})

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	err = handleRegistrationRequest(ctx, amf, ue, m)
	if err != nil {
		t.Fatalf("registration request should be accepted, got: %v", err)
	}

	if ue.T3560 != nil {
		t.Fatalf("timer T3560 should be stopped")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message")
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeAuthenticationRequest {
		t.Fatalf("expected an authentication request message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

// TestHandleRegistrationRequest_CipheredNAS_RegistrationAccepted validates that
// a Registration Request with ciphered NAS is properly decrypted and accepted
// with a ciphered Registration Accept message.
func TestHandleRegistrationRequest_CipheredNAS_RegistrationAccepted(t *testing.T) {
	ctx := context.TODO()
	rand := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	autn := []byte{17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	supi := "imsi-001019756139935"
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		Ausf: &FakeAusf{
			AvKgAka: &models.Av5gAka{
				Rand: hex.EncodeToString(rand),
				Autn: hex.EncodeToString(autn),
			},
			Supi:  supi,
			Kseaf: "testkey",
		},
		UEs: make(map[string]*amfContext.AmfUe),
	}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.Supi = supi
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	ue.MacFailed = false

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	m, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCount.Get())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	err = handleRegistrationRequest(ctx, amf, ue, m)
	if err != nil {
		t.Fatalf("registration request should be accepted, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	payload := make([]byte, len(resp.NasPdu))
	copy(payload, resp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a protected and ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected a registration accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

// TestHandleRegistrationRequest_CipheredNAS_RegistrationRejectedWrongKey validates that
// a ciphered registration request with a wrong key is rejected with a plain NAS message.
func TestHandleRegistrationRequest_CipheredNAS_RegistrationRejectedWrongKey(t *testing.T) {
	ctx := context.TODO()
	rand := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	autn := []byte{17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	supi := "imsi-001019756139935"
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		Ausf: &FakeAusf{
			AvKgAka: &models.Av5gAka{
				Rand: hex.EncodeToString(rand),
				Autn: hex.EncodeToString(autn),
			},
			Supi:  supi,
			Kseaf: "testkey",
		},
		UEs: make(map[string]*amfContext.AmfUe),
	}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.Supi = supi
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	ue.MacFailed = false

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	m, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCount.Get())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	key = [16]uint8{0x00, 0x00, 0x00, 0x00, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	ue.KnasEnc = key

	err = handleRegistrationRequest(ctx, amf, ue, m)
	if err == nil {
		t.Fatalf("registration request should be rejected, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message")
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationReject {
		t.Fatalf("expected an registration reject message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

func buildTestRegistrationRequestMessage(cipherAlg uint8, key *[16]uint8, ulcount uint32) (*nas.GmmMessage, error) {
	m := nas.NewGmmMessage()

	registrationRequest := nasMessage.NewRegistrationRequest(0)
	registrationRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	registrationRequest.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	registrationRequest.SetSpareHalfOctet(0x00)
	registrationRequest.SetMessageType(nas.MsgTypeRegistrationRequest)
	registrationRequest.NgksiAndRegistrationType5GS.SetNasKeySetIdentifiler(uint8(0))
	registrationRequest.SetRegistrationType5GS(nasMessage.RegistrationType5GSInitialRegistration)
	registrationRequest.SetFOR(1)
	registrationRequest.MobileIdentity5GS = nasType.MobileIdentity5GS{
		Iei:    nasMessage.MobileIdentity5GSType5gGuti,
		Len:    15,
		Buffer: make([]uint8, 15),
	}
	registrationRequest.UESecurityCapability = &nasType.UESecurityCapability{}

	if key != nil {
		registrationRequest.UESecurityCapability = &nasType.UESecurityCapability{
			Iei:    nasMessage.RegistrationRequestUESecurityCapabilityType,
			Len:    2,
			Buffer: []uint8{0x00, 0x00},
		}
		registrationRequest.SetEA0_5G(1)
		registrationRequest.SetEA1_128_5G(1)
		registrationRequest.SetEA2_128_5G(1)
		registrationRequest.SetEA2_128_5G(0)
		registrationRequest.SetIA0_5G(1)
		registrationRequest.SetIA1_128_5G(1)
		registrationRequest.SetIA2_128_5G(1)
		registrationRequest.SetIA2_128_5G(0)
	}

	m.RegistrationRequest = registrationRequest
	m.SetMessageType(nas.MsgTypeRegistrationRequest)

	if key == nil {
		return m, nil
	}

	data := new(bytes.Buffer)

	err := m.EncodeRegistrationRequest(data)
	if err != nil {
		return nil, fmt.Errorf("could not encode registration request: %v", err)
	}

	nasPdu := data.Bytes()

	if err = security.NASEncrypt(cipherAlg, *key, ulcount, security.Bearer3GPP, security.DirectionUplink, nasPdu); err != nil {
		return nil, fmt.Errorf("could not encrypt NAS message: %v", err)
	}

	registrationRequest.NASMessageContainer = nasType.NewNASMessageContainer(nasMessage.RegistrationRequestNASMessageContainerType)
	registrationRequest.NASMessageContainer.SetLen(uint16(len(nasPdu)))
	registrationRequest.SetNASMessageContainerContents(nasPdu)
	registrationRequest.UplinkDataStatus = nil
	registrationRequest.PDUSessionStatus = nil

	return m, nil
}

func buildUeAndRadio() (*amfContext.AmfUe, *FakeNGAPSender, error) {
	ue := amfContext.NewAmfUe()

	ngapSender := FakeNGAPSender{}
	radio := amfContext.Radio{
		RanUEs:     make(map[int64]*amfContext.RanUe),
		Conn:       nil,
		Log:        logger.AmfLog.With(zap.String("ran_addr", "test_localhost")),
		NGAPSender: &ngapSender,
	}

	ranUe, err := radio.NewUe(0)
	ranUe.Tai = models.Tai{
		PlmnID: &models.PlmnID{
			Mcc: "001",
			Mnc: "01",
		},
		Tac: "000001",
	}

	if err != nil {
		return nil, nil, fmt.Errorf("could not create a new ranUe: %v", err)
	}

	ue.AttachRanUe(ranUe)

	return ue, &ngapSender, nil
}
