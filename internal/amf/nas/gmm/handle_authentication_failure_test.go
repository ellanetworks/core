// Copyright 2026 Ella Networks

package gmm

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	amfContext "github.com/ellanetworks/core/internal/amf"
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

func TestHandleAuthenticationFailure_WrongState_Error(t *testing.T) {
	testcases := []amfContext.StateType{amfContext.Deregistered, amfContext.SecurityMode, amfContext.ContextSetup, amfContext.Registered}
	for _, tc := range testcases {
		t.Run(fmt.Sprintf("State-%s", tc), func(t *testing.T) {
			ue, _, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not build UE and radio: %v", err)
			}

			ue.ForceState(tc)

			msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMMACFailure, nil)

			expected := fmt.Sprintf("state mismatch: receive Authentication Failure message in state %s", tc)

			err = handleAuthenticationFailure(t.Context(), amfContext.New(nil, nil, nil), ue, msg)
			if err == nil || err.Error() != expected {
				t.Fatalf("expected error: %s, got: %v", expected, err)
			}
		})
	}
}

func TestHandleAuthenticationFailure_T3560Stopped(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceState(amfContext.Authentication)
	ue.T3560 = amfContext.NewTimer(10*time.Minute, 5, func(e int32) {}, func() {})

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMMACFailure, nil)

	_ = handleAuthenticationFailure(t.Context(), amfContext.New(nil, nil, nil), ue, msg)

	if ue.T3560 != nil {
		t.Fatal("expected timer T3560 to be stopped and cleared")
	}
}

func TestHandleAuthenticationFailure_MACFailure_DeregistersAndSendsReject(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceState(amfContext.Authentication)

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMMACFailure, nil)

	err = handleAuthenticationFailure(t.Context(), amfContext.New(nil, nil, nil), ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if ue.GetState() != amfContext.Deregistered {
		t.Fatalf("expected UE state to be Deregistered, got: %s", ue.GetState())
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

	ue.ForceState(amfContext.Authentication)

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMNon5GAuthenticationUnacceptable, nil)

	err = handleAuthenticationFailure(t.Context(), amfContext.New(nil, nil, nil), ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if ue.GetState() != amfContext.Deregistered {
		t.Fatalf("expected UE state to be Deregistered, got: %s", ue.GetState())
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

	ue.ForceState(amfContext.Authentication)
	ue.NgKsi = models.NgKsi{Ksi: 3}
	ue.AuthFailureCauseSynchFailureTimes = 2
	ue.AuthenticationCtx = &ausf.AuthResult{
		Rand: hex.EncodeToString(make([]byte, 16)),
		Autn: hex.EncodeToString(make([]byte, 16)),
	}
	ue.ABBA = []uint8{0x00, 0x00}

	amf := amfContext.New(nil, nil, nil)

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMngKSIAlreadyInUse, nil)

	err = handleAuthenticationFailure(t.Context(), amf, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if ue.NgKsi.Ksi != 4 {
		t.Fatalf("expected NgKsi.Ksi to be 4, got: %d", ue.NgKsi.Ksi)
	}

	if ue.AuthFailureCauseSynchFailureTimes != 0 {
		t.Fatalf("expected AuthFailureCauseSynchFailureTimes to be 0, got: %d", ue.AuthFailureCauseSynchFailureTimes)
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

	ue.ForceState(amfContext.Authentication)
	ue.NgKsi = models.NgKsi{Ksi: 6}
	ue.AuthenticationCtx = &ausf.AuthResult{
		Rand: hex.EncodeToString(make([]byte, 16)),
		Autn: hex.EncodeToString(make([]byte, 16)),
	}
	ue.ABBA = []uint8{0x00, 0x00}

	amf := amfContext.New(nil, nil, nil)

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMngKSIAlreadyInUse, nil)

	err = handleAuthenticationFailure(t.Context(), amf, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if ue.NgKsi.Ksi != 0 {
		t.Fatalf("expected NgKsi.Ksi to wrap to 0, got: %d", ue.NgKsi.Ksi)
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

	ue.ForceState(amfContext.Authentication)
	ue.AuthFailureCauseSynchFailureTimes = 0
	ue.Suci = "suci-0-001-01-0000-0-0-0000000001"
	ue.Tai = ue.RanUe.Tai

	expectedAv := &ausf.AuthResult{
		Rand: hex.EncodeToString(make([]byte, 16)),
		Autn: hex.EncodeToString(make([]byte, 16)),
	}

	amf := amfContext.New(nil, &FakeAusf{
		AvKgAka: expectedAv,
	}, nil)

	auts := [14]uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e}
	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMSynchFailure, &auts)

	err = handleAuthenticationFailure(t.Context(), amf, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if ue.AuthFailureCauseSynchFailureTimes != 1 {
		t.Fatalf("expected AuthFailureCauseSynchFailureTimes to be 1, got: %d", ue.AuthFailureCauseSynchFailureTimes)
	}

	if ue.AuthenticationCtx != expectedAv {
		t.Fatal("expected AuthenticationCtx to be updated from AUSF response")
	}

	if len(ue.ABBA) != 2 || ue.ABBA[0] != 0x00 || ue.ABBA[1] != 0x00 {
		t.Fatalf("expected ABBA to be {0x00, 0x00}, got: %v", ue.ABBA)
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

	ue.ForceState(amfContext.Authentication)
	ue.AuthFailureCauseSynchFailureTimes = 0
	ue.Suci = "suci-0-001-01-0000-0-0-0000000001"
	ue.Tai = ue.RanUe.Tai

	amf := amfContext.New(nil, &FakeAusf{
		Error: fmt.Errorf("ausf unavailable"),
	}, nil)

	auts := [14]uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e}
	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMSynchFailure, &auts)

	err = handleAuthenticationFailure(t.Context(), amf, ue, msg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected no downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestHandleAuthenticationFailure_SynchFailure_SecondTime_DeregistersAndSendsReject(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceState(amfContext.Authentication)
	ue.AuthFailureCauseSynchFailureTimes = 1

	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMSynchFailure, nil)

	err = handleAuthenticationFailure(t.Context(), amfContext.New(nil, nil, nil), ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if ue.GetState() != amfContext.Deregistered {
		t.Fatalf("expected UE state to be Deregistered, got: %s", ue.GetState())
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
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceState(amfContext.Authentication)
	ue.AuthFailureCauseSynchFailureTimes = 0

	// Build message with SynchFailure cause but nil AuthenticationFailureParameter
	msg := buildTestAuthenticationFailureMessage(nasMessage.Cause5GMMSynchFailure, nil)

	// This must not panic — before the fix it caused a nil pointer dereference
	err = handleAuthenticationFailure(t.Context(), amfContext.New(nil, nil, nil), ue, msg)
	if err == nil {
		t.Fatal("expected error when AuthenticationFailureParameter is nil, got nil")
	}
}
