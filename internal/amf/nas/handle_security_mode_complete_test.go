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
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

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
				&fgs.SecurityModeComplete{},
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

	handleSecurityModeComplete(t.Context(), amfInstance, ue, msg, true)

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
	imeisv := fgs.PEIIdentity(fgs.PEI{Type: fgs.IdentityIMEISV, Digits: "3520990017614823"})
	msg.IMEISV = &imeisv

	handleSecurityModeComplete(t.Context(), amfInstance, ue, msg, true)

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

	handleSecurityModeComplete(t.Context(), amfInstance, ue, msg, true)

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

	if err := amfInstance.AdoptAuthenticatedSupi(context.TODO(), ue, mustSUPIFromPrefixed("imsi-001019756139935"), amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("AdoptAuthenticatedSupi: %v", err)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeInitial

	msg, err := buildTestSecurityModeCompleteMessageWithRegistrationRequest()
	if err != nil {
		t.Fatalf("could not build security mode complete message with registration request: %v", err)
	}

	handleSecurityModeComplete(t.Context(), amfInstance, ue, msg, true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgRegistrationAccept))
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

	if err := amfInstance.AdoptAuthenticatedSupi(context.TODO(), ue, mustSUPIFromPrefixed("imsi-001019756139935"), amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("AdoptAuthenticatedSupi: %v", err)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeInitial

	msg, err := buildTestSecurityModeCompleteMessageWithRegistrationRequest()
	if err != nil {
		t.Fatalf("could not build security mode complete message with registration request: %v", err)
	}

	msg.NASMessageContainer = []uint8{0xDE, 0xAD, 0xBE, 0xEF}

	handleSecurityModeComplete(t.Context(), amfInstance, ue, msg, true)

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatalf("expected a UE Context Release Command to release the aborted registration, got %d", len(ngapSender.SentUEContextReleaseCommand))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func TestHandleSecurityMode_PlainRegistrationNotReplayed_Aborts(t *testing.T) {
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

	if err := amfInstance.AdoptAuthenticatedSupi(context.TODO(), ue, mustSUPIFromPrefixed("imsi-001019756139935"), amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("AdoptAuthenticatedSupi: %v", err)
	}

	conn := ue.Conn()
	conn.RegistrationType5GS = fgs.RegistrationTypeInitial
	conn.RegistrationRequest = &fgs.RegistrationRequest{
		RegistrationType:     fgs.RegistrationTypeInitial,
		MobileIdentity:       testMobileIdentity(),
		UESecurityCapability: &fgs.UESecurityCapability{EA: 0xc0, IA: 0xc0},
	}
	conn.RegistrationRequestReplayRequired = true

	handleSecurityModeComplete(t.Context(), amfInstance, ue, buildTestSecurityModeCompleteMessage(), true)

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatalf("UE Context Release Command count is %d, want 1", len(ngapSender.SentUEContextReleaseCommand))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("Downlink NAS Transport count is %d, want 0", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestHandleSecurityMode_ProtectedRegistrationNeedsNoReplay(t *testing.T) {
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

	if err := amfInstance.AdoptAuthenticatedSupi(context.TODO(), ue, mustSUPIFromPrefixed("imsi-001019756139935"), amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("AdoptAuthenticatedSupi: %v", err)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(nas.CipheringAES)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	conn := ue.Conn()
	conn.RegistrationType5GS = fgs.RegistrationTypeInitial
	conn.RegistrationRequest = &fgs.RegistrationRequest{
		RegistrationType:     fgs.RegistrationTypeInitial,
		FOR:                  true,
		MobileIdentity:       testMobileIdentity(),
		UESecurityCapability: &fgs.UESecurityCapability{EA: 0xc0, IA: 0xc0},
	}
	conn.RegistrationRequestReplayRequired = false

	handleSecurityModeComplete(t.Context(), amfInstance, ue, buildTestSecurityModeCompleteMessage(), true)

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatalf("UE Context Release Command count is %d, want 0", len(ngapSender.SentUEContextReleaseCommand))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("Downlink NAS Transport count is %d, want 1", len(ngapSender.SentDownlinkNASTransport))
	}

	assertPlainGmm(t, ngapSender.SentDownlinkNASTransport[0].NASPDU, uint8(fgs.MsgRegistrationAccept))
}

func TestHandleSecurityMode_ProtectedRegistrationWithFailedMACNeedsNoReplay(t *testing.T) {
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

	if err := amfInstance.AdoptAuthenticatedSupi(context.TODO(), ue, mustSUPIFromPrefixed("imsi-001019756139935"), amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("AdoptAuthenticatedSupi: %v", err)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(nas.CipheringAES)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	m, err := buildTestRegistrationRequestMessage(nas.CipheringAES, &key, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	if err := handleRegistrationRequestMessage(t.Context(), amfInstance, ue,
		mustParseRegistrationRequest(t, m), m, false, false); err != nil {
		t.Fatalf("handle registration request message: %v", err)
	}

	ue.ForceRegStepForTest(amf.RegStepSecurityMode)

	handleSecurityModeComplete(t.Context(), amfInstance, ue, buildTestSecurityModeCompleteMessage(), true)

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatalf("UE Context Release Command count is %d, want 0; the registration was aborted", len(ngapSender.SentUEContextReleaseCommand))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("Downlink NAS Transport count is %d, want 1", len(ngapSender.SentDownlinkNASTransport))
	}

	assertPlainGmm(t, ngapSender.SentDownlinkNASTransport[0].NASPDU, uint8(fgs.MsgRegistrationAccept))
}

func buildTestSecurityModeCompleteMessage() *fgs.SecurityModeComplete {
	return &fgs.SecurityModeComplete{}
}

func buildTestSecurityModeCompleteMessageWithRegistrationRequest() (*fgs.SecurityModeComplete, error) {
	regReq, err := buildRegReqBytes(uint8(fgs.RegistrationTypeInitial), testMobileIdentity(), &fgs.UESecurityCapability{EA: 0xc0, IA: 0xc0}, 0, nil, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("could not encode registration request: %v", err)
	}

	return &fgs.SecurityModeComplete{NASMessageContainer: regReq}, nil
}
