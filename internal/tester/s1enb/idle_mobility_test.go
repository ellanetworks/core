// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 33.501 §8.6.1, Annex A.14.1
func TestIdleMappedSecurityContextAgreesWithTheCore(t *testing.T) {
	const (
		ulCount = 7
		dlCount = 11
	)

	mapped, err := interworking.MapToEPSOnIdleMobility(interworking.FiveGToEPSInput{
		KAMF:       testKAMF(),
		NgKSI:      nas.KeySetIdentifier{Value: 4},
		ULNASCount: ulCount,
		DLNASCount: dlCount,
		Algorithms: interworking.EPSNASAlgorithms{
			Ciphering: nas.CipheringAES,
			Integrity: nas.IntegrityAES,
		},
		UESecurityCapability: eps.UESecurityCapability{EEA: 0xf0, EIA: 0x70},
	})
	if err != nil {
		t.Fatalf("MapToEPSOnIdleMobility: %v", err)
	}

	e := s1enb.NewUnboundUE()

	if err := e.InstallMappedSecurityContextForIdleMobility(s1enb.IdleMobilityFrom5GS{
		KAMF:             testKAMF(),
		UplinkNASCount:   ulCount,
		DownlinkNASCount: dlCount,
		EPSCiphering:     uint8(nas.CipheringAES),
		EPSIntegrity:     uint8(nas.IntegrityAES),
		EKSI:             4,
	}); err != nil {
		t.Fatalf("InstallMappedSecurityContextForIdleMobility: %v", err)
	}

	if !bytes.Equal(e.MappedKASME(), mapped.KASME[:]) {
		t.Fatalf("K'ASME = %x, want the core's %x", e.MappedKASME(), mapped.KASME)
	}

	handover, err := interworking.MapToEPSOnHandover(interworking.FiveGToEPSInput{
		KAMF:       testKAMF(),
		ULNASCount: ulCount,
		DLNASCount: dlCount,
	})
	if err != nil {
		t.Fatalf("MapToEPSOnHandover: %v", err)
	}

	if bytes.Equal(e.MappedKASME(), handover.Context.KASME[:]) {
		t.Fatal("the idle-mode derivation produced the handover key, so a real UE would not follow")
	}
}

// TS 33.501 §8.5.2 step 4
func TestIdleTrackingAreaUpdateIsMACdWithThe5GContext(t *testing.T) {
	security := s1enb.IdleMobilityFrom5GS{
		KAMF:             testKAMF(),
		KNASInt:          [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		NIA:              uint8(nas.IntegrityAES),
		UplinkNASCount:   nas.MakeCount(0, 3),
		DownlinkNASCount: nas.MakeCount(0, 1),
		EPSCiphering:     uint8(nas.CipheringAES),
		EPSIntegrity:     uint8(nas.IntegrityAES),
		EKSI:             4,
	}

	e := s1enb.NewUnboundUE()

	wire, err := e.BuildIdleTrackingAreaUpdate(s1enb.IdleTrackingAreaUpdateOpts{
		GUTI: eps.GUTI{
			PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 0x0100, MMECode: 0x40,
			TMSI: [4]byte{0x00, 0x00, 0xde, 0xad},
		},
		ActiveFlag: true,
		Security:   security,
	})
	if err != nil {
		t.Fatalf("BuildIdleTrackingAreaUpdate: %v", err)
	}

	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:    nas.IntegrityAES,
		IntegrityKey: security.KNASInt,
		Ciphering:    nas.CipheringNull,
	})
	if err != nil {
		t.Fatal(err)
	}

	plain, err := eps.VerifyWith5GContext(wire, security.UplinkNASCount, nas.DirectionUplink, sc)
	if err != nil {
		t.Fatalf("the core could not verify the update against the 5G context: %v", err)
	}

	req, err := eps.ParseTrackingAreaUpdateRequest(plain)
	if err != nil {
		t.Fatalf("parse TAU: %v", err)
	}

	if req.OldGUTIType == nil || *req.OldGUTIType != eps.GUTITypeNative {
		t.Errorf("Old GUTI type = %v, want native (TS 24.301 §5.5.3.2.2 case z)", req.OldGUTIType)
	}

	if req.UEStatus == nil || !req.UEStatus.N1ModeReg {
		t.Errorf("UE status = %v, want 5GMM-REGISTERED", req.UEStatus)
	}

	if !req.ActiveFlag {
		t.Error("the active flag was not set, so no S1-U would be established")
	}

	if _, _, err := eps.Unprotect(wire, security.UplinkNASCount, nas.DirectionUplink, sc); err == nil {
		t.Error("the update also verifies as an ordinary EPS message, so the bearer is not the 3GPP access one")
	}
}
