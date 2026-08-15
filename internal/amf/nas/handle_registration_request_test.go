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
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// TestRegistrationTypeName checks that the metric and log label comes from the
// codec, which names every assigned type and marks the rest unknown. The table
// this replaced reported a disaster roaming initial registration as "Reserved".
func TestRegistrationTypeName(t *testing.T) {
	cases := map[fgs.RegistrationType]string{
		fgs.RegistrationTypeInitial:                "Initial registration",
		fgs.RegistrationTypeMobilityUpdating:       "Mobility registration updating",
		fgs.RegistrationTypePeriodicUpdating:       "Periodic registration updating",
		fgs.RegistrationTypeEmergency:              "Emergency registration",
		fgs.RegistrationTypeDisasterRoamingInitial: "Disaster roaming initial registration",
		fgs.RegistrationTypeSNPNOnboarding:         "SNPN onboarding registration",
		fgs.RegistrationType(0):                    "unknown (0)",
	}

	for in, want := range cases {
		if got := registrationTypeName(in); got != want {
			t.Errorf("registrationTypeName(%d) = %q, want %q", uint8(in), got, want)
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

	handleRegistrationRequest(ctx, &amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)
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

	m, err := buildRegReqBytes(uint8(fgs.RegistrationTypeInitial), fgs.NoIdentity(), &fgs.UESecurityCapability{EA: 0xc0, IA: 0xc0}, 0, nil, 0, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, &amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

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

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

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

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered after the reject, got %v", ue.State())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgRegistrationReject))
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

	m, err := buildRegReqBytes(uint8(fgs.RegistrationTypeInitial), testMobileIdentity(), nil, 0, nil, 0, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered after the reject, got %v", ue.State())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgRegistrationReject))
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

	m, err := buildRegReqBytes(uint8(fgs.RegistrationTypeMobilityUpdating), testMobileIdentity(), nil, 0, nil, 0, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered after the reject, got %v", ue.State())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgRegistrationReject))
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

	m, err := buildRegReqBytes(uint8(fgs.RegistrationTypePeriodicUpdating), testMobileIdentity(), nil, 0, nil, 0, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	if err := handleRegistrationRequestMessage(ctx, amfInstance, ue, regReqFgs(t, m), m, true, false); err != nil {
		t.Fatalf("periodic registration without UE security capability should not be rejected here, got: %v", err)
	}

	for _, sent := range ngapSender.SentDownlinkNASTransport {
		if len(sent.NASPDU) < 3 || fgs.SecurityHeaderType(sent.NASPDU[1]&0x0f) != fgs.SHTPlain {
			continue
		}

		if sent.NASPDU[2] == uint8(fgs.MsgRegistrationReject) {
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

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

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

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgIdentityRequest))
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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgAuthenticationRequest))
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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

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

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	if len(resp.NASPDU) < 7 || fgs.SecurityHeaderType(resp.NASPDU[1]&0x0f) != fgs.SHTIntegrityProtectedCiphered {
		t.Fatalf("expected a protected and ciphered NAS message")
	}

	decoded, err := amf.DecodeNASMessage(ue, resp.NASPDU)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message")
	}

	if decoded.MessageType != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected a registration accept message, got %d", decoded.MessageType)
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

	// Seed the stored request with the raw plaintext of the incoming so the
	// identical-IEs path (resend accept) is exercised.
	conn := ue.Conn()
	conn.RegistrationRequest = regReqFgs(t, m)
	conn.RegistrationRequestPlain = m
	conn.RegistrationAcceptPlain = []byte{0x7e, 0x00, 0x42}

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepContextSetup)

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected the new registration to progress with an Authentication Request, got %d downlinks", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgAuthenticationRequest))
}

// TS 24.501 §5.5.1.2.8 case d
func TestHandleRegistrationRequest_ContextSetup_UnmodeledIEDiffers_Progresses(t *testing.T) {
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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepContextSetup)

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	conn := ue.Conn()
	conn.RegistrationRequest = regReqFgs(t, m)
	conn.RegistrationRequestPlain = m
	conn.RegistrationAcceptPlain = []byte{0x7e, 0x00, 0x42}

	// Requested mapped NSSAI (0x35): the message preserves it without modelling
	// it, so the two differ in bytes while every field the AMF reads is equal.
	incoming := append(append([]byte{}, m...), 0x35, 0x02, 0x01, 0x01)

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, incoming), incoming, true, false)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected the new registration to progress with an Authentication Request, got %d downlinks", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgAuthenticationRequest))
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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message (AuthenticationRequest), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgAuthenticationRequest))
}

