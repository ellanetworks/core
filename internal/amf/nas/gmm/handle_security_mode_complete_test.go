package gmm

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

func TestHandleSecurityMode_WrongUEMode(t *testing.T) {
	testcases := []amfContext.StateType{
		amfContext.Deregistered,
		amfContext.Authentication,
		amfContext.ContextSetup,
		amfContext.Registered,
	}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%v", tc), func(t *testing.T) {
			expected := fmt.Errorf("state mismatch: receive Security Mode Complete message in state %s", tc)

			err := handleSecurityModeComplete(
				t.Context(),
				&amfContext.AMF{},
				&amfContext.AmfUe{State: tc},
				nil,
			)
			if err == nil || err.Error() != expected.Error() {
				t.Fatalf("expected error: %v, got: %v", expected, err)
			}
		})
	}
}

func TestHandleSecurityMode_MacFailed(t *testing.T) {
	expected := "NAS message integrity check failed"

	err := handleSecurityModeComplete(
		t.Context(),
		&amfContext.AMF{},
		&amfContext.AmfUe{
			State:     amfContext.SecurityMode,
			MacFailed: true,
		},
		nil,
	)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %v, got: %v", expected, err)
	}
}

func TestHandleSecurityMode_TimerT3560Stopped(t *testing.T) {
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"1\"]",
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
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.State = amfContext.SecurityMode
	ue.T3560 = amfContext.NewTimer(10*time.Minute, 10, func(e int32) {}, func() {})

	msg := buildTestSecurityModeCompleteMessage()

	err = handleSecurityModeComplete(t.Context(), amf, ue, msg.SecurityModeComplete)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if ue.T3560 != nil {
		t.Fatal("expected timer T3560 to be stopped and cleared")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func TestHandleSecurityMode_MsgIncludingIMEISV_UpdatesPEI(t *testing.T) {
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"1\"]",
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
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.State = amfContext.SecurityMode
	ue.T3560 = amfContext.NewTimer(10*time.Minute, 10, func(e int32) {}, func() {})

	msg := buildTestSecurityModeCompleteMessage()
	msg.IMEISV = &nasType.IMEISV{
		Octet: [9]uint8{nasMessage.MobileIdentity5GSTypeImeisv + 0x30, 0x25, 0x90, 0x09, 0x10, 0x67, 0x41, 0x28, 0xF3},
		Len:   9,
	}

	err = handleSecurityModeComplete(t.Context(), amf, ue, msg.SecurityModeComplete)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := "imeisv-3520990017614823"
	if ue.Pei != expected {
		t.Fatalf("expected PEI: %v, got: %v", expected, ue.Pei)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func TestHandleSecurityMode_ValidSecurityContext_UpdatesSecurityContext(t *testing.T) {
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"1\"]",
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
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.State = amfContext.SecurityMode
	ue.SecurityContextAvailable = true
	ue.NgKsi = models.NgKsi{Ksi: 0}
	ue.MacFailed = false

	ue.Kgnb = []uint8{}
	ue.NH = []uint8{}
	ue.NCC = 0

	msg := buildTestSecurityModeCompleteMessage()

	err = handleSecurityModeComplete(t.Context(), amf, ue, msg.SecurityModeComplete)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ue.Kgnb) == 0 || len(ue.NH) == 0 || ue.NCC == 0 {
		t.Fatalf("expected security context to be updated, got: Kgnb: %v, NH: %v, NCC: %v", ue.Kgnb, ue.NH, ue.NCC)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func TestHandleSecurityMode_ValidSecurityContextWithBadAMFKey_UpdatesSecurityContextError(t *testing.T) {
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"1\"]",
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
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.State = amfContext.SecurityMode
	ue.SecurityContextAvailable = true
	ue.NgKsi = models.NgKsi{Ksi: 0}
	ue.MacFailed = false

	ue.Kgnb = []uint8{}
	ue.NH = []uint8{}
	ue.NCC = 0
	ue.Kamf = "this is not hex"

	expected := "error updating security context"

	msg := buildTestSecurityModeCompleteMessage()

	err = handleSecurityModeComplete(t.Context(), amf, ue, msg.SecurityModeComplete)
	if err == nil || !strings.HasPrefix(err.Error(), expected) {
		t.Fatalf("expected error starting with: %v, got: %v", expected, err)
	}

	if len(ue.Kgnb) != 0 || len(ue.NH) != 0 || ue.NCC != 0 {
		t.Fatalf("expected security context to be not be updated, got: Kgnb: %v, NH: %v, NCC: %v", ue.Kgnb, ue.NH, ue.NCC)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func TestHandleSecurityMode_NASMessageContainer_RegistrationAccepted(t *testing.T) {
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"1\"]",
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
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.State = amfContext.SecurityMode
	ue.Supi = "imsi-001019756139935"
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0
	ue.RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration

	msg, err := buildTestSecurityModeCompleteMessageWithRegistrationRequest()
	if err != nil {
		t.Fatalf("could not build security mode complete message with registration request: %v", err)
	}

	err = handleSecurityModeComplete(t.Context(), amf, ue, msg.SecurityModeComplete)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
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

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected a registration accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

func TestHandleSecurityMode_InvalidNASMessageContainer_Error(t *testing.T) {
	amf := &amfContext.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"1\"]",
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
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.State = amfContext.SecurityMode
	ue.Supi = "imsi-001019756139935"
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0
	ue.RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration

	msg, err := buildTestSecurityModeCompleteMessageWithRegistrationRequest()
	if err != nil {
		t.Fatalf("could not build security mode complete message with registration request: %v", err)
	}

	msg.SecurityModeComplete.SetNASMessageContainerContents([]uint8{0xDE, 0xAD, 0xBE, 0xEF})

	expected := "failed to decode nas message container"

	err = handleSecurityModeComplete(t.Context(), amf, ue, msg.SecurityModeComplete)
	if err == nil || !strings.HasPrefix(err.Error(), expected) {
		t.Fatalf("expected an error starting with: %v, got: %v", expected, err)
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
