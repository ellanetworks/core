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

func buildServedTestUE(t *testing.T, amfInstance *amf.AMF, imsi string) *amf.UeContext {
	t.Helper()

	ue := buildTestUE(t)

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		t.Fatalf("invalid IMSI %q: %v", imsi, err)
	}

	ue.SetSupiForTest(supi)
	ue.ForceStateForTest(amf.RegistrationInitiated)

	if err := amfInstance.CommitUEIdentity(t.Context(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	return ue
}

func TestBuildConfigurationUpdateCommand_WithoutGUTI(t *testing.T) {
	raw, err := amf.BuildConfigurationUpdateCommand(etsi.InvalidGUTI5G, "ELLACORE5G", "ELLACORE", false)
	if err != nil {
		t.Fatalf("BuildConfigurationUpdateCommand failed: %v", err)
	}

	cuc, err := fgs.ParseConfigurationUpdateCommand(raw)
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

	raw, err := amf.BuildConfigurationUpdateCommand(guti, "ELLACORE5G", "ELLACORE", true)
	if err != nil {
		t.Fatalf("BuildConfigurationUpdateCommand failed: %v", err)
	}

	cuc, err := fgs.ParseConfigurationUpdateCommand(raw)
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
	_, err := amf.BuildConfigurationUpdateCommand(etsi.InvalidGUTI5G, "ELLACORE5G", "ELLACORE", true)
	if err == nil {
		t.Fatal("expected error when includeGUTI is true but GUTI is invalid")
	}
}

func TestBuildRegistrationAccept_MultipleAllowedNSSAI(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	ue := buildServedTestUE(t, amfInstance, "001019756139901")
	ue.AllowedNssai = []models.Snssai{
		{Sst: 1, Sd: "010203"},
		{Sst: 2, Sd: "aabbcc"},
	}

	raw, err := amf.BuildRegistrationAccept(amfInstance, ue, etsi.InvalidGUTI5G, nil, nil, nil, nil, models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatalf("BuildRegistrationAccept failed: %v", err)
	}

	ra, err := fgs.ParseRegistrationAccept(raw)
	if err != nil {
		t.Fatalf("parse RegistrationAccept: %v", err)
	}

	want := fgs.NSSAI{{SST: 1, SD: &[3]byte{1, 2, 3}}, {SST: 2, SD: &[3]byte{0xaa, 0xbb, 0xcc}}}
	if !reflect.DeepEqual(ra.AllowedNSSAI, want) {
		t.Fatalf("AllowedNSSAI = %+v, want %+v", ra.AllowedNSSAI, want)
	}
}

func TestBuildRegistrationAccept_SingleAllowedNSSAI(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	ue := buildServedTestUE(t, amfInstance, "001019756139902")
	ue.AllowedNssai = []models.Snssai{
		{Sst: 1, Sd: "010203"},
	}

	raw, err := amf.BuildRegistrationAccept(amfInstance, ue, etsi.InvalidGUTI5G, nil, nil, nil, nil, models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatalf("BuildRegistrationAccept failed: %v", err)
	}

	ra, err := fgs.ParseRegistrationAccept(raw)
	if err != nil {
		t.Fatalf("parse RegistrationAccept: %v", err)
	}

	want := fgs.NSSAI{{SST: 1, SD: &[3]byte{1, 2, 3}}}
	if !reflect.DeepEqual(ra.AllowedNSSAI, want) {
		t.Fatalf("AllowedNSSAI = %+v, want %+v", ra.AllowedNSSAI, want)
	}
}

func TestBuildRegistrationAccept_EmptyAllowedNSSAI(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	ue := buildServedTestUE(t, amfInstance, "001019756139903")
	ue.AllowedNssai = []models.Snssai{}

	raw, err := amf.BuildRegistrationAccept(amfInstance, ue, etsi.InvalidGUTI5G, nil, nil, nil, nil, models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatalf("BuildRegistrationAccept failed: %v", err)
	}

	ra, err := fgs.ParseRegistrationAccept(raw)
	if err != nil {
		t.Fatalf("parse RegistrationAccept: %v", err)
	}

	if ra.AllowedNSSAI != nil {
		t.Fatal("expected AllowedNSSAI to be absent when list is empty")
	}
}

// TS 24.501 §8.2.7.31, TS 23.502 §4.11.1.3.3 steps 17-18
func TestRegistrationAcceptDropsTheEPSBearerStatusOnceTheRegistrationIsDone(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	amfInstance.EPS = &fakeEPSPeer{}

	ue := buildServedTestUE(t, amfInstance, "001019756139904")
	attachTestConn(t, ue)

	if err := ue.CreateSmContext(3, "ref-3", &models.Snssai{Sst: 1, Sd: "010203"}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	ue.SetEPSBearerIdentity(3, 6)

	conn := ue.Conn()
	if conn == nil {
		t.Fatal("no NAS connection")
	}

	conn.ArrivedFromEPS = true

	if status := epsBearerStatusOf(t, amfInstance, ue); status == nil || !status.Active[6] {
		t.Fatalf("EPS bearer context status = %+v, want EBI 6 active while the arrival is being registered", status)
	}

	ue.ClearRegistrationRequestData()

	if status := epsBearerStatusOf(t, amfInstance, ue); status != nil {
		t.Errorf("EPS bearer context status = %+v, want the IE omitted: the arrival flag outlived its registration", status)
	}
}

func epsBearerStatusOf(t *testing.T, amfInstance *amf.AMF, ue *amf.UeContext) *nas.EPSBearerContextStatus {
	t.Helper()

	raw, err := amf.BuildRegistrationAccept(amfInstance, ue, etsi.InvalidGUTI5G, nil, nil, nil, nil,
		models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatalf("BuildRegistrationAccept: %v", err)
	}

	ra, err := fgs.ParseRegistrationAccept(raw)
	if err != nil {
		t.Fatalf("parse RegistrationAccept: %v", err)
	}

	return ra.EPSBearerContextStatus
}

// TS 24.501 §5.5.1.3.4
func TestBuildRegistrationAcceptRefusesAContextTheAMFDoesNotServe(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	served := buildServedTestUE(t, amfInstance, "001019756139905")

	for _, tc := range []struct {
		name string
		ue   *amf.UeContext
	}{
		{"never committed", buildTestUE(t)},
		{"superseded by a newer context for the same subscriber", served},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ue == served {
				// A second context for the same SUPI displaces the first in the index.
				buildServedTestUE(t, amfInstance, "001019756139905")
			}

			_, err := amf.BuildRegistrationAccept(amfInstance, tc.ue, etsi.InvalidGUTI5G, nil, nil, nil, nil,
				models.PlmnID{Mcc: "001", Mnc: "01"})
			if err == nil {
				t.Fatal("built a registration accept for a UE the AMF cannot resolve by SUPI")
			}
		})
	}
}