// TS 24.501 §5.5.1.2.8 case e
func TestHandleRegistrationRequest_Authenticating_IdenticalIEs_Ignored(t *testing.T) {
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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	ue.Conn().RegistrationRequestPlain = m

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("an identical pre-accept duplicate must be ignored (no downlink), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	if ue.RegStep() != amf.RegStepAuthenticating {
		t.Fatalf("an identical pre-accept duplicate must not restart; RegStep = %v", ue.RegStep())
	}
}

// An identical retransmission during the security mode procedure is ignored, not
// restarted (TS 24.501 §5.5.1.2.8 case e).
func TestHandleRegistrationRequest_SecurityMode_IdenticalIEs_Ignored(t *testing.T) {
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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepSecurityMode)

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	ue.Conn().RegistrationRequestPlain = m

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("an identical pre-accept duplicate must be ignored (no downlink), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	if ue.RegStep() != amf.RegStepSecurityMode {
		t.Fatalf("an identical pre-accept duplicate must not restart; RegStep = %v", ue.RegStep())
	}
}

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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

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

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

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
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgAuthenticationRequest))
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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	m, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCount())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	decipherGmm(t, ue, resp.NASPDU, uint8(fgs.MsgRegistrationAccept))
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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	m, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCount())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	key = [16]uint8{0x00, 0x00, 0x00, 0x00, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	ue.SetKnasEncForTest(key)

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered after the reject, got %v", ue.State())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgRegistrationReject))
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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	// Simulate MAC verification failure (amf.AMF has no valid security context)
	ue.SetSecuredForTest(false)

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	m, err := buildTestRegistrationRequestMessage(algo, &key, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, false, false)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgAuthenticationRequest))

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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	m, err := buildTestRegistrationRequestMessageWithNgKsi(0, nil, 0, 3)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	m, err := buildTestRegistrationRequestMessageWithNgKsi(0, nil, 0, 6)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

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

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	m, err := buildTestRegistrationRequestMessageWithNgKsi(0, nil, 0, 7)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if ue.NgKsiForTest().Ksi != 0 {
		t.Fatalf("expected ngKSI=0 (reset from no-key-available=7), got %d", ue.NgKsiForTest().Ksi)
	}

	if ue.NgKsiForTest().Tsc != models.ScTypeNative {
		t.Fatalf("expected TSC=NATIVE, got %v", ue.NgKsiForTest().Tsc)
	}
}

func buildTestRegistrationRequestMessage(cipherAlg nas.CipheringAlgorithm, key *[16]uint8, ulcount uint32) ([]byte, error) {
	return buildTestRegistrationRequestMessageWithNgKsi(cipherAlg, key, ulcount, 0)
}

func buildTestRegistrationRequestMessageWithNgKsi(cipherAlg nas.CipheringAlgorithm, key *[16]uint8, ulcount uint32, ngKsi uint8) ([]byte, error) {
	return buildRegReqBytes(uint8(fgs.RegistrationTypeInitial), testMobileIdentity(), &fgs.UESecurityCapability{EA: 0xc0, IA: 0xc0}, cipherAlg, key, ulcount, ngKsi)
}

// TS 24.501 §5.5.1.2.2
func buildRegReqBytes(regType uint8, mobileIdentity fgs.MobileIdentity, ueSecCap *fgs.UESecurityCapability, cipherAlg nas.CipheringAlgorithm, key *[16]uint8, ulcount uint32, ngKsi uint8) ([]byte, error) {
	m := &fgs.RegistrationRequest{
		RegistrationType:     fgs.RegistrationType(regType),
		FOR:                  true,
		NgKSI:                nas.KeySetIdentifier{Value: ngKsi},
		MobileIdentity:       mobileIdentity,
		UESecurityCapability: ueSecCap,
	}

	if key == nil {
		return m.MarshalBinary()
	}

	plain, err := m.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("could not encode registration request: %v", err)
	}

	ciph, err := nas.CipherFor(cipherAlg)
	if err != nil {
		return nil, err
	}

	ciphered, err := ciph.Apply(*key, ulcount, nas.Bearer3GPP, nas.DirectionUplink, plain)
	if err != nil {
		return nil, fmt.Errorf("could not encrypt NAS message: %v", err)
	}

	m.NASMessageContainer = ciphered

	return m.MarshalBinary()
}

