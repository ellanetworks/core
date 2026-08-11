// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"testing"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap/zapcore"
)

func containedRegistrationRequest(t *testing.T, sec *UESecurity) *fgs.RegistrationRequest {
	t.Helper()

	raw, err := BuildSecurityModeComplete(&SecurityModeCompleteOpts{
		UESecurity: sec,
		IMEISV:     "1234567890123456",
	})
	if err != nil {
		t.Fatalf("BuildSecurityModeComplete: %v", err)
	}

	smc, err := fgs.ParseSecurityModeComplete(raw)
	if err != nil {
		t.Fatalf("parse SECURITY MODE COMPLETE: %v", err)
	}

	if smc.NASMessageContainer == nil {
		t.Fatal("SECURITY MODE COMPLETE carries no NAS message container")
	}

	req, err := fgs.ParseRegistrationRequest(smc.NASMessageContainer)
	if err != nil {
		t.Fatalf("parse the contained REGISTRATION REQUEST: %v", err)
	}

	return req
}

// TS 24.501 §4.4.6
func TestSecurityModeCompleteCarriesTheS1UENetworkCapability(t *testing.T) {
	sec := goldenUESecurity()
	capability := DefaultS1UENetworkCapability
	sec.S1UENetworkCapability = &capability

	req := containedRegistrationRequest(t, sec)

	if req.S1UENetworkCapability == nil {
		t.Fatal("the contained REGISTRATION REQUEST carries no S1 UE network capability")
	}

	got, err := eps.ParseUENetworkCapability(req.S1UENetworkCapability)
	if err != nil {
		t.Fatalf("parse S1 UE network capability: %v", err)
	}

	if got.EEA != capability.EEA || got.EIA != capability.EIA {
		t.Errorf("S1 UE network capability = EEA %08b / EIA %08b, want EEA %08b / EIA %08b",
			got.EEA, got.EIA, capability.EEA, capability.EIA)
	}

	if got.EIA&0x7f == 0 {
		t.Error("the default S1 UE network capability offers no non-null EPS integrity algorithm")
	}
}

func TestSecurityModeCompleteOmitsTheS1UENetworkCapabilityWhenUnset(t *testing.T) {
	sec := goldenUESecurity()
	sec.S1UENetworkCapability = nil

	if req := containedRegistrationRequest(t, sec); req.S1UENetworkCapability != nil {
		t.Error("a UE with no EPS algorithm support offered one anyway")
	}
}

// TS 24.501 §4.4.6
func TestInitialRegistrationRequestOmitsTheS1UENetworkCapability(t *testing.T) {
	sec := goldenUESecurity()
	capability := DefaultS1UENetworkCapability
	sec.S1UENetworkCapability = &capability

	raw, err := BuildRegistrationRequest(&RegistrationRequestOpts{
		RegistrationType:  uint8(fgs.RegistrationTypeInitial),
		IncludeCapability: false,
		UESecurity:        sec,
	})
	if err != nil {
		t.Fatalf("BuildRegistrationRequest: %v", err)
	}

	req, err := fgs.ParseRegistrationRequest(raw)
	if err != nil {
		t.Fatalf("parse REGISTRATION REQUEST: %v", err)
	}

	if req.S1UENetworkCapability != nil {
		t.Error("the cleartext initial REGISTRATION REQUEST carries the S1 UE network capability")
	}
}

// TS 33.501 §6.7.2
func TestSecurityModeCommandStoresTheEPSNASAlgorithms(t *testing.T) {
	sec := goldenUESecurity()

	if sec.EPSNASAlgorithms != nil {
		t.Fatal("a fresh UE already holds EPS NAS algorithms")
	}

	logger.Init(zapcore.ErrorLevel)

	ue := &UE{UeSecurity: sec}

	smc := &fgs.SecurityModeCommand{
		SelectedEPSNASSecurityAlgorithms: &fgs.SelectedEPSNASSecurityAlgorithms{
			Ciphering: 2, // 128-EEA2
			Integrity: 2, // 128-EIA2
		},
	}

	storeEPSNASAlgorithms(ue, smc)

	got := ue.UeSecurity.EPSNASAlgorithms
	if got == nil {
		t.Fatal("the UE did not store the selected EPS NAS algorithms")
	}

	if got.Ciphering != 2 || got.Integrity != 2 {
		t.Errorf("EPS NAS algorithms = (%d, %d), want the pair the AMF selected", got.Ciphering, got.Integrity)
	}
}
