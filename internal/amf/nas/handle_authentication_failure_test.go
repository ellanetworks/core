// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

func buildTestAuthenticationFailureMessage(cause uint8, auts *[14]uint8) *nasMessage.AuthenticationFailure {
	msg := nasMessage.NewAuthenticationFailure(0)
	msg.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	msg.SetSpareHalfOctet(0x00)
	msg.SetMessageType(nas.MsgTypeAuthenticationFailure)
	msg.SetCauseValue(cause)

	if auts != nil {
		msg.AuthenticationFailureParameter = nasType.NewAuthenticationFailureParameter(
			nasMessage.AuthenticationFailureAuthenticationFailureParameterType)
		msg.SetLen(14)
		msg.SetAuthenticationFailureParameter(*auts)
	}

	return msg
}

// An AUTHENTICATION FAILURE received outside the authentication exchange is ignored:
// no downlink is emitted.
func TestHandleAuthenticationFailure_WrongState_Error(t *testing.T) {
	testcases := []struct {
		name  string
		setup func(*amf.UeContext)
	}{
		{"Deregistered", func(ue *amf.UeContext) { ue.ForceStateForTest(amf.Deregistered) }},
		{"Registered", func(ue *amf.UeContext) { ue.ForceStateForTest(amf.Registered) }},
		{"SecurityMode", func(ue *amf.UeContext) { ue.ForceRegStepForTest(amf.RegStepSecurityMode) }},
		{"ContextSetup", func(ue *amf.UeContext) { ue.ForceRegStepForTest(amf.RegStepContextSetup) }},
	}
	for _, tc := range testcases {
		t.Run(fmt.Sprintf("State-%s", tc.name), func(t *testing.T) {
			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not build UE and radio: %v", err)
			}

			tc.setup(ue)

			msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMMACFailure, nil)

			handleAuthenticationFailure(t.Context(), amf.New(nil, nil, nil), ue, msg)

			if len(ngapSender.SentDownlinkNASTransport) != 0 {
				t.Fatalf("expected Authentication Failure outside the authentication exchange to be ignored, but a downlink was sent")
			}
		})
	}
}

func TestHandleAuthenticationFailure_T3560Stopped(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	conn := ue.Conn()
	conn.AuthenticationCtx = &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))}
	conn.NASGuardForTest().Arm(10*time.Minute, 5, func(e int32) {}, func() {})

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMMACFailure, nil)

	handleAuthenticationFailure(t.Context(), amf.New(nil, nil, nil), ue, msg)

	if conn.NASGuardForTest().Active() {
		t.Fatal("expected timer T3560 to be stopped and cleared")
	}
}

func TestHandleAuthenticationFailure_MACFailure_DeregistersAndSendsReject(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	ue.Conn().AuthenticationCtx = &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))}

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMMACFailure, nil)

	handleAuthenticationFailure(t.Context(), amf.New(nil, nil, nil), ue, msg)

	if ue.State() != amf.Deregistered {
		t.Fatalf("expected UE state to be amf.Deregistered, got: %s", ue.State())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
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

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeAuthenticationReject {
		t.Fatalf("expected AuthenticationReject message, got: %v", nm.GmmHeader.GetMessageType())
	}
}

func TestHandleAuthenticationFailure_Non5GAuthUnacceptable_DeregistersAndSendsReject(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	ue.Conn().AuthenticationCtx = &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))}

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMNon5GAuthenticationUnacceptable, nil)

	handleAuthenticationFailure(t.Context(), amf.New(nil, nil, nil), ue, msg)

	if ue.State() != amf.Deregistered {
		t.Fatalf("expected UE state to be amf.Deregistered, got: %s", ue.State())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
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

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeAuthenticationReject {
		t.Fatalf("expected AuthenticationReject message, got: %v", nm.GmmHeader.GetMessageType())
	}
}

func TestHandleAuthenticationFailure_NgKSIAlreadyInUse_KsiIncremented_SendsAuthRequest(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	ue.SetNgKsiForTest(models.NgKsi{Ksi: 3})
	ue.Conn().SetResyncTried(true)
	ue.Conn().AuthenticationCtx = &ausf.AuthResult{
		Rand: hex.EncodeToString(make([]byte, 16)),
		Autn: hex.EncodeToString(make([]byte, 16)),
	}
	ue.SetABBAForTest([]uint8{0x00, 0x00})

	amfInstance := amf.New(nil, nil, nil)

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMngKSIAlreadyInUse, nil)

	handleAuthenticationFailure(t.Context(), amfInstance, ue, msg)

	if ue.NgKsiForTest().Ksi != 4 {
		t.Fatalf("expected NgKsi.Ksi to be 4, got: %d", ue.NgKsiForTest().Ksi)
	}

	if ue.Conn().ResyncTried() {
		t.Fatalf("expected resyncTried to be false after NgKSI reselection")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
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
		t.Fatalf("expected AuthenticationRequest message, got: %v", nm.GmmHeader.GetMessageType())
	}
}

