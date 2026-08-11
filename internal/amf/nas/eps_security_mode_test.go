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

// epsInterworkingAMF is an AMF running N26, with an AES-only operator policy.
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

// securityModeCommandsIn counts the SECURITY MODE COMMANDs among the downlink
// messages sent. A registration that skips the security mode procedure still
// answers with a REGISTRATION ACCEPT, so the count matters, not the total.
func securityModeCommandsIn(t *testing.T, sent []*ngap.DownlinkNASTransport) int {
	t.Helper()

	n := 0

	for _, pdu := range sent {
		// The command is integrity protected and not ciphered (TS 24.501 §4.4.5),
		// so the plain message it wraps is readable without keys.
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

// registeredS1UE is a UE holding a current 5G NAS security context that has just
// sent a protected REGISTRATION REQUEST disclosing S1 mode support and its EPS
// algorithms — the state TS 24.501 §5.4.2.2 addresses.
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
	// 128-EEA1/2/3 and 128-EIA1/2/3 in EPS; no UMTS algorithms.
	ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, []byte{0x70, 0x70, 0x00, 0x00})

	conn := ue.Conn()
	conn.RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating
	conn.RegistrationRequest = &fgs.RegistrationRequest{RegistrationType: fgs.RegistrationTypeMobilityUpdating}

	return ue, ngapSender
}

// A UE whose security context needs no change, but which has never been told its
// EPS NAS algorithms, gets the security mode command that carries them; the
// registration then continues from SECURITY MODE COMPLETE (TS 24.501 §5.4.2.2).
func TestSecurityMode_DeliversEPSNASAlgorithmsOnValidContext(t *testing.T) {
	ue, ngapSender := registeredS1UE(t)

	securityMode(context.Background(), epsInterworkingAMF(true), ue)

	if got := securityModeCommandsIn(t, ngapSender.SentDownlinkNASTransport); got != 1 {
		t.Fatalf("sent %d SECURITY MODE COMMANDs, want 1", got)
	}

	if !ue.Procedures().Active(procedure.SecurityMode) {
		t.Error("the security mode procedure must stay claimed until SECURITY MODE COMPLETE")
	}

	// Offered, not yet in use: the UE has not answered.
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

// Nothing is offered and no extra round trip is spent while the AMF has no N26
// interface to offer: the IWK N26 bit it advertises says the same thing, and the
// two must not disagree (TS 24.501 §8.2.25.4, §9.11.3.5).
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

// A UE that has already been given its algorithms is not asked again.
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
