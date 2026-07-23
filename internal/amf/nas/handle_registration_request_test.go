// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
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
	amfInstance := amf.AMF{}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	ue := amf.NewUeContext()

	handleRegistrationRequest(ctx, &amfInstance, ue, regReqPlain(t, m), true)
}

// TestHandleRegistrationRequest_ErrorMissingIdentity validates the graceful
// handling of the case where a UE sends a Registration Request missing
// Mobile Identity 5GS field
func TestHandleRegistrationRequest_ErrorMissingIdentity(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.AMF{}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	m.RegistrationRequest.MobileIdentity5GS = nasType.MobileIdentity5GS{}

	handleRegistrationRequest(ctx, &amfInstance, ue, regReqPlain(t, m), true)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered, got %v", ue.State())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

// TestHandleRegistrationRequest_ErrorMissingOperatorInfo validates the graceful
// handling of the case where an operator is not configured.
func TestHandleRegistrationRequest_ErrorMissingOperatorInfo(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: nil,
	}, nil, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered, got %v", ue.State())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

// TestHandleRegistrationRequest_RejectTrackingAreaNotAllowed validates that a
// registration request for a non-allowed tracking area is rejected.
func TestHandleRegistrationRequest_RejectTrackingAreaNotAllowed(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, nil, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Conn().Tai = models.Tai{
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

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered after the reject, got %v", ue.State())
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
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, nil, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	m.UESecurityCapability = nil

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered after the reject, got %v", ue.State())
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

// TestHandleRegistrationRequest_RejectMissingSecurityCapability_Mobility
// validates that a Mobility Registration Updating without UE Security
// Capability is rejected. Per TS 24.501 the UE shall include the
// IE for every registration type except periodic registration updating.
func TestHandleRegistrationRequest_RejectMissingSecurityCapability_Mobility(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, nil, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	m.SetRegistrationType5GS(nasMessage.RegistrationType5GSMobilityRegistrationUpdating)
	m.UESecurityCapability = nil

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered after the reject, got %v", ue.State())
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

	if err := nm.PlainNasDecode(&resp.NasPdu); err != nil {
		t.Fatalf("could not decode plain NAS message")
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationReject {
		t.Fatalf("expected a registration reject message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

// TestHandleRegistrationRequest_PeriodicAllowsMissingSecurityCapability
// validates that a periodic registration updating may omit the UE Security
// Capability IE per TS 24.501.
func TestHandleRegistrationRequest_PeriodicAllowsMissingSecurityCapability(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, nil, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	m.SetRegistrationType5GS(nasMessage.RegistrationType5GSPeriodicRegistrationUpdating)
	m.UESecurityCapability = nil

	if err := handleRegistrationRequestMessage(ctx, amfInstance, ue, regReqFgs(t, m), true); err != nil {
		t.Fatalf("periodic registration without UE security capability should not be rejected here, got: %v", err)
	}

	for _, sent := range ngapSender.SentDownlinkNASTransport {
		nm := new(nas.Message)

		nm.SecurityHeaderType = nas.GetSecurityHeaderType(sent.NasPdu) & 0x0f
		if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
			continue
		}

		if err := nm.PlainNasDecode(&sent.NasPdu); err != nil {
			continue
		}

		if nm.GmmHeader.GetMessageType() == nas.MsgTypeRegistrationReject {
			t.Fatalf("periodic registration should not produce a RegistrationReject for missing UE security capability")
		}
	}
}

// TestHandleRegistrationRequest_Timers_Stopped validates that the timers
// T3513 and T3565 are stopped when receiving a registration request.
func TestHandleRegistrationRequest_Timers_Stopped(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, nil, nil)

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.ArmPagingForTest(10*time.Minute, 10)

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if ue.PagingActiveForTest() {
		t.Fatalf("timer T3513 should have been stopped")
	}
}

// TestHandleRegistrationRequest_IdentityRequest_MissingSUCI_SUPI validates that
// a registration request for a UE missing SUCI and SUPI triggers an
// Identity Request message.
func TestHandleRegistrationRequest_IdentityRequest_MissingSUCI_SUPI(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, nil, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

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
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: []byte("testkey"),
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

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
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"CAFE64\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: []byte("testkey"),
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	// The RAN reports the TAC as lowercase hex (hex.EncodeToString); the operator
	// configured it uppercase. Canonicalisation makes them match.
	ue.Tai.Tac = "cafe64"
	ue.Conn().Tai.Tac = "cafe64"

	ue.Suci = "testsuci"
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a protected and ciphered NAS message")
	}

	decoded, err := amf.DecodeNASMessage(ue, resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message")
	}

	if decoded.Message.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected a registration accept message, got '%v'", decoded.Message.GmmHeader.GetMessageType())
	}
}

// TestHandleRegistrationRequest_ContextSetup_IdenticalIEs_ResendsAccept validates
// that a duplicate REGISTRATION REQUEST with identical IEs while awaiting REGISTRATION
// COMPLETE resends the REGISTRATION ACCEPT without re-authenticating or dropping the
// UE (TS 24.501 §5.5.1.2.8 case d).
func TestHandleRegistrationRequest_ContextSetup_IdenticalIEs_ResendsAccept(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, nil, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepContextSetup)

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	// Seed the stored request with the fgs form of the incoming so the identical-IEs
	// path (resend accept) is exercised.
	stored := regReqFgs(t, m)

	conn := ue.Conn()
	conn.RegistrationRequest = stored
	conn.RegistrationAcceptPdu = []byte{0x7e, 0x00, 0x42}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected the Registration Accept to be resent, got %d downlinks", len(ngapSender.SentDownlinkNASTransport))
	}

	if ue.State() == amf.Deregistered {
		t.Fatal("an identical duplicate must not deregister the UE")
	}
}

// TestHandleRegistrationRequest_ContextSetup_DifferingIEs_Progresses validates that
// a REGISTRATION REQUEST with differing IEs while awaiting REGISTRATION COMPLETE
// aborts the previous registration and progresses the new one — here re-dispatched as
// a fresh registration that authenticates (TS 24.501 §5.5.1.2.8 case d).
func TestHandleRegistrationRequest_ContextSetup_DifferingIEs_Progresses(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: []byte("testkey"),
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))
	ue.ForceRegStepForTest(amf.RegStepContextSetup)
	// No stored prior request, so the incoming one differs and the procedure progresses.

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected the new registration to progress with an Authentication Request, got %d downlinks", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if err := nm.PlainNasDecode(&resp.NasPdu); err != nil {
		t.Fatalf("could not decode NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeAuthenticationRequest {
		t.Fatalf("expected AuthenticationRequest, got message type %d", nm.GmmHeader.GetMessageType())
	}
}

