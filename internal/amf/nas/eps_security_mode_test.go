// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

func epsInterworkingAMF() *amf.AMF {
	a := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: `["000001"]`,
			Ciphering:     `["AES"]`,
			Integrity:     `["AES"]`,
		},
	}, nil, nil)

	return a
}

func securityModeCommandsIn(t *testing.T, sent []*ngap.DownlinkNASTransport) int {
	t.Helper()

	n := 0

	for _, pdu := range sent {
		// TS 24.501 §4.4.5
		spm, err := fgs.ParseSecurityProtectedMessage(pdu.NASPDU)
		if err != nil {
			continue
		}

		msgType, err := fgs.PeekMessageType(spm.UnverifiedPayload)
		if err != nil {
			t.Fatalf("peek message type: %v", err)
		}

		if msgType == fgs.MsgSecurityModeCommand {
			n++
		}
	}

	return n
}

func registeredS1UE(t *testing.T) (*amf.UeContext, *fakeNGAPSender) {
	t.Helper()

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("build UE and radio: %v", err)
	}

	ue.SetSecuredForTest(true)

	ng := ue.NgKsiForTest()
	ng.Ksi = 1
	ue.SetNgKsiForTest(ng)

	ue.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0})
	ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, []byte{0x70, 0x70, 0x00, 0x00})

	conn := ue.Conn()
	conn.RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating
	conn.RegistrationRequest = &fgs.RegistrationRequest{RegistrationType: fgs.RegistrationTypeMobilityUpdating}

	return ue, ngapSender
}

// TS 24.501 §5.4.2.2
func TestSecurityMode_DeliversEPSNASAlgorithmsOnValidContext(t *testing.T) {
	ue, ngapSender := registeredS1UE(t)

	securityMode(context.Background(), epsInterworkingAMF(), ue)

	if got := securityModeCommandsIn(t, ngapSender.SentDownlinkNASTransport); got != 1 {
		t.Fatalf("sent %d SECURITY MODE COMMANDs, want 1", got)
	}

	if !ue.Procedures().Active(procedure.SecurityMode) {
		t.Error("the security mode procedure must stay claimed until SECURITY MODE COMPLETE")
	}

	if _, ok := ue.EPSNASAlgorithmsInUse(); ok {
		t.Error("the algorithms must not count as held before the UE accepts them")
	}

	ue.MarkEPSNASAlgorithmsDelivered()

	got, ok := ue.EPSNASAlgorithmsInUse()
	if !ok {
		t.Fatal("the UE holds no EPS NAS algorithms after accepting the command")
	}

	if got.Ciphering != nas.CipheringAES || got.Integrity != nas.IntegrityAES {
		t.Fatalf("EPS NAS algorithms = (%s, %s), want the operator's AES pair", got.Ciphering, got.Integrity)
	}
}

func TestSecurityMode_EPSNASAlgorithmsDeliveredOnlyOnce(t *testing.T) {
	ue, ngapSender := registeredS1UE(t)
	amfInstance := epsInterworkingAMF()

	securityMode(context.Background(), amfInstance, ue)
	ue.MarkEPSNASAlgorithmsDelivered()
	ue.EndKeyChainProc(procedure.SecurityMode)

	securityMode(context.Background(), amfInstance, ue)

	if got := securityModeCommandsIn(t, ngapSender.SentDownlinkNASTransport); got != 1 {
		t.Fatalf("sent %d SECURITY MODE COMMANDs across both registrations, want 1", got)
	}
}

func initialRegistrationS1UE(t *testing.T) (*amf.UeContext, *fakeNGAPSender) {
	t.Helper()

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("build UE and radio: %v", err)
	}

	ue.SetSecuredForTest(true)

	ng := ue.NgKsiForTest()
	ng.Ksi = 1
	ue.SetNgKsiForTest(ng)

	ue.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0})
	ue.SetKamfForTest("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")

	if err := ue.InstallNASSecurityContext(nas.CipheringAES, nas.IntegrityAES, amf.MintAuthProofForSecurityMode()); err != nil {
		t.Fatalf("install NAS security context: %v", err)
	}

	ue.ForceStateForTest(amf.RegistrationInitiated)
	ue.ForceRegStepForTest(amf.RegStepSecurityMode)

	conn := ue.Conn()
	conn.RegistrationType5GS = fgs.RegistrationTypeInitial
	conn.RegistrationRequestReplayRequired = true

	return ue, ngapSender
}

