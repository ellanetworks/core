// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func mappableUE(t *testing.T) *amf.UeContext {
	t.Helper()

	ue := buildTestUE(t)
	ue.SetKamfForTest("6c38fea1e0a2ff9f8ba6a1e4f4de8b8e1b3b7f2e9d5c0a4738261f5e0d9c8b7a")
	ue.SetNgKsiForTest(models.NgKsi{Ksi: 4, Tsc: models.ScTypeNative})
	ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, s1NetworkCapability(0xe0, 0xe0, 0x00, 0x00))

	if _, ok := ue.SelectEPSNASAlgorithms([]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES}); !ok {
		t.Fatal("selecting the EPS NAS algorithms failed")
	}

	ue.MarkEPSNASAlgorithmsDelivered()

	return ue
}

func TestMapSecurityContextToEPS(t *testing.T) {
	ue := mappableUE(t)

	got, err := ue.MapSecurityContextToEPS()
	if err != nil {
		t.Fatalf("MapSecurityContextToEPS: %v", err)
	}

	if got.Context.EKSI != (nas.KeySetIdentifier{Value: 4, Mapped: true}) {
		t.Fatalf("eKSI = %+v, want value 4 of a mapped context", got.Context.EKSI)
	}

	if got.Context.Algorithms.Ciphering != nas.CipheringAES || got.Context.Algorithms.Integrity != nas.IntegrityAES {
		t.Fatalf("EPS NAS algorithms = %+v, want the pair the UE was given", got.Context.Algorithms)
	}

	if want := (nas.AlgorithmSet(0xe0)); got.Context.UESecurityCapability.EEA != want {
		t.Fatalf("EPS security capability EEA = %08b, want %08b", got.Context.UESecurityCapability.EEA, want)
	}

	if got.Context.NCC != 2 {
		t.Fatalf("NCC = %d, want 2", got.Context.NCC)
	}

	var zero [32]byte
	if bytes.Equal(got.Context.KASME[:], zero[:]) {
		t.Fatal("K'ASME was not derived")
	}
}

// TS 33.501 §8.3.2 step 2
func TestMapSecurityContextToEPSConsumesTheDownlinkCount(t *testing.T) {
	ue := mappableUE(t)

	first, err := ue.MapSecurityContextToEPS()
	if err != nil {
		t.Fatalf("MapSecurityContextToEPS: %v", err)
	}

	second, err := ue.MapSecurityContextToEPS()
	if err != nil {
		t.Fatalf("MapSecurityContextToEPS: %v", err)
	}

	if first.Container.SequenceNumber == second.Container.SequenceNumber {
		t.Fatal("a second mapping reused the downlink NAS COUNT of the first")
	}

	if bytes.Equal(first.Context.KASME[:], second.Context.KASME[:]) {
		t.Fatal("a second mapping produced the same K'ASME")
	}

	next, err := ue.NextDownlinkCountForTest()
	if err != nil {
		t.Fatalf("downlink counter: %v", err)
	}

	if next.SQN() == first.Container.SequenceNumber || next.SQN() == second.Container.SequenceNumber {
		t.Fatal("the downlink counter did not advance past the mapped COUNTs")
	}
}

func TestMapSecurityContextToEPSNeedsTheEPSNASAlgorithms(t *testing.T) {
	ue := buildTestUE(t)
	ue.SetKamfForTest("6c38fea1e0a2ff9f8ba6a1e4f4de8b8e1b3b7f2e9d5c0a4738261f5e0d9c8b7a")
	ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, s1NetworkCapability(0xe0, 0xe0, 0x00, 0x00))

	_, err := ue.MapSecurityContextToEPS()
	if !errors.Is(err, amf.ErrNoEPSNASAlgorithms) {
		t.Fatalf("error = %v, want ErrNoEPSNASAlgorithms", err)
	}
}

func TestMapSecurityContextToEPSNeedsTheEPSSecurityCapability(t *testing.T) {
	ue := mappableUE(t)
	ue.ForgetS1CapabilityForTest()

	_, err := ue.MapSecurityContextToEPS()
	if !errors.Is(err, amf.ErrNoEPSSecurityCapability) {
		t.Fatalf("error = %v, want ErrNoEPSSecurityCapability", err)
	}
}

func TestMapSecurityContextToEPSNeedsASecurityContext(t *testing.T) {
	ue := amf.NewUeContext()
	if _, err := ue.MapSecurityContextToEPS(); err == nil {
		t.Fatal("a UE with no 5G NAS security context must not be mapped")
	}
}