// TS 24.501 §5.5.1.2.8
func TestHandleRegistrationRequestMessage_ContainerStoresOuterBytes(t *testing.T) {
	ctx := context.TODO()
	supi := mustSUPIFromPrefixed("imsi-001019756139935")
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: "[\"000001\"]"},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))},
		Supi:    supi,
		Kseaf:   []byte("testkey"),
	}, nil)

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.Suci = "testsuci"
	ue.SetSupiForTest(supi)

	if err := amfInstance.CommitUEIdentity(context.TODO(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	ue.SetSecuredForTest(true)
	ue.SetKnasEncForTest(key)
	ue.SetCipheringAlgForTest(algo)

	m, err := buildTestRegistrationRequestMessage(algo, &key, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	outer := m
	inner := append([]byte(nil), regReqFgs(t, outer).NASMessageContainer...)

	_ = handleRegistrationRequestMessage(ctx, amfInstance, ue, regReqFgs(t, outer), outer, true, false)

	stored := ue.Conn().RegistrationRequestPlain
	if !bytes.Equal(stored, outer) {
		t.Fatalf("stored plain must be the outer message bytes for duplicate detection")
	}

	if bytes.Equal(stored, inner) {
		t.Fatal("stored plain must not be the inner container contents")
	}
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
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeInitial
	ue.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0}) // EA1/2/3 + IA1/2/3

	acceptRegistrationUESecurityCapability(context.Background(), ue, &fgs.UESecurityCapability{EA: 0x80, IA: 0x80}, false) // only EA1 + IA1

	if !ue.UESecurityCapabilityForTest().Equal(fgs.UESecurityCapability{EA: 0x80, IA: 0x80}) {
		t.Fatalf("Initial Registration must replace stored caps, got %#v", ue.UESecurityCapabilityForTest())
	}
}

func TestAcceptRegistrationUESecurityCapability_EmergencyOverwrites(t *testing.T) {
	ue := newBoundUe(t)
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeEmergency
	ue.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0})

	acceptRegistrationUESecurityCapability(context.Background(), ue, &fgs.UESecurityCapability{EA: 0x00, IA: 0x00}, false)

	if !ue.UESecurityCapabilityForTest().Equal(fgs.UESecurityCapability{EA: 0x00, IA: 0x00}) {
		t.Fatalf("Emergency Registration must replace stored caps, got %#v", ue.UESecurityCapabilityForTest())
	}
}

func TestAcceptRegistrationUESecurityCapability_MobilityNoStored(t *testing.T) {
	ue := newBoundUe(t)
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating
	ue.SetUESecurityCapabilityForTest(nil)

	acceptRegistrationUESecurityCapability(context.Background(), ue, &fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0}, false)

	if !ue.UESecurityCapabilityForTest().Equal(fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0}) {
		t.Fatalf("Mobility Update with no stored caps must adopt received caps, got %#v", ue.UESecurityCapabilityForTest())
	}
}

// TestAcceptRegistrationUESecurityCapability_MobilityRejectsDowngrade is
// the regression for TS 33.501 downgrade protection on Mobility
// Registration Update.
func TestAcceptRegistrationUESecurityCapability_MobilityRejectsDowngrade(t *testing.T) {
	ue := newBoundUe(t)
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating
	ue.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0})

	acceptRegistrationUESecurityCapability(context.Background(), ue, &fgs.UESecurityCapability{EA: 0x00, IA: 0x00}, false)

	if !ue.UESecurityCapabilityForTest().Equal(fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0}) {
		t.Fatalf("Mobility Update must NOT overwrite stored caps with forged downgrade (TS 33.501): %#v", ue.UESecurityCapabilityForTest())
	}
}

func TestAcceptRegistrationUESecurityCapability_PeriodicRejectsDowngrade(t *testing.T) {
	ue := newBoundUe(t)
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypePeriodicUpdating
	ue.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0})

	acceptRegistrationUESecurityCapability(context.Background(), ue, &fgs.UESecurityCapability{EA: 0x00, IA: 0x00}, false)

	if !ue.UESecurityCapabilityForTest().Equal(fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0}) {
		t.Fatalf("Periodic Update must NOT overwrite stored caps with forged downgrade")
	}
}

