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

func epsInterworkingAMF(n26 bool) *amf.AMF {
	a := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: `["000001"]`,
			Ciphering:     `["AES"]`,
			Integrity:     `["AES"]`,
		},
	}, nil, nil)
	a.N26Enabled = n26

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

	securityMode(context.Background(), epsInterworkingAMF(true), ue)

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

// TS 24.501 §8.2.25.4, §9.11.3.5
func TestSecurityMode_NoEPSNASAlgorithmsWithoutN26(t *testing.T) {
	ue, ngapSender := registeredS1UE(t)

	securityMode(context.Background(), epsInterworkingAMF(false), ue)

	if got := securityModeCommandsIn(t, ngapSender.SentDownlinkNASTransport); got != 0 {
		t.Fatalf("sent %d SECURITY MODE COMMANDs, want none", got)
	}

	if ue.Procedures().Active(procedure.SecurityMode) {
		t.Error("no security mode procedure must be claimed")
	}
}

func TestSecurityMode_EPSNASAlgorithmsDeliveredOnlyOnce(t *testing.T) {
	ue, ngapSender := registeredS1UE(t)
	amfInstance := epsInterworkingAMF(true)

	securityMode(context.Background(), amfInstance, ue)
	ue.MarkEPSNASAlgorithmsDelivered()
	ue.EndKeyChainProc(procedure.SecurityMode)

	securityMode(context.Background(), amfInstance, ue)

	if got := securityModeCommandsIn(t, ngapSender.SentDownlinkNASTransport); got != 1 {
		t.Fatalf("sent %d SECURITY MODE COMMANDs across both registrations, want 1", got)
	}
}
