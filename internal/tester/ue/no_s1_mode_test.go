// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"testing"

	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestRegistrationRequestOfAUEWithoutS1Mode(t *testing.T) {
	for _, noS1Mode := range []bool{false, true} {
		sec := goldenUESecurity()
		sec.NoS1Mode = noS1Mode

		_, complete, err := buildRegistrationRequest(&RegistrationRequestOpts{
			RegistrationType:  uint8(fgs.RegistrationTypeInitial),
			IncludeCapability: true,
			UESecurity:        sec,
		})
		if err != nil {
			t.Fatalf("build (no S1 mode %v): %v", noS1Mode, err)
		}

		msg, err := fgs.ParseRegistrationRequest(complete)
		if err != nil {
			t.Fatalf("parse (no S1 mode %v): %v", noS1Mode, err)
		}

		if msg.GMMCapability == nil {
			t.Fatalf("no 5GMM capability (no S1 mode %v)", noS1Mode)
		}

		if msg.GMMCapability.S1Mode == noS1Mode {
			t.Errorf("S1 mode = %v, want %v", msg.GMMCapability.S1Mode, !noS1Mode)
		}

		if !msg.GMMCapability.LPP {
			t.Errorf("LPP not set (no S1 mode %v)", noS1Mode)
		}
	}
}

func TestNewUEWithoutS1ModeSendsNoS1UENetworkCapability(t *testing.T) {
	u, err := NewUE(&UEOpts{
		Msin: "0000000001", Mcc: "001", Mnc: "01", RoutingIndicator: "0000",
		K: "465B5CE8B199B49FAA5F0A2EE238A6BC", OpC: "E8ED289DEBA952E4283B54E88E6183CA",
		Amf: "8000", Sqn: "000000000001",
		HomeNetworkPublicKey: sidf.HomeNetworkPublicKey{ProtectionScheme: sidf.NullScheme, PublicKeyID: "0"},
		NoS1Mode:             true,
	})
	if err != nil {
		t.Fatalf("new UE: %v", err)
	}

	if !u.UeSecurity.NoS1Mode || u.UeSecurity.S1UENetworkCapability != nil {
		t.Fatalf("UE without S1 mode: no S1 mode %v, S1 UE network capability %v", u.UeSecurity.NoS1Mode, u.UeSecurity.S1UENetworkCapability)
	}
}