func replayedRegistrationContainer(t *testing.T) []byte {
	t.Helper()

	raw, err := (&fgs.RegistrationRequest{
		RegistrationType:      fgs.RegistrationTypeInitial,
		NgKSI:                 nas.KeySetIdentifier{Value: 1},
		MobileIdentity:        mappedGUTIIdentity(),
		UESecurityCapability:  &fgs.UESecurityCapability{EA: 0xE0, IA: 0xE0},
		GMMCapability:         &fgs.GMMCapability{S1Mode: true},
		S1UENetworkCapability: []byte{0x70, 0x70, 0x00, 0x00},
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("build replayed REGISTRATION REQUEST: %v", err)
	}

	return raw
}

// TS 24.501 §5.4.2.4
func TestSecurityModeComplete_InitialRegistration_ProvidesEPSNASAlgorithms(t *testing.T) {
	ue, ngapSender := initialRegistrationS1UE(t)
	amfInstance := epsInterworkingAMF()

	handleSecurityModeComplete(t.Context(), amfInstance, ue,
		&fgs.SecurityModeComplete{NASMessageContainer: replayedRegistrationContainer(t)}, true)

	if got := securityModeCommandsIn(t, ngapSender.SentDownlinkNASTransport); got != 1 {
		t.Fatalf("sent %d SECURITY MODE COMMANDs after the replayed registration, want 1", got)
	}

	smc := parseSentSecurityModeCommand(t, ngapSender.SentDownlinkNASTransport)
	if smc.SelectedEPSNASSecurityAlgorithms == nil {
		t.Fatal("the second SECURITY MODE COMMAND carries no Selected EPS NAS security algorithms IE")
	}

	if ue.RegStep() != amf.RegStepSecurityMode {
		t.Errorf("registration step = %v, want the registration deferred until the second SECURITY MODE COMPLETE", ue.RegStep())
	}

	if _, ok := ue.EPSNASAlgorithmsInUse(); ok {
		t.Error("the algorithms must not count as held before the UE accepts them")
	}

	if ue.Conn().RegistrationRequestReplayRequired {
		t.Error("the replay requirement must be cleared once the container has arrived")
	}

	handleSecurityModeComplete(t.Context(), amfInstance, ue, &fgs.SecurityModeComplete{}, true)

	got, ok := ue.EPSNASAlgorithmsInUse()
	if !ok {
		t.Fatal("the UE holds no EPS NAS algorithms after the second SECURITY MODE COMPLETE")
	}

	if got.Ciphering != nas.CipheringAES || got.Integrity != nas.IntegrityAES {
		t.Fatalf("EPS NAS algorithms = (%s, %s), want the operator's AES pair", got.Ciphering, got.Integrity)
	}

	if ue.RegStep() == amf.RegStepSecurityMode {
		t.Error("the registration must resume once the second SECURITY MODE COMPLETE arrives")
	}
}

func parseSentSecurityModeCommand(t *testing.T, sent []*ngap.DownlinkNASTransport) *fgs.SecurityModeCommand {
	t.Helper()

	for _, pdu := range sent {
		spm, err := fgs.ParseSecurityProtectedMessage(pdu.NASPDU)
		if err != nil {
			continue
		}

		if mt, err := fgs.PeekMessageType(spm.UnverifiedPayload); err != nil || mt != fgs.MsgSecurityModeCommand {
			continue
		}

		smc, err := fgs.ParseSecurityModeCommand(spm.UnverifiedPayload)
		if err != nil {
			t.Fatalf("parse SECURITY MODE COMMAND: %v", err)
		}

		return smc
	}

	t.Fatal("no SECURITY MODE COMMAND was sent")

	return nil
}