// TestHandleRegistrationRequest_UEStateAuthentication_Error validates that
// a registration request for a UE in the middle of an authentication procedure
// triggers an error.
func TestHandleRegistrationRequest_UEStateAuthentication_RestartsRegistration(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: []byte("testkey"),
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))
	ue.ForceRegStepForTest(amf.RegStepAuthenticating)

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message (AuthenticationRequest), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeAuthenticationRequest {
		t.Fatalf("expected AuthenticationRequest, got message type %d", nm.GmmHeader.GetMessageType())
	}
}

// TestHandleRegistrationRequest_SecurityMode_AuthenticationRequest validates
// that a registration request coming in while the security mode procedure is
// on-going resets the state of the UE to deregistered, triggering a new
// AuthenticationRequest.
func TestHandleRegistrationRequest_SecurityMode_AuthenticationRequest(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: []byte("testkey"),
	}, nil)
	amfInstance.NASGuardCfg.Enable = false // Prevent timer from being re-started during re-entry.

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	ue.ForceRegStepForTest(amf.RegStepSecurityMode)
	ue.Conn().NASGuardForTest().Arm(10*time.Minute, 10, func(e int32) {}, func() {})

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	// The prior registration is aborted: its context is deregistered and its NAS
	// connection — carrying T3560 — is released. The new registration runs on a fresh
	// context reusing the same radio, so the Authentication Request still goes out.
	if ue.State() != amf.Deregistered {
		t.Fatalf("aborted registration context should be Deregistered, got %s", ue.State())
	}

	if ue.Conn() != nil {
		t.Fatal("old context's NAS connection (and its T3560 guard) should be released")
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
	supi := mustSUPIFromPrefixed("imsi-001019756139935")
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(rand),
			Autn: hex.EncodeToString(autn),
		},
		Supi:  supi,
		Kseaf: []byte("testkey"),
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(supi)
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(security.AlgIntegrity128NIA0)

	m, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCount())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

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

	if err := security.NASEncrypt(ue.CipheringAlgForTest(), ue.KnasEncForTest(), ue.ULCount(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
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
	supi := mustSUPIFromPrefixed("imsi-001019756139935")
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(rand),
			Autn: hex.EncodeToString(autn),
		},
		Supi:  supi,
		Kseaf: []byte("testkey"),
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(supi)
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(security.AlgIntegrity128NIA0)

	m, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCount())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	key = [16]uint8{0x00, 0x00, 0x00, 0x00, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	ue.SetKnasEncForTest(key)

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered after the reject, got %v", ue.State())
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

// TestHandleRegistrationRequest_CipheredNAS_MacFailed_SkipContainer validates that
// when MAC verification fails (no valid security context), the NASMessageContainer
// decryption is skipped and the registration proceeds with cleartext IEs only,
// triggering an authentication request; the container is never decoded.
func TestHandleRegistrationRequest_CipheredNAS_MacFailed_SkipContainer(t *testing.T) {
	ctx := context.TODO()
	supi := mustSUPIFromPrefixed("imsi-001019756139935")
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  supi,
		Kseaf: []byte("testkey"),
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(supi)
	// Simulate MAC verification failure (amf.AMF has no valid security context)
	ue.SetSecuredForTest(false)

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2

	m, err := buildTestRegistrationRequestMessage(algo, &key, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), false)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeAuthenticationRequest {
		t.Fatalf("expected an authentication request (re-auth after MAC failure), got '%v'", nm.GmmHeader.GetMessageType())
	}

	if !ue.Conn().RetransmissionOfInitialNASMsg {
		t.Fatalf("RetransmissionOfInitialNASMsg should be set when MAC failed with NASMessageContainer")
	}
}

