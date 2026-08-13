// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap/zapcore"
)

// TS 33.501 §8.6.2, Annex A.15.1: the UE and the AMF derive the same mapped 5G
// context from K_ASME and the uplink EPS NAS COUNT of the enclosed TRACKING AREA
// UPDATE REQUEST — not the NH the handover variant uses.
func TestIdleMappedSecurityContextFromEPSAgreesWithTheCore(t *testing.T) {
	logger.Init(zapcore.ErrorLevel)

	var in interworking.EPSSecurityContext

	for i := range in.KASME {
		in.KASME[i] = byte(i * 3)
		in.NH[i] = byte(i*5 + 1)
	}

	in.EKSI = nas.KeySetIdentifier{Value: 3}
	in.ULNASCount = nas.MakeCount(0, 9)
	in.Algorithms = interworking.EPSNASAlgorithms{Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES}
	in.UESecurityCapability = eps.UESecurityCapability{EEA: 0xf0, EIA: 0x70}
	in.UE5GSecurityCapability = &fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0}

	mapped, err := interworking.MapTo5GSOnIdleMobility(in,
		[]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES})
	if err != nil {
		t.Fatalf("MapTo5GSOnIdleMobility: %v", err)
	}

	u := &UE{UeSecurity: &UESecurity{}}

	if err := u.InstallMappedSecurityContextForIdleMobility(MappedFromEPSIdle{
		KASME:          in.KASME,
		UplinkNASCount: in.ULNASCount,
		EKSI:           in.EKSI.Value,
		Ciphering:      uint8(mapped.Ciphering),
		Integrity:      uint8(mapped.Integrity),
	}); err != nil {
		t.Fatalf("InstallMappedSecurityContextForIdleMobility: %v", err)
	}

	if !bytes.Equal(u.UeSecurity.Kamf, mapped.KAMF[:]) {
		t.Fatalf("K'AMF = %x, want the core's %x", u.UeSecurity.Kamf, mapped.KAMF)
	}

	if u.UeSecurity.KnasInt != mapped.KNASInt || u.UeSecurity.KnasEnc != mapped.KNASEnc {
		t.Fatal("the mapped NAS keys differ from the core's, so the security mode command would not verify")
	}

	if u.UeSecurity.NgKsi.Tsc != models.ScTypeMapped || u.UeSecurity.NgKsi.Ksi != int32(in.EKSI.Value) {
		t.Fatalf("ngKSI = %+v, want the eKSI %d of a mapped context", u.UeSecurity.NgKsi, in.EKSI.Value)
	}

	if u.UeSecurity.ULCount != 0 || u.UeSecurity.DLCount != 0 {
		t.Fatalf("NAS COUNTs = %d/%d, want both 0 (TS 33.501 §8.6.2)", u.UeSecurity.ULCount, u.UeSecurity.DLCount)
	}

	handover, err := interworking.MapTo5GSOnHandover(in,
		[]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES})
	if err != nil {
		t.Fatalf("MapTo5GSOnHandover: %v", err)
	}

	if bytes.Equal(u.UeSecurity.Kamf, handover.Context.KAMF[:]) {
		t.Fatal("the idle-mode derivation produced the handover key, so a real UE would not follow")
	}
}

// TS 24.501 §5.5.1.3.2 c: the registration of an inter-system change in idle
// mode carries the TAU REQUEST the UE would have sent in S1 mode, plus the
// native 5G-GUTI it still holds (§8.2.6.12 a).
func TestRegistrationRequestCarriesTheEPSNASContainerAndAdditionalGUTI(t *testing.T) {
	logger.Init(zapcore.ErrorLevel)

	tau := []byte{0x17, 0xde, 0xad, 0xbe, 0xef, 0x00, 0x07, 0x48}

	native := fgs.GUTIIdentity(fgs.GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 1, AMFSetID: 1, AMFPointer: 0,
		TMSI: [4]byte{1, 2, 3, 4},
	})

	mapped := fgs.GUTIIdentity(fgs.GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 1, AMFSetID: 1, AMFPointer: 0,
		TMSI: [4]byte{9, 9, 9, 9},
	})

	raw, err := BuildRegistrationRequest(&RegistrationRequestOpts{
		RegistrationType:       uint8(fgs.RegistrationTypeMobilityUpdating),
		IncludeCapability:      true,
		UESecurity:             &UESecurity{Guti: &mapped, NgKsi: models.NgKsi{Ksi: 4, Tsc: models.ScTypeMapped}},
		UEStatus:               &fgs.UEStatus{S1ModeReg: true},
		EPSNASMessageContainer: tau,
		AdditionalGUTI:         &native,
	})
	if err != nil {
		t.Fatalf("BuildRegistrationRequest: %v", err)
	}

	req, err := fgs.ParseRegistrationRequest(raw)
	if err != nil {
		t.Fatalf("ParseRegistrationRequest: %v", err)
	}

	if !bytes.Equal(req.EPSNASMessageContainer, tau) {
		t.Errorf("EPS NAS message container = %x, want %x", req.EPSNASMessageContainer, tau)
	}

	if req.AdditionalGUTI == nil || req.AdditionalGUTI.GUTI == nil ||
		req.AdditionalGUTI.GUTI.TMSI != [4]byte{1, 2, 3, 4} {
		t.Errorf("Additional GUTI = %+v, want the native 5G-GUTI", req.AdditionalGUTI)
	}

	if req.UEStatus == nil || !req.UEStatus.S1ModeReg {
		t.Errorf("UE status = %v, want EMM-REGISTERED", req.UEStatus)
	}
}