func TestHandleAuthenticationFailure_NgKSIAlreadyInUse_KsiWrapsToZero(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	ue.SetNgKsiForTest(models.NgKsi{Ksi: 6})
	ue.Conn().AuthenticationCtx = &ausf.AuthResult{
		Rand: hex.EncodeToString(make([]byte, 16)),
		Autn: hex.EncodeToString(make([]byte, 16)),
	}
	ue.SetABBAForTest([]uint8{0x00, 0x00})

	amfInstance := amf.New(nil, nil, nil)

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMngKSIAlreadyInUse, nil)

	handleAuthenticationFailure(t.Context(), amfInstance, ue, msg)

	if ue.NgKsiForTest().Ksi != 0 {
		t.Fatalf("expected NgKsi.Ksi to wrap to 0, got: %d", ue.NgKsiForTest().Ksi)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestHandleAuthenticationFailure_SynchFailure_FirstTime_Success(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	ue.Conn().AuthenticationCtx = &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))}
	ue.Conn().SetResyncTried(false)
	ue.Suci = "suci-0-001-01-0000-0-0-0000000001"
	ue.Tai = ue.Conn().Tai

	expectedAv := &ausf.AuthResult{
		Rand: hex.EncodeToString(make([]byte, 16)),
		Autn: hex.EncodeToString(make([]byte, 16)),
	}

	amfInstance := amf.New(nil, &fakeAusf{
		AvKgAka: expectedAv,
	}, nil)

	auts := [14]uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e}
	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMSynchFailure, &auts)

	handleAuthenticationFailure(t.Context(), amfInstance, ue, msg)

	if !ue.Conn().ResyncTried() {
		t.Fatalf("expected resyncTried to be true after one synch failure")
	}

	if ue.Conn().AuthenticationCtx != expectedAv {
		t.Fatal("expected AuthenticationCtx to be updated from AUSF response")
	}

	if len(ue.ABBAForTest()) != 2 || ue.ABBAForTest()[0] != 0x00 || ue.ABBAForTest()[1] != 0x00 {
		t.Fatalf("expected ABBA to be {0x00, 0x00}, got: %v", ue.ABBAForTest())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
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
		t.Fatalf("expected AuthenticationRequest message, got: %v", nm.GmmHeader.GetMessageType())
	}
}

func TestHandleAuthenticationFailure_SynchFailure_FirstTime_AusfError(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	ue.Conn().AuthenticationCtx = &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))}
	ue.Conn().SetResyncTried(false)
	ue.Suci = "suci-0-001-01-0000-0-0-0000000001"
	ue.Tai = ue.Conn().Tai

	amfInstance := amf.New(nil, &fakeAusf{
		Error: fmt.Errorf("ausf unavailable"),
	}, nil)

	auts := [14]uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e}
	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMSynchFailure, &auts)

	handleAuthenticationFailure(t.Context(), amfInstance, ue, msg)

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected no downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestHandleAuthenticationFailure_SynchFailure_SecondTime_DeregistersAndSendsReject(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	ue.Conn().AuthenticationCtx = &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))}
	ue.Conn().SetResyncTried(true)

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMSynchFailure, nil)

	handleAuthenticationFailure(t.Context(), amf.New(nil, nil, nil), ue, msg)

	if ue.State() != amf.Deregistered {
		t.Fatalf("expected UE state to be amf.Deregistered, got: %s", ue.State())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
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

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeAuthenticationReject {
		t.Fatalf("expected AuthenticationReject message, got: %v", nm.GmmHeader.GetMessageType())
	}
}

func TestHandleAuthenticationFailure_SynchFailure_NilAuthenticationFailureParameter(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	ue.Conn().AuthenticationCtx = &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))}
	ue.Conn().SetResyncTried(false)

	// Build message with SynchFailure cause but nil AuthenticationFailureParameter
	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMSynchFailure, nil)

	// This must not panic — before the fix it caused a nil pointer dereference. A
	// SynchFailure with no AUTS is dropped without emitting a downlink.
	handleAuthenticationFailure(t.Context(), amf.New(nil, nil, nil), ue, msg)

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected no downlink for SynchFailure with nil AuthenticationFailureParameter, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}
}

// TestHandleAuthenticationFailure_OutOfEnumerationCauseIgnored verifies an
// AUTHENTICATION FAILURE carrying a cause outside the enumeration is ignored: the
// authentication guard (T3560) is left armed and nothing is sent, rather than
// stranding the UE (semantically incorrect message, TS 24.501 §7.8). Mirrors the MME.
func TestHandleAuthenticationFailure_OutOfEnumerationCauseIgnored(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	conn := ue.Conn()
	conn.AuthenticationCtx = &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))}
	conn.NASGuardForTest().Arm(10*time.Minute, 5, func(e int32) {}, func() {})

	// #111 "protocol error, unspecified" is a valid 5GMM cause but not an
	// AUTHENTICATION FAILURE cause.
	msg := buildTestAuthenticationFailureMessage(0x6f, nil)

	handleAuthenticationFailure(t.Context(), amf.New(nil, nil, nil), ue, msg)

	if !conn.NASGuardForTest().Active() {
		t.Fatal("the authentication guard must stay armed on an out-of-enumeration cause")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("an ignored Authentication Failure must send nothing, got %d downlinks", len(ngapSender.SentDownlinkNASTransport))
	}
}

// TestHandleAuthenticationFailure_NoChallengeInFlightIgnored verifies a spurious
// AUTHENTICATION FAILURE in the identity sub-window of RegStepAuthenticating (no
// challenge sent, so AuthenticationCtx is nil) is ignored rather than releasing
// the UE (admissible without integrity, TS 24.501 §4.4.4.3). Mirrors the MME.
func TestHandleAuthenticationFailure_NoChallengeInFlightIgnored(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	// No AuthenticationCtx: the identity sub-window, no challenge in flight.

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMMACFailure, nil)

	handleAuthenticationFailure(t.Context(), amf.New(nil, nil, nil), ue, msg)

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("a spurious Authentication Failure with no challenge in flight must send nothing, got %d downlinks", len(ngapSender.SentDownlinkNASTransport))
	}
}