// TestHandleRegistrationRequest_NgKsi_Increment validates that the amf.AMF
// allocates the next available ngKSI slot (current + 1) to avoid reusing
// the UE's active key, per 3GPP TS 24.501 section 9.11.3.32.
func TestHandleRegistrationRequest_NgKsi_Increment(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: []byte("testkey"),
	}, nil)

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))

	m, err := buildTestRegistrationRequestMessageWithNgKsi(0, nil, 0, 3)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if ue.NgKsiForTest().Ksi != 4 {
		t.Fatalf("expected ngKSI=4 (next after 3), got %d", ue.NgKsiForTest().Ksi)
	}
}

// TestHandleRegistrationRequest_NgKsi_WrapAt6 validates that when the UE sends
// ngKSI=6 (the maximum valid value), the amf.AMF wraps around to 0, never
// using value 7 (which means "no key available").
func TestHandleRegistrationRequest_NgKsi_WrapAt6(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: []byte("testkey"),
	}, nil)

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))

	m, err := buildTestRegistrationRequestMessageWithNgKsi(0, nil, 0, 6)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if ue.NgKsiForTest().Ksi != 0 {
		t.Fatalf("expected ngKSI=0 (wrapped from 6), got %d", ue.NgKsiForTest().Ksi)
	}
}

