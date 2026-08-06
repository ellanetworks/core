// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func buildTestUE(t *testing.T) *amf.UeContext {
	t.Helper()

	ue := amf.NewUeContext()
	ue.SetSecuredForTest(true)

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(nas.CipheringAES)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	return ue
}

// decryptNAS unwraps a downlink protected NAS message to its plaintext (the
// message is encoded at NAS COUNT 0 by the builders under test).
func decryptNAS(t *testing.T, ue *amf.UeContext, raw []byte) []byte {
	t.Helper()

	plain, err := unprotected(fgs.Unprotect(raw, 0, nas.DirectionDownlink, mustSecurityContext(t, ue.IntegrityAlgForTest(), ue.CipheringAlgForTest(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("NAS unprotect failed: %v", err)
	}

	return plain
}

func TestBuildConfigurationUpdateCommand_WithoutGUTI(t *testing.T) {
	ue := buildTestUE(t)

	raw, err := amf.BuildConfigurationUpdateCommand(amf.New(nil, nil, nil), ue, etsi.InvalidGUTI5G, "ELLACORE5G", "ELLACORE", false)
	if err != nil {
		t.Fatalf("BuildConfigurationUpdateCommand failed: %v", err)
	}

	cuc, err := fgs.ParseConfigurationUpdateCommand(decryptNAS(t, ue, raw))
	if err != nil {
		t.Fatalf("parse ConfigurationUpdateCommand: %v", err)
	}

	if cuc.GUTI != nil {
		t.Fatal("expected GUTI to be absent when includeGUTI is false")
	}

	if cuc.FullNameForNetwork == nil {
		t.Fatal("expected FullNameForNetwork to be present")
	}

	if cuc.ShortNameForNetwork == nil {
		t.Fatal("expected ShortNameForNetwork to be present")
	}
}

func TestBuildConfigurationUpdateCommand_WithGUTI(t *testing.T) {
	ue := buildTestUE(t)

	tmsi, err := etsi.NewTMSI(1)
	if err != nil {
		t.Fatalf("failed to create TMSI: %v", err)
	}

	guti, err := etsi.NewGUTI5G("001", "01", "000001", tmsi)
	if err != nil {
		t.Fatalf("failed to create GUTI: %v", err)
	}

	ue.SetGutiForTest(guti)

	raw, err := amf.BuildConfigurationUpdateCommand(amf.New(nil, nil, nil), ue, guti, "ELLACORE5G", "ELLACORE", true)
	if err != nil {
		t.Fatalf("BuildConfigurationUpdateCommand failed: %v", err)
	}

	cuc, err := fgs.ParseConfigurationUpdateCommand(decryptNAS(t, ue, raw))
	if err != nil {
		t.Fatalf("parse ConfigurationUpdateCommand: %v", err)
	}

	if cuc.GUTI == nil {
		t.Fatal("expected GUTI to be present when includeGUTI is true")
	}

	if cuc.FullNameForNetwork == nil {
		t.Fatal("expected FullNameForNetwork to be present")
	}

	if cuc.ShortNameForNetwork == nil {
		t.Fatal("expected ShortNameForNetwork to be present")
	}
}

func TestBuildConfigurationUpdateCommand_WithGUTI_InvalidGUTI_Error(t *testing.T) {
	ue := buildTestUE(t)
	ue.SetGutiForTest(etsi.InvalidGUTI5G)

	_, err := amf.BuildConfigurationUpdateCommand(amf.New(nil, nil, nil), ue, etsi.InvalidGUTI5G, "ELLACORE5G", "ELLACORE", true)
	if err == nil {
		t.Fatal("expected error when includeGUTI is true but GUTI is invalid")
	}
}

func TestBuildRegistrationAccept_MultipleAllowedNSSAI(t *testing.T) {
	ue := buildTestUE(t)
	ue.AllowedNssai = []models.Snssai{
		{Sst: 1, Sd: "010203"},
		{Sst: 2, Sd: "aabbcc"},
	}

	amfInstance := amf.New(nil, nil, nil)

	raw, err := amf.BuildRegistrationAccept(amfInstance, ue, etsi.InvalidGUTI5G, nil, nil, nil, nil, models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatalf("BuildRegistrationAccept failed: %v", err)
	}

	ra, err := fgs.ParseRegistrationAccept(decryptNAS(t, ue, raw))
	if err != nil {
		t.Fatalf("parse RegistrationAccept: %v", err)
	}

	want := fgs.NSSAI{{SST: 1, SD: &[3]byte{1, 2, 3}}, {SST: 2, SD: &[3]byte{0xaa, 0xbb, 0xcc}}}
	if !reflect.DeepEqual(ra.AllowedNSSAI, want) {
		t.Fatalf("AllowedNSSAI = %+v, want %+v", ra.AllowedNSSAI, want)
	}
}

func TestBuildRegistrationAccept_SingleAllowedNSSAI(t *testing.T) {
	ue := buildTestUE(t)
	ue.AllowedNssai = []models.Snssai{
		{Sst: 1, Sd: "010203"},
	}

	amfInstance := amf.New(nil, nil, nil)

	raw, err := amf.BuildRegistrationAccept(amfInstance, ue, etsi.InvalidGUTI5G, nil, nil, nil, nil, models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatalf("BuildRegistrationAccept failed: %v", err)
	}

	ra, err := fgs.ParseRegistrationAccept(decryptNAS(t, ue, raw))
	if err != nil {
		t.Fatalf("parse RegistrationAccept: %v", err)
	}

	want := fgs.NSSAI{{SST: 1, SD: &[3]byte{1, 2, 3}}}
	if !reflect.DeepEqual(ra.AllowedNSSAI, want) {
		t.Fatalf("AllowedNSSAI = %+v, want %+v", ra.AllowedNSSAI, want)
	}
}

func TestBuildRegistrationAccept_EmptyAllowedNSSAI(t *testing.T) {
	ue := buildTestUE(t)
	ue.AllowedNssai = []models.Snssai{}

	amfInstance := amf.New(nil, nil, nil)

	raw, err := amf.BuildRegistrationAccept(amfInstance, ue, etsi.InvalidGUTI5G, nil, nil, nil, nil, models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatalf("BuildRegistrationAccept failed: %v", err)
	}

	ra, err := fgs.ParseRegistrationAccept(decryptNAS(t, ue, raw))
	if err != nil {
		t.Fatalf("parse RegistrationAccept: %v", err)
	}

	if ra.AllowedNSSAI != nil {
		t.Fatal("expected AllowedNSSAI to be absent when list is empty")
	}
}

func TestBuildRegistrationAccept_IWKN26FollowsS1ModeSupport(t *testing.T) {
	for _, tc := range []struct {
		name       string
		capability *fgs.GMMCapability
		want       bool
	}{
		{"UE supports S1 mode", &fgs.GMMCapability{S1Mode: true}, true},
		{"UE does not support S1 mode", &fgs.GMMCapability{}, false},
		{"UE sent no 5GMM capability", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ue := buildTestUE(t)
			ue.SetGMMCapability(tc.capability)

			raw, err := amf.BuildRegistrationAccept(amf.New(nil, nil, nil), ue, etsi.InvalidGUTI5G, nil, nil, nil, nil, models.PlmnID{Mcc: "001", Mnc: "01"})
			if err != nil {
				t.Fatalf("BuildRegistrationAccept failed: %v", err)
			}

			ra, err := fgs.ParseRegistrationAccept(decryptNAS(t, ue, raw))
			if err != nil {
				t.Fatalf("parse RegistrationAccept: %v", err)
			}

			if ra.NetworkFeatureSupport == nil {
				t.Fatal("Registration Accept carries no 5GS network feature support")
			}

			if ra.NetworkFeatureSupport.IWKN26 != tc.want {
				t.Errorf("IWK N26 = %v, want %v", ra.NetworkFeatureSupport.IWKN26, tc.want)
			}
		})
	}
}
