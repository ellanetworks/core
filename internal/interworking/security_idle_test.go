// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 33.501 Annex A.14.1
func TestMapToEPSOnIdleMobilityDerivesKASMEFromTheUplinkCount(t *testing.T) {
	kamf := mustHex(t, testKAMF)
	ul := nas.MakeCount(0x0102, 0x03)
	dl := nas.MakeCount(0x0044, 0x05)

	got, err := interworking.MapToEPSOnIdleMobility(interworking.FiveGToEPSInput{
		KAMF:       kamf,
		NgKSI:      nas.KeySetIdentifier{Value: 5},
		ULNASCount: ul,
		DLNASCount: dl,
		Algorithms: interworking.EPSNASAlgorithms{Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES},
	})
	if err != nil {
		t.Fatalf("MapToEPSOnIdleMobility: %v", err)
	}

	want := kdf(t, kamf, 0x73, u32(ul.Value()))
	if !bytes.Equal(got.KASME[:], want) {
		t.Fatalf("K'ASME = %x, want %x", got.KASME, want)
	}

	if handover := kdf(t, kamf, 0x74, u32(dl.Value())); bytes.Equal(got.KASME[:], handover) {
		t.Fatal("K'ASME is the handover derivation, so the UE's idle-mode key would not match")
	}

	if got.EKSI != (nas.KeySetIdentifier{Value: 5, Mapped: true}) {
		t.Fatalf("eKSI = %+v, want value 5 of a mapped context", got.EKSI)
	}
}

// TS 33.501 §8.6.1
func TestMapToEPSOnIdleMobilityCarriesBothCounts(t *testing.T) {
	ul := nas.MakeCount(7, 8)
	dl := nas.MakeCount(9, 10)

	got, err := interworking.MapToEPSOnIdleMobility(interworking.FiveGToEPSInput{
		KAMF:       mustHex(t, testKAMF),
		ULNASCount: ul,
		DLNASCount: dl,
	})
	if err != nil {
		t.Fatalf("MapToEPSOnIdleMobility: %v", err)
	}

	if got.ULNASCount != ul {
		t.Errorf("EPS uplink NAS COUNT = %d, want the 5G context's %d", got.ULNASCount, ul)
	}

	if got.DLNASCount != dl {
		t.Errorf("EPS downlink NAS COUNT = %d, want the 5G context's %d unconsumed", got.DLNASCount, dl)
	}
}

// TS 33.401 §7.2.6.2
func TestMapToEPSOnIdleMobilityCarriesNoNextHop(t *testing.T) {
	got, err := interworking.MapToEPSOnIdleMobility(interworking.FiveGToEPSInput{
		KAMF:       mustHex(t, testKAMF),
		ULNASCount: nas.MakeCount(1, 2),
	})
	if err != nil {
		t.Fatalf("MapToEPSOnIdleMobility: %v", err)
	}

	if got.NH != ([32]byte{}) || got.NCC != 0 {
		t.Errorf("NH/NCC = %x/%d, want them unset", got.NH, got.NCC)
	}
}

// TS 33.501 Annex A.15.1
func TestMapTo5GSOnIdleMobilityDerivesKAMFFromTheUplinkCount(t *testing.T) {
	kasme := mustHex(t, testKASME)
	ul := nas.MakeCount(0x0011, 0x22)

	var key [32]byte

	copy(key[:], kasme)

	in := interworking.EPSSecurityContext{
		KASME:                key,
		EKSI:                 nas.KeySetIdentifier{Value: 3},
		ULNASCount:           ul,
		DLNASCount:           nas.MakeCount(4, 5),
		Algorithms:           interworking.EPSNASAlgorithms{Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES},
		UESecurityCapability: eps.UESecurityCapability{EEA: 0xf0, EIA: 0x70},
		NH:                   [32]byte{1, 2, 3},
		NCC:                  2,
	}

	got, err := interworking.MapTo5GSOnIdleMobility(in,
		[]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES})
	if err != nil {
		t.Fatalf("MapTo5GSOnIdleMobility: %v", err)
	}

	want := kdf(t, kasme, 0x75, u32(ul.Value()))
	if !bytes.Equal(got.KAMF[:], want) {
		t.Fatalf("K'AMF = %x, want %x", got.KAMF, want)
	}

	if handover := kdf(t, kasme, 0x76, in.NH[:]); bytes.Equal(got.KAMF[:], handover) {
		t.Fatal("K'AMF is the handover derivation, so the UE's idle-mode key would not match")
	}

	if got.NgKSI != (nas.KeySetIdentifier{Value: 3, Mapped: true}) {
		t.Fatalf("ngKSI = %+v, want value 3 of a mapped context", got.NgKSI)
	}

	if got.EPSAlgorithms != in.Algorithms {
		t.Errorf("EPS algorithms = %+v, want the ones already in use %+v", got.EPSAlgorithms, in.Algorithms)
	}
}