func TestAcceptRegistrationUESecurityCapability_MobilityIdenticalCapsNoop(t *testing.T) {
	ue := newBoundUe(t)
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating
	ue.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0})

	acceptRegistrationUESecurityCapability(context.Background(), ue, &fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0}, false)

	if !ue.UESecurityCapabilityForTest().Equal(fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0}) {
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

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	// Progressed, not rejected: the de-registration must have been aborted.
	if ue.State() == amf.DeregistrationInitiated {
		t.Fatal("network-initiated de-registration must be aborted on an initial registration collision")
	}
}

// regReqFgs parses a test REGISTRATION REQUEST into the home-built form.
func regReqFgs(t *testing.T, pdu []byte) *fgs.RegistrationRequest {
	t.Helper()

	req, err := fgs.ParseRegistrationRequest(pdu)
	if err != nil {
		t.Fatalf("parse registration request: %v", err)
	}

	return req
}

// testMobileIdentity is the 5G-GUTI these tests present as the UE identity; the
// value is immaterial to what they assert, only that one is present.
func testMobileIdentity() fgs.MobileIdentity {
	return fgs.GUTIIdentity(fgs.GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 1, AMFSetID: 1, AMFPointer: 1})
}

// mustParseRegistrationRequest decodes a wire REGISTRATION REQUEST for a test that
// drives the handler directly.
func mustParseRegistrationRequest(t *testing.T, plain []byte) *fgs.RegistrationRequest {
	t.Helper()

	req, err := fgs.ParseRegistrationRequest(plain)
	if err != nil && !nas.SoftOnly(err) {
		t.Fatalf("build REGISTRATION REQUEST: %v", err)
	}

	return req
}

// TestHandleRegistrationRequest_ContextSetup_AfterSecurityModeContainer_ResendsAccept
// pins the oracle case d compares against once a security mode procedure has run.
// The UE opens with cleartext IEs only and repeats the complete message in the
// SECURITY MODE COMPLETE's NAS message container (TS 24.501 §4.4.6); from then on
// the AMF holds the container's message. A UE that retransmits after the ACCEPT
// now has a security context and sends that complete message, so comparing it
// against the cleartext-only opener would re-authenticate instead of resending.
func TestHandleRegistrationRequest_ContextSetup_AfterSecurityModeContainer_ResendsAccept(t *testing.T) {
	ctx := context.TODO()
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: "[\"000001\"]"},
	}, nil, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	// The cleartext-only opener: no UE security capability, as a UE with no
	// security context sends it.
	opener, err := buildRegReqBytes(uint8(fgs.RegistrationTypeInitial), testMobileIdentity(), nil, 0, nil, 0, 0)
	if err != nil {
		t.Fatalf("could not build the initial registration request: %v", err)
	}

	// The complete message the SECURITY MODE COMPLETE carries, which contextSetup
	// stores as the registration request from then on.
	complete, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build the container registration request: %v", err)
	}

	if bytes.Equal(opener, complete) {
		t.Fatal("the opener and the container message must differ for this test to mean anything")
	}

	conn := ue.Conn()
	conn.RegistrationRequest = regReqFgs(t, opener)
	conn.RegistrationRequestPlain = opener
	conn.RegistrationAcceptPlain = []byte{0x7e, 0x00, 0x42}

	contextSetup(ctx, amfInstance, ue, regReqFgs(t, complete), complete)
	ue.ForceRegStepForTest(amf.RegStepContextSetup)

	before := len(ngapSender.SentDownlinkNASTransport)

	handleRegistrationRequest(ctx, amfInstance, ue, mustParseRegistrationRequest(t, complete), complete, true, false)

	if got := len(ngapSender.SentDownlinkNASTransport) - before; got != 1 {
		t.Fatalf("expected the Registration Accept to be resent, got %d downlinks", got)
	}

	if ue.State() == amf.Deregistered {
		t.Fatal("an identical duplicate must not deregister the UE")
	}
}

func TestAcceptRegistrationUESecurityCapability_MobilityStoresVerifiedChange(t *testing.T) {
	ue := newBoundUe(t)
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating
	ue.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0})

	changed := fgs.UESecurityCapability{EA: 0xC0, IA: 0xC0}

	acceptRegistrationUESecurityCapability(context.Background(), ue, &changed, true)

	if !ue.UESecurityCapabilityForTest().Equal(changed) {
		t.Fatalf("stored capability = %#v, want the verified %#v", ue.UESecurityCapabilityForTest(), changed)
	}
}
