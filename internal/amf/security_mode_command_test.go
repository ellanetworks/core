// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/nas/fgs"
)

func securityModeCommandForTest(t *testing.T, ue *amf.UeContext) *fgs.SecurityModeCommand {
	t.Helper()

	raw, err := amf.BuildSecurityModeCommand(ue)
	if err != nil {
		t.Fatalf("BuildSecurityModeCommand: %v", err)
	}

	smc, err := fgs.ParseSecurityModeCommand(raw)
	if err != nil {
		t.Fatalf("parse SecurityModeCommand: %v", err)
	}

	return smc
}

func horizontalDerivationRequired(smc *fgs.SecurityModeCommand) bool {
	return smc.AdditionalSecurityInformation != nil && smc.AdditionalSecurityInformation.HDP
}

func securityModeCommandUE(t *testing.T, origin amf.KeyOrigin, regType fgs.RegistrationType) *amf.UeContext {
	t.Helper()

	ue := buildTestUE(t)
	ue.SetUESecurityCapabilityForTest(amf.UESecCapForTest([]uint8{2}, []uint8{2}))
	attachTestConn(t, ue)
	ue.SetKeyOriginForTest(origin)

	conn := ue.Conn()
	if conn == nil {
		t.Fatal("no NAS connection")
	}

	conn.SetRegistrationType5GS(uint8(regType))

	return ue
}

func TestSecurityModeCommandDoesNotClaimHorizontalDerivationAfterPrimaryAuth(t *testing.T) {
	for _, regType := range []fgs.RegistrationType{
		fgs.RegistrationTypeInitial,
		fgs.RegistrationTypeMobilityUpdating,
		fgs.RegistrationTypePeriodicUpdating,
		fgs.RegistrationTypeEmergency,
	} {
		ue := securityModeCommandUE(t, amf.KeyOriginPrimaryAuth, regType)

		if horizontalDerivationRequired(securityModeCommandForTest(t, ue)) {
			t.Errorf("registration type %v: HDP is set, but the KAMF comes from primary authentication, not from a KAMF to K'AMF derivation", regType)
		}
	}
}

func TestSecurityModeCommandDoesNotClaimHorizontalDerivationForAMappedContext(t *testing.T) {
	ue := securityModeCommandUE(t, amf.KeyOriginMappedFromEPS, fgs.RegistrationTypeMobilityUpdating)

	if horizontalDerivationRequired(securityModeCommandForTest(t, ue)) {
		t.Error("HDP is set, but a mapped KAMF' is derived from the KASME, not horizontally")
	}
}

func TestSecurityModeCommandSignalsHorizontalDerivation(t *testing.T) {
	ue := securityModeCommandUE(t, amf.KeyOriginHorizontalDerivation, fgs.RegistrationTypeMobilityUpdating)

	if !horizontalDerivationRequired(securityModeCommandForTest(t, ue)) {
		t.Error("HDP is not set for a horizontally derived KAMF")
	}
}

func TestSecurityModeCommandOmitsAdditionalSecurityInformationWhenNothingToSignal(t *testing.T) {
	ue := securityModeCommandUE(t, amf.KeyOriginPrimaryAuth, fgs.RegistrationTypeMobilityUpdating)

	smc := securityModeCommandForTest(t, ue)
	if smc.AdditionalSecurityInformation != nil {
		t.Fatalf("Additional 5G security information = %+v, want the IE omitted", *smc.AdditionalSecurityInformation)
	}
}

func TestSecurityModeCommandRequestsRetransmissionWithoutHorizontalDerivation(t *testing.T) {
	ue := securityModeCommandUE(t, amf.KeyOriginPrimaryAuth, fgs.RegistrationTypeMobilityUpdating)

	conn := ue.Conn()
	conn.RetransmissionOfInitialNASMsg = true

	smc := securityModeCommandForTest(t, ue)
	if smc.AdditionalSecurityInformation == nil {
		t.Fatal("the Additional 5G security information IE is missing")
	}

	if !smc.AdditionalSecurityInformation.RINMR || smc.AdditionalSecurityInformation.HDP {
		t.Fatalf("Additional 5G security information = %+v, want RINMR alone", *smc.AdditionalSecurityInformation)
	}
}
