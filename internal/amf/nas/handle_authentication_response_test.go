// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

// buildAuthResponsePlain builds a plain AUTHENTICATION RESPONSE. A nil res omits
// the RES* IE; a non-nil res (including empty) includes it (IEI 0x2D, TLV).
func buildAuthResponsePlain(res []byte) []byte {
	b := []byte{fgs.EPD5GMM, 0x00, uint8(fgs.MsgAuthenticationResponse)}
	if res != nil {
		b = append(b, 0x2D, uint8(len(res)))
		b = append(b, res...)
	}

	return b
}

// A missing RES* (nil authentication response parameter IE) is treated as an
// unsuccessful authentication per TS 24.501: a GUTI-identified UE is
// asked to identify via SUCI, a SUCI-identified UE is rejected.
func TestHandleAuthenticationResponse_NilAuthenticationResponseParameter(t *testing.T) {
	testcases := []struct {
		name    string
		idType  uint8
		msgType uint8
	}{
		// The AMF authenticates identify-first (on the UE's SUCI), so an
		// authentication failure is rejected regardless of the identity the UE
		// registered with — no redundant re-identification (mirrors the MME).
		{"used GUTI", nasMessage.MobileIdentity5GSType5gGuti, nas.MsgTypeAuthenticationReject},
		{"used SUCI", nasMessage.MobileIdentity5GSTypeSuci, nas.MsgTypeAuthenticationReject},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not create UE and radio: %v", err)
			}

			ue.ForceRegStepForTest(amf.RegStepAuthenticating)
			ue.Conn().AuthenticationCtx = &ausf.AuthResult{Rand: "DEADBEEF"}
			ue.Conn().IdentityTypeUsedForRegistration = tc.idType

			handleAuthenticationResponse(context.TODO(), amf.New(nil, nil, nil), ue, buildAuthResponsePlain(nil))

			if len(ngapSender.SentDownlinkNASTransport) != 1 {
				t.Fatalf("should have sent a Downlink NAS Transport message")
			}

			resp := ngapSender.SentDownlinkNASTransport[0]
			nm := new(nas.Message)
			nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

			if err := nm.PlainNasDecode(&resp.NasPdu); err != nil {
				t.Fatalf("could not decode plain NAS message: %v", err)
			}

			if nm.GmmHeader.GetMessageType() != tc.msgType {
				t.Fatalf("expected message of type %v, got %v", tc.msgType, nm.GmmHeader.GetMessageType())
			}
		})
	}
}

// Precondition failures (wrong state, missing authentication context, undecodable
// RAND) leave the authentication exchange untouched: no downlink is emitted.
func TestHandleAuthenticationResponse_PreconditionErrors(t *testing.T) {
	type TestCase struct {
		name  string
		setup func(*amf.UeContext)
	}

	testcases := []TestCase{
		{
			"wrong UE state",
			func(ue *amf.UeContext) {},
		},
		{
			"nil authentication context",
			func(ue *amf.UeContext) {
				ue.ForceRegStepForTest(amf.RegStepAuthenticating)
			},
		},
		{
			"invalid rand in UE context",
			func(ue *amf.UeContext) {
				ue.ForceRegStepForTest(amf.RegStepAuthenticating)
				ue.Conn().AuthenticationCtx = &ausf.AuthResult{Rand: "Not hex"}
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not create UE and radio: %v", err)
			}

			tc.setup(ue)

			handleAuthenticationResponse(context.TODO(), amf.New(nil, nil, nil), ue, buildAuthResponsePlain(make([]byte, 16)))

			if len(ngapSender.SentDownlinkNASTransport) != 0 {
				t.Fatalf("expected precondition failure to emit no downlink, but a Downlink NAS Transport was sent")
			}
		})
	}
}

func TestHandleAuthenticationResponse_TimerT3560Stopped(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	conn := ue.Conn()
	conn.AuthenticationCtx = &ausf.AuthResult{
		Rand:      "DEADBEEF",
		HxresStar: "not a match",
	}
	conn.IdentityTypeUsedForRegistration = nasMessage.MobileIdentity5GSTypeSuci
	conn.NASGuardForTest().Arm(10*time.Minute, 5, func(e int32) {}, func() {})

	handleAuthenticationResponse(t.Context(), amf.New(nil, nil, nil), ue, buildAuthResponsePlain(make([]byte, 16)))

	if conn.NASGuardForTest().Active() {
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
			nas.MsgTypeAuthenticationReject,
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

			ue.ForceRegStepForTest(amf.RegStepAuthenticating)
			ue.Conn().AuthenticationCtx = &ausf.AuthResult{
				Rand:      "DEADBEEF",
				HxresStar: "not a match",
			}
			ue.Conn().IdentityTypeUsedForRegistration = tc.id_type

			handleAuthenticationResponse(t.Context(), amf.New(nil, nil, nil), ue, buildAuthResponsePlain(make([]byte, 16)))

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
		state    amf.StateType
	}

	testcases := []TestCase{
		// Identify-first: an authentication failure rejects and deregisters
		// regardless of the registration identity (mirrors the MME).
		{
			"used GUTI",
			nasMessage.MobileIdentity5GSType5gGuti,
			nas.MsgTypeAuthenticationReject,
			amf.Deregistered,
		},
		{
			"used SUCI",
			nasMessage.MobileIdentity5GSTypeSuci,
			nas.MsgTypeAuthenticationReject,
			amf.Deregistered,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			amfInstance := amf.New(&fakeDBInstance{
				Operator: &db.Operator{
					Mcc:           "001",
					Mnc:           "01",
					SupportedTACs: "[\"1\"]",
				},
			}, &fakeAusf{
				AvKgAka: &ausf.AuthResult{
					Rand: hex.EncodeToString(make([]byte, 16)),
					Autn: hex.EncodeToString(make([]byte, 16)),
				},
				Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
				Kseaf: []byte("testkey"),
				Error: fmt.Errorf("failure"),
			}, nil)

			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not create UE and radio: %v", err)
			}

			ue.ForceRegStepForTest(amf.RegStepAuthenticating)
			ue.Conn().AuthenticationCtx = &ausf.AuthResult{
				Rand:      "DEADBEEF",
				HxresStar: "192a898722d89d0c3e4c6f2de48c796a",
			}
			ue.Conn().IdentityTypeUsedForRegistration = tc.id_type

			handleAuthenticationResponse(t.Context(), amfInstance, ue, buildAuthResponsePlain(make([]byte, 16)))

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

func TestHandleAuthenticationResponse_DeriveKamf_Success(t *testing.T) {
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"1\"]",
			Integrity:     `["SNOW3G","NULL"]`,
			Ciphering:     `["SNOW3G","NULL"]`,
		},
	}, &fakeAusf{
		AvKgAka: &ausf.AuthResult{
			Rand: hex.EncodeToString(make([]byte, 16)),
			Autn: hex.EncodeToString(make([]byte, 16)),
		},
		Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
		Kseaf: []byte{0xC0, 0xFF, 0xEE},
	}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepAuthenticating)
	ue.Conn().AuthenticationCtx = &ausf.AuthResult{
		Rand:      "DEADBEEF",
		HxresStar: "192a898722d89d0c3e4c6f2de48c796a",
	}
	ue.SetUESecurityCapabilityForTest(amf.UESecCapBytesForTest([]uint8{0, 1}, []uint8{0, 1}))

	handleAuthenticationResponse(t.Context(), amfInstance, ue, buildAuthResponsePlain(make([]byte, 16)))

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

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeSecurityModeCommand {
		t.Fatalf("expected a security mode command message, got '%v'", nm.GmmHeader.GetMessageType())
	}
}
