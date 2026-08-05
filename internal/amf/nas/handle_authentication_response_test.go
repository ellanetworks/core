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
)

// buildAuthResponsePlain builds a plain AUTHENTICATION RESPONSE. A nil res omits
// the RES* IE; a non-nil res (including empty) includes it (IEI 0x2D, TLV).
func buildAuthResponse(res []byte) *fgs.AuthenticationResponse {
	return &fgs.AuthenticationResponse{RES: res}
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
		{"used GUTI", uint8(fgs.IdentityGUTI), uint8(fgs.MsgAuthenticationReject)},
		{"used SUCI", uint8(fgs.IdentitySUCI), uint8(fgs.MsgAuthenticationReject)},
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

			handleAuthenticationResponse(context.TODO(), amf.New(nil, nil, nil), ue, buildAuthResponse(nil))

			if len(ngapSender.SentDownlinkNASTransport) != 1 {
				t.Fatalf("should have sent a Downlink NAS Transport message")
			}

			resp := ngapSender.SentDownlinkNASTransport[0]
			assertPlainGmm(t, resp.NASPDU, tc.msgType)
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

			handleAuthenticationResponse(context.TODO(), amf.New(nil, nil, nil), ue, buildAuthResponse(make([]byte, 16)))

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
	conn.IdentityTypeUsedForRegistration = uint8(fgs.IdentitySUCI)
	conn.NASGuardForTest().Arm(10*time.Minute, 5, func(e int32) {}, func() {})

	handleAuthenticationResponse(t.Context(), amf.New(nil, nil, nil), ue, buildAuthResponse(make([]byte, 16)))

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
			uint8(fgs.IdentityGUTI),
			uint8(fgs.MsgAuthenticationReject),
		},
		{
			"used SUCI",
			uint8(fgs.IdentitySUCI),
			uint8(fgs.MsgAuthenticationReject),
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

			handleAuthenticationResponse(t.Context(), amf.New(nil, nil, nil), ue, buildAuthResponse(make([]byte, 16)))

			if len(ngapSender.SentDownlinkNASTransport) != 1 {
				t.Fatalf("should have sent a Downlink NAS Transport message")
			}

			resp := ngapSender.SentDownlinkNASTransport[0]
			assertPlainGmm(t, resp.NASPDU, tc.msg_type)
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
			uint8(fgs.IdentityGUTI),
			uint8(fgs.MsgAuthenticationReject),
			amf.Deregistered,
		},
		{
			"used SUCI",
			uint8(fgs.IdentitySUCI),
			uint8(fgs.MsgAuthenticationReject),
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

			handleAuthenticationResponse(t.Context(), amfInstance, ue, buildAuthResponse(make([]byte, 16)))

			if len(ngapSender.SentDownlinkNASTransport) != 1 {
				t.Fatalf("should have sent a Downlink NAS Transport message")
			}

			resp := ngapSender.SentDownlinkNASTransport[0]
			assertPlainGmm(t, resp.NASPDU, tc.msg_type)
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
	ue.SetUESecurityCapabilityForTest(amf.UESecCapForTest([]uint8{0, 1}, []uint8{0, 1}))

	handleAuthenticationResponse(t.Context(), amfInstance, ue, buildAuthResponse(make([]byte, 16)))

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]

	if fgs.SecurityHeaderType(resp.NASPDU[1]&0x0f) != fgs.SHTIntegrityProtectedNewContext {
		t.Fatalf("expected a protected with new 5g NAS security context NAS message, got: %v", resp.NASPDU[1]&0x0f)
	}

	inner := resp.NASPDU[7:]
	if len(inner) < 3 || inner[2] != uint8(fgs.MsgSecurityModeCommand) {
		t.Fatalf("expected a security mode command message, got '%v'", inner[2])
	}
}
