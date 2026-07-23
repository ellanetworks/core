// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

// encSMC encodes a free5gc SECURITY MODE COMPLETE message to its plain wire bytes,
// the form the handler receives.
func encSMC(t *testing.T, msg *nasMessage.SecurityModeComplete) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := msg.EncodeSecurityModeComplete(&buf); err != nil {
		t.Fatalf("could not encode Security Mode Complete: %v", err)
	}

	return buf.Bytes()
}

func TestHandleSecurityMode_WrongUEMode(t *testing.T) {
	testcases := []struct {
		name  string
		setup func(*amf.UeContext)
		state amf.StateType
	}{
		{"Deregistered", func(ue *amf.UeContext) { ue.ForceStateForTest(amf.Deregistered) }, amf.Deregistered},
		{"Registered", func(ue *amf.UeContext) { ue.ForceStateForTest(amf.Registered) }, amf.Registered},
		{"Authenticating", func(ue *amf.UeContext) { ue.ForceRegStepForTest(amf.RegStepAuthenticating) }, amf.RegistrationInitiated},
		{"ContextSetup", func(ue *amf.UeContext) { ue.ForceRegStepForTest(amf.RegStepContextSetup) }, amf.RegistrationInitiated},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ue := amf.NewUeContext()
			tc.setup(ue)

			// Outside the security mode exchange the handler bails without advancing
			// the registration (TS 24.501).
			handleSecurityModeComplete(
				t.Context(),
				amf.New(nil, nil, nil),
				ue,
				nil,
				true,
			)

			if ue.State() != tc.state {
				t.Fatalf("wrong-mode Security Mode Complete changed state to %v, want %v", ue.State(), tc.state)
			}
		})
	}
}

func TestHandleSecurityMode_TimerT3560Stopped(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"1\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepSecurityMode)
	ue.Conn().NASGuardForTest().Arm(10*time.Minute, 10, func(e int32) {}, func() {})

	msg := buildTestSecurityModeCompleteMessage()

	handleSecurityModeComplete(t.Context(), amfInstance, ue, encSMC(t, msg.SecurityModeComplete), true)

	if ue.Conn().NASGuardForTest().Active() {
		t.Fatal("expected timer T3560 to be stopped and cleared")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func TestHandleSecurityMode_MsgIncludingIMEISV_UpdatesPEI(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"1\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepSecurityMode)
	ue.Conn().NASGuardForTest().Arm(10*time.Minute, 10, func(e int32) {}, func() {})

	msg := buildTestSecurityModeCompleteMessage()
	msg.IMEISV = &nasType.IMEISV{
		Iei:   nasMessage.SecurityModeCompleteIMEISVType,
		Octet: [9]uint8{nasMessage.MobileIdentity5GSTypeImeisv + 0x30, 0x25, 0x90, 0x09, 0x10, 0x67, 0x41, 0x28, 0xF3},
		Len:   9,
	}

	handleSecurityModeComplete(t.Context(), amfInstance, ue, encSMC(t, msg.SecurityModeComplete), true)

	expected := "imeisv-3520990017614823"
	if ue.Imei.String() != expected {
		t.Fatalf("expected PEI: %v, got: %v", expected, ue.Imei.String())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func TestHandleSecurityMode_ValidSecurityContext_UpdatesSecurityContext(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"1\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepSecurityMode)
	ue.SetSecuredForTest(true)
	ue.SetNgKsiForTest(models.NgKsi{Ksi: 0})

	ue.SetKgnbForTest([]uint8{})
	ue.SetNHForTest([]uint8{})
	ue.SetNCCForTest(0)

	msg := buildTestSecurityModeCompleteMessage()

	handleSecurityModeComplete(t.Context(), amfInstance, ue, encSMC(t, msg.SecurityModeComplete), true)

	if len(ue.KgnbForTest()) == 0 || ue.NHForTest() == [32]uint8{} || ue.NCCForTest() == 0 {
		t.Fatalf("expected security context to be updated, got: Kgnb: %v, NH: %v, NCC: %v", ue.KgnbForTest(), ue.NHForTest(), ue.NCCForTest())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func TestHandleSecurityMode_NASMessageContainer_RegistrationAccepted(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"1\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		&fakeSmf{},
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepSecurityMode)
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(security.AlgIntegrity128NIA0)
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration

	msg, err := buildTestSecurityModeCompleteMessageWithRegistrationRequest()
	if err != nil {
		t.Fatalf("could not build security mode complete message with registration request: %v", err)
	}

	handleSecurityModeComplete(t.Context(), amfInstance, ue, encSMC(t, msg.SecurityModeComplete), true)

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

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected a registration accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

func TestHandleSecurityMode_InvalidNASMessageContainer_Error(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"1\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepSecurityMode)
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(security.AlgIntegrity128NIA0)
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration

	msg, err := buildTestSecurityModeCompleteMessageWithRegistrationRequest()
	if err != nil {
		t.Fatalf("could not build security mode complete message with registration request: %v", err)
	}

	msg.SecurityModeComplete.SetNASMessageContainerContents([]uint8{0xDE, 0xAD, 0xBE, 0xEF})

	handleSecurityModeComplete(t.Context(), amfInstance, ue, encSMC(t, msg.SecurityModeComplete), true)

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatalf("expected a UE Context Release Command to release the aborted registration, got %d", len(ngapSender.SentUEContextReleaseCommand))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func buildTestSecurityModeCompleteMessage() *nas.GmmMessage {
	m := nas.NewGmmMessage()

	secModeComplete := nasMessage.NewSecurityModeComplete(0)
	secModeComplete.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	secModeComplete.SetSpareHalfOctet(0x00)
	secModeComplete.SetMessageType(nas.MsgTypeSecurityModeComplete)

	m.SecurityModeComplete = secModeComplete
	m.SetMessageType(nas.MsgTypeSecurityModeComplete)

	return m
}

func buildTestSecurityModeCompleteMessageWithRegistrationRequest() (*nas.GmmMessage, error) {
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
	m.SetMessageType(nas.MsgTypeSecurityModeComplete)

	data := new(bytes.Buffer)

	err := m.EncodeRegistrationRequest(data)
	if err != nil {
		return nil, fmt.Errorf("could not encode registration request: %v", err)
	}

	nasPdu := data.Bytes()

	secModeComplete := nasMessage.NewSecurityModeComplete(0)
	secModeComplete.NASMessageContainer = nasType.NewNASMessageContainer(nasMessage.RegistrationRequestNASMessageContainerType)
	secModeComplete.NASMessageContainer.SetLen(uint16(len(nasPdu)))
	secModeComplete.SetNASMessageContainerContents(nasPdu)
	secModeComplete.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	secModeComplete.SetSpareHalfOctet(0x00)
	secModeComplete.SetMessageType(nas.MsgTypeSecurityModeComplete)

	m.SecurityModeComplete = secModeComplete

	return m, nil
}