func TestInstallMappedSecurityContextFromEPS(t *testing.T) {
	ue := amf.NewUeContext()

	mapped := interworking.Mapped5GSecurityContext{
		NgKSI:                nas.KeySetIdentifier{Value: 6, Mapped: true},
		DLNASCount:           1,
		Ciphering:            nas.CipheringAES,
		Integrity:            nas.IntegrityAES,
		UESecurityCapability: interworking.DefaultUE5GSecurityCapability,
		EPSAlgorithms:        interworking.EPSNASAlgorithms{Ciphering: nas.CipheringSNOW3G, Integrity: nas.IntegritySNOW3G},
		NCC:                  1,
	}
	for i := range mapped.KAMF {
		mapped.KAMF[i] = byte(i)
		mapped.KNASEnc[i%16] = byte(i + 1)
		mapped.KNASInt[i%16] = byte(i + 2)
		mapped.NH[i] = byte(i + 3)
	}

	if err := ue.InstallMappedSecurityContextFromEPS(mapped, amf.MintAuthProofForInterworking()); err != nil {
		t.Fatalf("InstallMappedSecurityContextFromEPS: %v", err)
	}

	if !ue.SecurityContextIsValid() {
		t.Fatal("the mapped context must be current")
	}

	if got := ue.NgKsi(); got.Ksi != 6 || got.Tsc != models.ScTypeMapped {
		t.Fatalf("ngKSI = %+v, want value 6 of a mapped context", got)
	}

	if ue.NEA() != nas.CipheringAES || ue.NIA() != nas.IntegrityAES {
		t.Fatalf("algorithms = (%s, %s), want the mapped pair", ue.NEA(), ue.NIA())
	}

	eps, ok := ue.EPSNASAlgorithmsInUse()
	if !ok || eps != mapped.EPSAlgorithms {
		t.Fatalf("EPS NAS algorithms = (%v, %+v), want the MME's %+v", ok, eps, mapped.EPSAlgorithms)
	}

	next, err := ue.NextDownlinkCountForTest()
	if err != nil {
		t.Fatalf("downlink counter: %v", err)
	}

	if next != 1 {
		t.Fatalf("next downlink NAS COUNT = %d, want 1", next)
	}

	if ue.ULCount() != 0 {
		t.Fatalf("expected uplink NAS COUNT = %d, want 0", ue.ULCount())
	}

	if ue.NCCForTest() != 1 {
		t.Fatalf("NCC = %d, want the pair the AMF stores", ue.NCCForTest())
	}

	// The AMF names no K_gNB after a handover from EPS: the target gNB derived its
	// own from the pair it was handed (TS 33.501 §8.4.2 step 5).
	if ue.KgnbForTest() != nil {
		t.Fatal("the AMF must hold no K_gNB after installing a context mapped from EPS")
	}
}

func TestMappedContextRoundTrip(t *testing.T) {
	ue := mappableUE(t)

	toEPS, err := ue.MapSecurityContextToEPS()
	if err != nil {
		t.Fatalf("MapSecurityContextToEPS: %v", err)
	}

	back, err := interworking.MapTo5GSOnHandover(interworking.EPSSecurityContext{
		KASME:                toEPS.Context.KASME,
		EKSI:                 toEPS.Context.EKSI,
		ULNASCount:           toEPS.Context.ULNASCount,
		DLNASCount:           toEPS.Context.DLNASCount,
		Algorithms:           toEPS.Context.Algorithms,
		UESecurityCapability: toEPS.Context.UESecurityCapability,
		NH:                   toEPS.Context.NH,
		NCC:                  toEPS.Context.NCC,
	}, []nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES})
	if err != nil {
		t.Fatalf("MapTo5GSOnHandover: %v", err)
	}

	if back.Context.NgKSI.Value != toEPS.Context.EKSI.Value {
		t.Fatalf("ngKSI value = %d, want the eKSI's %d", back.Context.NgKSI.Value, toEPS.Context.EKSI.Value)
	}

	if !back.Context.NgKSI.Mapped {
		t.Fatal("a context mapped from EPS must be marked mapped")
	}

	if back.Container.NCC != toEPS.Context.NCC {
		t.Fatalf("container NCC = %d, want the EPS NCC %d", back.Container.NCC, toEPS.Context.NCC)
	}

	if err := ue.InstallMappedSecurityContextFromEPS(back.Context, amf.MintAuthProofForInterworking()); err != nil {
		t.Fatalf("InstallMappedSecurityContextFromEPS: %v", err)
	}

	if !ue.SecurityContextIsValid() {
		t.Fatal("the round-tripped context must be current")
	}
}
