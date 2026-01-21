package gmm

import (
	"context"
	"encoding/hex"
	"fmt"
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

func TestHandleAuthenticationResponse_PreconditionErrors(t *testing.T) {
	type TestCase struct {
		name string
		ue   *amfContext.AmfUe
		err  error
	}

	testcases := []TestCase{
		{
			"wrong UE state",
			&amfContext.AmfUe{State: amfContext.Deregistered},
			fmt.Errorf("state mismatch: receive Authentication Response message in state %s", amfContext.Deregistered),
		},
		{
			"nil authentication context",
			&amfContext.AmfUe{State: amfContext.Authentication, AuthenticationCtx: nil},
			fmt.Errorf("ue Authentication Context is nil"),
		},
		{
			"invalid rand in UE context",
			&amfContext.AmfUe{State: amfContext.Authentication, AuthenticationCtx: &models.Av5gAka{Rand: "Not hex"}},
			fmt.Errorf("failed to decode RAND: encoding/hex: invalid byte: U+004E 'N'"),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := handleAuthenticationResponse(context.TODO(), &amfContext.AMF{}, tc.ue, &nasMessage.AuthenticationResponse{AuthenticationResponseParameter: &nasType.AuthenticationResponseParameter{}})
			if err == nil || err.Error() != tc.err.Error() {
				t.Fatalf("expected error: %v, got: %v", tc.err, err)
			}
		})
	}
}

func TestHandleAuthenticationResponse_TimerT3560Stopped(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.State = amfContext.Authentication
	ue.AuthenticationCtx = &models.Av5gAka{
		Rand:      "DEADBEEF",
		HxresStar: "not a match",
	}
	ue.IdentityTypeUsedForRegistration = nasMessage.MobileIdentity5GSTypeSuci
	ue.T3560 = amfContext.NewTimer(10*time.Minute, 5, func(e int32) {}, func() {})

	_ = handleAuthenticationResponse(t.Context(), &amfContext.AMF{}, ue, &nasMessage.AuthenticationResponse{AuthenticationResponseParameter: &nasType.AuthenticationResponseParameter{}})

	if ue.T3560 != nil {
		t.Fatal("expected timer T3560 to be stopped and cleared")
	}
}

func TestHandleAuthenticationResponse_hResStartMismatch(t *testing.T) {
	type TestCase struct {
		name     string
		id_type  uint8
		msg_type uint8
	}

	testcases := []TestCase{
		{
			"used GUTI",
			nasMessage.MobileIdentity5GSType5gGuti,
			nas.MsgTypeIdentityRequest,
		},
		{
			"used SUCI",
			nasMessage.MobileIdentity5GSTypeSuci,
			nas.MsgTypeAuthenticationReject,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not create UE and radio: %v", err)
			}

			ue.State = amfContext.Authentication
			ue.AuthenticationCtx = &models.Av5gAka{
				Rand:      "DEADBEEF",
				HxresStar: "not a match",
			}
			ue.IdentityTypeUsedForRegistration = tc.id_type

			err = handleAuthenticationResponse(t.Context(), &amfContext.AMF{}, ue, &nasMessage.AuthenticationResponse{AuthenticationResponseParameter: &nasType.AuthenticationResponseParameter{}})
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

			if nm.GmmHeader.GetMessageType() != tc.msg_type {
				t.Fatalf("expected message of type: '%v', got '%v'", tc.msg_type, nm.GmmHeader.GetMessageType())
			}
		})
	}
}

func TestHandleAuthenticationResponse_Auth5gAKA_Failure(t *testing.T) {
	type TestCase struct {
		name     string
		id_type  uint8
		msg_type uint8
		state    amfContext.StateType
	}

	testcases := []TestCase{
		{
			"used GUTI",
			nasMessage.MobileIdentity5GSType5gGuti,
			nas.MsgTypeIdentityRequest,
			amfContext.Authentication,
		},
		{
			"used SUCI",
			nasMessage.MobileIdentity5GSTypeSuci,
			nas.MsgTypeAuthenticationReject,
			amfContext.Deregistered,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
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
					Error: fmt.Errorf("failure"),
				},
				UEs: make(map[string]*amfContext.AmfUe),
			}

			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not create UE and radio: %v", err)
			}

			ue.State = amfContext.Authentication
			ue.AuthenticationCtx = &models.Av5gAka{
				Rand:      "DEADBEEF",
				HxresStar: "192a898722d89d0c3e4c6f2de48c796a",
			}
			ue.IdentityTypeUsedForRegistration = tc.id_type

			err = handleAuthenticationResponse(t.Context(), amf, ue, &nasMessage.AuthenticationResponse{AuthenticationResponseParameter: &nasType.AuthenticationResponseParameter{}})
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

			if nm.GmmHeader.GetMessageType() != tc.msg_type {
				t.Fatalf("expected message of type: '%v', got '%v'", tc.msg_type, nm.GmmHeader.GetMessageType())
			}
		})
	}
}

func TestHandleAuthenticationResponse_DeriveKamf_Failure(t *testing.T) {
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
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.State = amfContext.Authentication
	ue.AuthenticationCtx = &models.Av5gAka{
		Rand:      "DEADBEEF",
		HxresStar: "192a898722d89d0c3e4c6f2de48c796a",
	}

	expected := "couldn't derive Kamf: could not decode kseaf: encoding/hex: invalid byte: U+0074 't'"
	err = handleAuthenticationResponse(t.Context(), amf, ue, &nasMessage.AuthenticationResponse{AuthenticationResponseParameter: &nasType.AuthenticationResponseParameter{}})

	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %v, got: %v", expected, err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}
}

func TestHandleAuthenticationResponse_DeriveKamf_Success(t *testing.T) {
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
			Kseaf: "C0FFEE",
		},
		UEs: make(map[string]*amfContext.AmfUe),
	}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.State = amfContext.Authentication
	ue.AuthenticationCtx = &models.Av5gAka{
		Rand:      "DEADBEEF",
		HxresStar: "192a898722d89d0c3e4c6f2de48c796a",
	}
	ue.UESecurityCapability = &nasType.UESecurityCapability{
		Iei:    nasMessage.RegistrationRequestUESecurityCapabilityType,
		Len:    2,
		Buffer: []uint8{0x00, 0x00},
	}
	ue.UESecurityCapability.SetEA0_5G(1)
	ue.UESecurityCapability.SetEA1_128_5G(1)
	ue.UESecurityCapability.SetEA2_128_5G(1)
	ue.UESecurityCapability.SetEA2_128_5G(0)
	ue.UESecurityCapability.SetIA0_5G(1)
	ue.UESecurityCapability.SetIA1_128_5G(1)
	ue.UESecurityCapability.SetIA2_128_5G(1)
	ue.UESecurityCapability.SetIA2_128_5G(0)

	err = handleAuthenticationResponse(t.Context(), amf, ue, &nasMessage.AuthenticationResponse{AuthenticationResponseParameter: &nasType.AuthenticationResponseParameter{}})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
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

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext {
		t.Fatalf("expected a protected with new 5g NAS security context NAS message, got: %v", nm.SecurityHeaderType)
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeSecurityModeCommand {
		t.Fatalf("expected a security mode command message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}