// TestHandleRegistrationRequest_NgKsi_NoKeyAvailable validates that when the UE
// sends ngKSI=7 ("no key available"), the amf.AMF handles it gracefully by
// resetting to 0 with native security context type.
func TestHandleRegistrationRequest_NgKsi_NoKeyAvailable(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: []byte("testkey"),
	}, nil)

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))

	m, err := buildTestRegistrationRequestMessageWithNgKsi(0, nil, 0, 7)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	if ue.NgKsiForTest().Ksi != 0 {
		t.Fatalf("expected ngKSI=0 (reset from no-key-available=7), got %d", ue.NgKsiForTest().Ksi)
	}

	if ue.NgKsiForTest().Tsc != models.ScTypeNative {
		t.Fatalf("expected TSC=NATIVE, got %v", ue.NgKsiForTest().Tsc)
	}
}

func buildTestRegistrationRequestMessage(cipherAlg uint8, key *[16]uint8, ulcount uint32) (*nas.GmmMessage, error) {
	return buildTestRegistrationRequestMessageWithNgKsi(cipherAlg, key, ulcount, 0)
}

func buildTestRegistrationRequestMessageWithNgKsi(cipherAlg uint8, key *[16]uint8, ulcount uint32, ngKsi uint8) (*nas.GmmMessage, error) {
	m := nas.NewGmmMessage()

	registrationRequest := nasMessage.NewRegistrationRequest(0)
	registrationRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	registrationRequest.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	registrationRequest.SetSpareHalfOctet(0x00)
	registrationRequest.SetMessageType(nas.MsgTypeRegistrationRequest)
	registrationRequest.NgksiAndRegistrationType5GS.SetNasKeySetIdentifiler(ngKsi)
	registrationRequest.SetRegistrationType5GS(nasMessage.RegistrationType5GSInitialRegistration)
	registrationRequest.SetFOR(1)
	registrationRequest.MobileIdentity5GS = nasType.MobileIdentity5GS{
		Iei:    nasMessage.MobileIdentity5GSType5gGuti,
		Len:    15,
		Buffer: make([]uint8, 15),
	}
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

func buildUeAndRadio() (*amf.UeContext, *fakeNGAPSender, error) {
	ue := amf.NewUeContext()

	ngapSender := fakeNGAPSender{}
	radio := amf.Radio{
		Log:  logger.AmfLog.With(logger.RanAddr("test_localhost")),
		Conn: &ngapSender,
	}

	amfInstance := amf.New(nil, nil, nil)
	radio.BindAMFForTest(amfInstance)

	ueConn, err := amfInstance.NewUeConn(&radio, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create a new ueConn: %v", err)
	}

	ueConn.Tai = models.Tai{
		PlmnID: &models.PlmnID{
			Mcc: "001",
			Mnc: "01",
		},
		Tac: "000001",
	}

	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	return ue, &ngapSender, nil
}

// newBoundUe returns a UeContext with a UeConn attached, so NasConn() is live.
func newBoundUe(t *testing.T) *amf.UeContext {
	t.Helper()

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	return ue
}

func TestAcceptRegistrationUESecurityCapability_InitialOverwrites(t *testing.T) {
	ue := newBoundUe(t)
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration
	ue.SetUESecurityCapabilityForTest([]byte{0xE0, 0xE0}) // EA1/2/3 + IA1/2/3

	acceptRegistrationUESecurityCapability(context.Background(), ue, []byte{0x80, 0x80}) // only EA1 + IA1

	if !bytes.Equal(ue.UESecurityCapabilityForTest(), []byte{0x80, 0x80}) {
		t.Fatalf("Initial Registration must replace stored caps, got %#v", ue.UESecurityCapabilityForTest())
	}
}

func TestAcceptRegistrationUESecurityCapability_EmergencyOverwrites(t *testing.T) {
	ue := newBoundUe(t)
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSEmergencyRegistration
	ue.SetUESecurityCapabilityForTest([]byte{0xE0, 0xE0})

	acceptRegistrationUESecurityCapability(context.Background(), ue, []byte{0x00, 0x00})

	if !bytes.Equal(ue.UESecurityCapabilityForTest(), []byte{0x00, 0x00}) {
		t.Fatalf("Emergency Registration must replace stored caps, got %#v", ue.UESecurityCapabilityForTest())
	}
}

func TestAcceptRegistrationUESecurityCapability_MobilityNoStored(t *testing.T) {
	ue := newBoundUe(t)
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSMobilityRegistrationUpdating
	ue.SetUESecurityCapabilityForTest(nil)

	acceptRegistrationUESecurityCapability(context.Background(), ue, []byte{0xE0, 0xE0})

	if !bytes.Equal(ue.UESecurityCapabilityForTest(), []byte{0xE0, 0xE0}) {
		t.Fatalf("Mobility Update with no stored caps must adopt received caps, got %#v", ue.UESecurityCapabilityForTest())
	}
}

// TestAcceptRegistrationUESecurityCapability_MobilityRejectsDowngrade is
// the regression for TS 33.501 downgrade protection on Mobility
// Registration Update.
func TestAcceptRegistrationUESecurityCapability_MobilityRejectsDowngrade(t *testing.T) {
	ue := newBoundUe(t)
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSMobilityRegistrationUpdating
	ue.SetUESecurityCapabilityForTest([]byte{0xE0, 0xE0})

	acceptRegistrationUESecurityCapability(context.Background(), ue, []byte{0x00, 0x00})

	if !bytes.Equal(ue.UESecurityCapabilityForTest(), []byte{0xE0, 0xE0}) {
		t.Fatalf("Mobility Update must NOT overwrite stored caps with forged downgrade (TS 33.501): %#v", ue.UESecurityCapabilityForTest())
	}
}

func TestAcceptRegistrationUESecurityCapability_PeriodicRejectsDowngrade(t *testing.T) {
	ue := newBoundUe(t)
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSPeriodicRegistrationUpdating
	ue.SetUESecurityCapabilityForTest([]byte{0xE0, 0xE0})

	acceptRegistrationUESecurityCapability(context.Background(), ue, []byte{0x00, 0x00})

	if !bytes.Equal(ue.UESecurityCapabilityForTest(), []byte{0xE0, 0xE0}) {
		t.Fatalf("Periodic Update must NOT overwrite stored caps with forged downgrade")
	}
}

func TestAcceptRegistrationUESecurityCapability_MobilityIdenticalCapsNoop(t *testing.T) {
	ue := newBoundUe(t)
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSMobilityRegistrationUpdating
	ue.SetUESecurityCapabilityForTest([]byte{0xE0, 0xE0})

	acceptRegistrationUESecurityCapability(context.Background(), ue, []byte{0xE0, 0xE0})

	if !bytes.Equal(ue.UESecurityCapabilityForTest(), []byte{0xE0, 0xE0}) {
		t.Fatalf("Mobility Update with identical caps must be a no-op")
	}
}

// TestHandleRegistrationRequest_InitialRegistrationAbortsNetworkDeregistration
// verifies an initial registration colliding with a network-initiated
// de-registration aborts the de-registration and progresses the registration
// (TS 24.501 §5.5.2.3.5 case d); it is not rejected with a state mismatch.
func TestHandleRegistrationRequest_InitialRegistrationAbortsNetworkDeregistration(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, nil, nil)

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.DeregistrationInitiated)

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, regReqPlain(t, m), true)

	// Progressed, not rejected: the de-registration must have been aborted.
	if ue.State() == amf.DeregistrationInitiated {
		t.Fatal("network-initiated de-registration must be aborted on an initial registration collision")
	}
}

// regReqPlain encodes a test REGISTRATION REQUEST to its plain wire bytes for the
// handler's plain-body seam.
func regReqPlain(t *testing.T, m *nas.GmmMessage) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := m.EncodeRegistrationRequest(&buf); err != nil {
		t.Fatalf("encode registration request: %v", err)
	}

	return buf.Bytes()
}

// regReqFgs parses a test REGISTRATION REQUEST into the home-built form.
func regReqFgs(t *testing.T, m *nas.GmmMessage) *fgs.RegistrationRequest {
	t.Helper()

	req, err := fgs.ParseRegistrationRequest(regReqPlain(t, m))
	if err != nil {
		t.Fatalf("parse registration request: %v", err)
	}

	return req
}