// TS 33.501 §8.6.2
func TestMapTo5GSOnIdleMobilityStartsBothCountsAtZero(t *testing.T) {
	var key [32]byte

	copy(key[:], mustHex(t, testKASME))

	got, err := interworking.MapTo5GSOnIdleMobility(interworking.EPSSecurityContext{
		KASME:                key,
		ULNASCount:           nas.MakeCount(3, 4),
		DLNASCount:           nas.MakeCount(5, 6),
		UESecurityCapability: eps.UESecurityCapability{EEA: 0xf0, EIA: 0x70},
	}, []nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES})
	if err != nil {
		t.Fatalf("MapTo5GSOnIdleMobility: %v", err)
	}

	if got.DLNASCount != 0 {
		t.Errorf("5G downlink NAS COUNT = %d, want 0", got.DLNASCount)
	}

	if got.TemporaryKgNB != ([32]byte{}) || got.NH != ([32]byte{}) || got.NCC != 0 {
		t.Errorf("AN key material = %x/%x/%d, want it unset", got.TemporaryKgNB, got.NH, got.NCC)
	}
}

// TS 33.501 §8.6.2
func TestMapTo5GSOnIdleMobilitySelectsAndKeysThe5GAlgorithms(t *testing.T) {
	kasme := mustHex(t, testKASME)

	var key [32]byte

	copy(key[:], kasme)

	ul := nas.MakeCount(0, 1)

	got, err := interworking.MapTo5GSOnIdleMobility(interworking.EPSSecurityContext{
		KASME:                key,
		ULNASCount:           ul,
		UESecurityCapability: eps.UESecurityCapability{EEA: 0xf0, EIA: 0x70},
	}, []nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES})
	if err != nil {
		t.Fatalf("MapTo5GSOnIdleMobility: %v", err)
	}

	if got.Ciphering != nas.CipheringAES || got.Integrity != nas.IntegrityAES {
		t.Fatalf("algorithms = %v/%v, want AES/AES", got.Ciphering, got.Integrity)
	}

	kamf := kdf(t, kasme, 0x75, u32(ul.Value()))
	wantEnc := kdf(t, kamf, 0x69, []byte{0x01}, []byte{uint8(nas.CipheringAES)})
	wantInt := kdf(t, kamf, 0x69, []byte{0x02}, []byte{uint8(nas.IntegrityAES)})

	if !bytes.Equal(got.KNASEnc[:], wantEnc[len(wantEnc)-16:]) {
		t.Errorf("K_NASenc = %x, want %x", got.KNASEnc, wantEnc[len(wantEnc)-16:])
	}

	if !bytes.Equal(got.KNASInt[:], wantInt[len(wantInt)-16:]) {
		t.Errorf("K_NASint = %x, want %x", got.KNASInt, wantInt[len(wantInt)-16:])
	}
}

func TestMapTo5GSOnIdleMobilityRefusesWithNoCommonAlgorithm(t *testing.T) {
	var key [32]byte

	copy(key[:], mustHex(t, testKASME))

	_, err := interworking.MapTo5GSOnIdleMobility(interworking.EPSSecurityContext{
		KASME:                key,
		UESecurityCapability: eps.UESecurityCapability{EEA: 0xf0, EIA: 0x70},
	}, nil, nil)
	if err == nil {
		t.Fatal("a context mapped under an empty operator policy, want a refusal")
	}
}
