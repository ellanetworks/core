// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

const arrivedEKSI uint8 = 3

func arrivedFromEPSUe(t *testing.T) (*amf.UeContext, *fakeNGAPSender) {
	t.Helper()

	ue, sender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	mapped := interworking.Mapped5GSecurityContext{
		NgKSI:                 nas.KeySetIdentifier{Value: arrivedEKSI, Mapped: true},
		DLNASCount:            1,
		Ciphering:             nas.CipheringAES,
		Integrity:             nas.IntegrityAES,
		UESecurityCapability:  fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0},
		EPSAlgorithms:         interworking.EPSNASAlgorithms{Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES},
		EPSSecurityCapability: eps.UESecurityCapability{EEA: 0xe0, EIA: 0xe0},
		NCC:                   1,
	}

	for i := range mapped.KAMF {
		mapped.KAMF[i] = byte(i + 1)
	}

	for i := range mapped.KNASEnc {
		mapped.KNASEnc[i] = byte(i + 1)
		mapped.KNASInt[i] = byte(i + 0x21)
	}

	if err := ue.InstallMappedSecurityContextFromEPS(mapped, amf.MintAuthProofForInterworking()); err != nil {
		t.Fatalf("InstallMappedSecurityContextFromEPS: %v", err)
	}

	return ue, sender
}

func mappedGUTIIdentity() fgs.MobileIdentity {
	return fgs.GUTIIdentity(etsi.MapGUTIEPSTo5G(eps.GUTI{
		PLMN:       nas.PLMN{MCC: "001", MNC: "01"},
		MMEGroupID: 0x0102,
		MMECode:    0x03,
		TMSI:       [4]byte{0xde, 0xad, 0xbe, 0xef},
	}))
}

func arrivedRegistrationAMF() *amf.AMF {
	return amf.New(&fakeDBInstance{
		Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`},
	}, nil, nil)
}

func TestMobilityRegistrationAfterAnArrivalFromEPSKeepsTheMappedContext(t *testing.T) {
	amfInstance := arrivedRegistrationAMF()
	ue, sender := arrivedFromEPSUe(t)

	before := ue.NgKsi()
	if before.Tsc != models.ScTypeMapped || before.Ksi != int32(arrivedEKSI) {
		t.Fatalf("the arrived UE holds ngKSI %+v, want the mapped eKSI %d", before, arrivedEKSI)
	}

	wire, err := buildRegReqBytes(uint8(fgs.RegistrationTypeMobilityUpdating), mappedGUTIIdentity(),
		&fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0}, 0, nil, 0, arrivedEKSI)
	if err != nil {
		t.Fatalf("build the registration request: %v", err)
	}

	req := mustParseRegistrationRequest(t, wire)
	req.NgKSI.Mapped = true

	if err := handleRegistrationRequestMessage(context.Background(), amfInstance, ue, req, wire, true, false); err != nil {
		t.Fatalf("the registration update after an arrival from EPS was refused: %v", err)
	}

	if len(sender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("the registration update drew a NAS reply: %+v", sender.SentDownlinkNASTransport[0])
	}

	if got := ue.NgKsi(); got != before {
		t.Errorf("ngKSI = %+v, want the mapped %+v it arrived with: the next security mode command or handover to EPS would name a context the UE does not hold",
			got, before)
	}
}

// TS 24.501 §5.4.2.2
func TestRegistrationRequestLeavesTheNgKSIAlone(t *testing.T) {
	amfInstance := arrivedRegistrationAMF()

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	setTestUESecurityCapability(ue)

	before := ue.NgKsi()

	wire, err := buildRegReqBytes(uint8(fgs.RegistrationTypeMobilityUpdating), testMobileIdentity(),
		&fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0}, 0, nil, 0, 2)
	if err != nil {
		t.Fatalf("build the registration request: %v", err)
	}

	if err := handleRegistrationRequestMessage(context.Background(), amfInstance, ue, mustParseRegistrationRequest(t, wire), wire, true, false); err != nil {
		t.Fatalf("handleRegistrationRequestMessage: %v", err)
	}

	if got := ue.NgKsi(); got != before {
		t.Errorf("ngKSI = %+v, want the %+v held before the request was taken in", got, before)
	}
}

// TS 24.501 §5.4.1.3.2
func TestAuthenticationTakesAFreshNgKSIAfterTheOneTheUECited(t *testing.T) {
	amfInstance := arrivedRegistrationAMF()

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	setTestUESecurityCapability(ue)
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))

	wire, err := buildRegReqBytes(uint8(fgs.RegistrationTypeMobilityUpdating), testMobileIdentity(),
		&fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0}, 0, nil, 0, 2)
	if err != nil {
		t.Fatalf("build the registration request: %v", err)
	}

	if err := handleRegistrationRequestMessage(context.Background(), amfInstance, ue, mustParseRegistrationRequest(t, wire), wire, true, false); err != nil {
		t.Fatalf("handleRegistrationRequestMessage: %v", err)
	}

	ue.Tai.PlmnID = nil

	if _, err := authenticationProcedure(context.Background(), amfInstance, ue); err == nil {
		t.Fatal("authentication built a request for a UE with no serving PLMN")
	}

	if got := ue.NgKsi(); got.Tsc != models.ScTypeNative || got.Ksi != 3 {
		t.Errorf("ngKSI = %+v, want the native identifier after the 2 the UE cited", got)
	}
}
