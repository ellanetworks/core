// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func s1NetworkCapability(eea, eia, uea, uia byte) []byte {
	return []byte{eea, eia, uea, uia}
}

func attachTestConn(t *testing.T, ue *amf.UeContext) {
	t.Helper()

	amfInstance := amf.New(nil, nil, nil)
	radio := &amf.Radio{Conn: &fakeNGAPSender{}}
	radio.BindAMFForTest(amfInstance)
	amfInstance.AttachUeConn(ue, amf.NewUeConnForTest(radio, 1, 1, zap.NewNop()))
}

func epsCapableUE(t *testing.T, s1 []byte) *amf.UeContext {
	t.Helper()

	ue := amf.NewUeContext()
	ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, s1)

	return ue
}

func TestSelectEPSNASAlgorithmsUsesTheS1Capability(t *testing.T) {
	ue := epsCapableUE(t, s1NetworkCapability(0b1010_0000, 0b0010_0000, 0x00, 0x00))

	ue.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0b0100_0000, IA: 0b0100_0000})

	selected, ok := ue.SelectEPSNASAlgorithms(
		[]nas.IntegrityAlgorithm{nas.IntegritySNOW3G, nas.IntegrityAES},
		[]nas.CipheringAlgorithm{nas.CipheringSNOW3G, nas.CipheringAES, nas.CipheringNull},
	)
	if !ok {
		t.Fatal("a UE advertising 128-EEA2 and 128-EIA2 must get an EPS algorithm pair")
	}

	if selected.Ciphering != nas.CipheringAES || selected.Integrity != nas.IntegrityAES {
		t.Fatalf("selected (%s, %s), want (128-EEA2, 128-EIA2)", selected.Ciphering, selected.Integrity)
	}
}

func TestNeedsEPSNASAlgorithms(t *testing.T) {
	noS1Mode := amf.NewUeContext()
	noS1Mode.SetUECapabilities(&fgs.GMMCapability{}, s1NetworkCapability(0xe0, 0xe0, 0, 0))

	if noS1Mode.NeedsEPSNASAlgorithms() {
		t.Error("a UE that does not support S1 mode is owed no EPS NAS algorithms")
	}

	noCapability := amf.NewUeContext()
	noCapability.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, nil)

	if !noCapability.NeedsEPSNASAlgorithms() {
		t.Error("an S1-capable UE holding no pair is owed one")
	}

	if _, ok := noCapability.SelectEPSNASAlgorithms([]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES}); ok {
		t.Error("no pair can be selected without the UE's EPS capability")
	}

	ue := epsCapableUE(t, s1NetworkCapability(0xe0, 0xe0, 0, 0))
	if !ue.NeedsEPSNASAlgorithms() {
		t.Fatal("an S1-capable UE that has disclosed its EPS algorithms is owed a selection")
	}

	if _, ok := ue.SelectEPSNASAlgorithms([]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES}); !ok {
		t.Fatal("selection failed")
	}

	if !ue.NeedsEPSNASAlgorithms() {
		t.Error("a selection the UE has not accepted must not settle the debt")
	}

	if _, ok := ue.EPSNASAlgorithmsInUse(); ok {
		t.Error("EPS NAS algorithms must not be readable before the UE accepts them")
	}

	ue.MarkEPSNASAlgorithmsDelivered()

	if ue.NeedsEPSNASAlgorithms() {
		t.Error("an accepted selection settles the debt")
	}

	got, ok := ue.EPSNASAlgorithmsInUse()
	if !ok || got.Ciphering != nas.CipheringAES || got.Integrity != nas.IntegrityAES {
		t.Fatalf("EPSNASAlgorithmsInUse = (%v, %s, %s), want 128-EEA2/128-EIA2", ok, got.Ciphering, got.Integrity)
	}
}

func TestAcceptEPSNASAlgorithmsWithoutSelection(t *testing.T) {
	ue := epsCapableUE(t, s1NetworkCapability(0xe0, 0xe0, 0, 0))
	ue.MarkEPSNASAlgorithmsDelivered()

	if _, ok := ue.EPSNASAlgorithmsInUse(); ok {
		t.Error("accepting without a selection must not invent one")
	}

	if !ue.NeedsEPSNASAlgorithms() {
		t.Error("accepting without a selection must not settle the debt")
	}
}

func mustReplayed(t *testing.T, ue *amf.UeContext) []byte {
	t.Helper()

	capability, ok := ue.EPSSecurityCapability()
	if !ok {
		t.Fatal("the UE has no EPS security capability")
	}

	raw, err := capability.MarshalBinary()
	if err != nil {
		t.Fatalf("encode EPS security capability: %v", err)
	}

	return raw
}

func TestEPSSecurityCapability(t *testing.T) {
	ue := epsCapableUE(t, s1NetworkCapability(0xe0, 0xc0, 0x80, 0xC0))

	got := mustReplayed(t, ue)
	want := []byte{0xe0, 0xc0, 0x80, 0x40}

	if !bytes.Equal(got, want) {
		t.Fatalf("EPS security capability = % x, want % x", got, want)
	}

	// A UE with no UMTS algorithms replays two octets only.
	noUMTS := epsCapableUE(t, s1NetworkCapability(0xe0, 0xc0, 0x00, 0x00))
	if got, want := mustReplayed(t, noUMTS), []byte{0xe0, 0xc0}; !bytes.Equal(got, want) {
		t.Fatalf("EPS security capability = % x, want % x", got, want)
	}

	if _, ok := amf.NewUeContext().EPSSecurityCapability(); ok {
		t.Error("a UE that sent no S1 capability has no EPS security capability")
	}
}

// TS 24.501 §5.5.1.3.2
func TestChangedS1CapabilityForgetsTheEPSNASAlgorithms(t *testing.T) {
	ue := epsCapableUE(t, s1NetworkCapability(0xe0, 0xe0, 0, 0))

	if _, ok := ue.SelectEPSNASAlgorithms([]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES}); !ok {
		t.Fatal("selection failed")
	}

	ue.MarkEPSNASAlgorithmsDelivered()

	// Re-sending the same capability changes nothing.
	ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, s1NetworkCapability(0xe0, 0xe0, 0, 0))

	if _, ok := ue.EPSNASAlgorithmsInUse(); !ok {
		t.Fatal("an unchanged capability must not discard the delivered algorithms")
	}

	// A different one does.
	ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, s1NetworkCapability(0xc0, 0xc0, 0, 0))

	if _, ok := ue.EPSNASAlgorithmsInUse(); ok {
		t.Error("a changed S1 UE network capability must discard the delivered algorithms")
	}

	if !ue.NeedsEPSNASAlgorithms() {
		t.Error("a changed S1 UE network capability must reopen the debt")
	}
}

func TestNewOfferKeepsTheDeliveredEPSNASAlgorithms(t *testing.T) {
	ue := epsCapableUE(t, s1NetworkCapability(0xe0, 0xe0, 0, 0))

	if _, ok := ue.SelectEPSNASAlgorithms([]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES}); !ok {
		t.Fatal("selection failed")
	}

	ue.MarkEPSNASAlgorithmsDelivered()

	if _, ok := ue.SelectEPSNASAlgorithms([]nas.IntegrityAlgorithm{nas.IntegritySNOW3G}, []nas.CipheringAlgorithm{nas.CipheringSNOW3G}); !ok {
		t.Fatal("re-selection failed")
	}

	got, ok := ue.EPSNASAlgorithmsInUse()
	if !ok || got.Ciphering != nas.CipheringAES || got.Integrity != nas.IntegrityAES {
		t.Fatalf("in use = (%v, %s, %s), want the accepted 128-EEA2/128-EIA2", ok, got.Ciphering, got.Integrity)
	}

	ue.MarkEPSNASAlgorithmsDelivered()

	if got, _ := ue.EPSNASAlgorithmsInUse(); got.Ciphering != nas.CipheringSNOW3G {
		t.Fatalf("in use = %s, want the newly accepted 128-EEA1", got.Ciphering)
	}
}

// TS 24.501 §8.2.25.4, §8.2.25.8
func TestBuildSecurityModeCommandCarriesEPSNASAlgorithms(t *testing.T) {
	ue := buildTestUE(t)
	ue.SetUESecurityCapabilityForTest(amf.UESecCapForTest([]uint8{2}, []uint8{2}))
	ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, s1NetworkCapability(0xe0, 0xe0, 0x00, 0x00))
	attachTestConn(t, ue)

	if _, ok := ue.SelectEPSNASAlgorithms([]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES}); !ok {
		t.Fatal("selection failed")
	}

	raw, err := amf.BuildSecurityModeCommand(ue)
	if err != nil {
		t.Fatalf("BuildSecurityModeCommand: %v", err)
	}

	smc, err := fgs.ParseSecurityModeCommand(decryptNAS(t, ue, raw))
	if err != nil {
		t.Fatalf("parse SecurityModeCommand: %v", err)
	}

	if smc.SelectedEPSNASSecurityAlgorithms == nil {
		t.Fatal("the Selected EPS NAS security algorithms IE is missing")
	}

	if got := *smc.SelectedEPSNASSecurityAlgorithms; got.Ciphering != 2 || got.Integrity != 2 {
		t.Fatalf("selected EPS NAS algorithms = %+v, want EEA2/EIA2", got)
	}

	if want := []byte{0xe0, 0xe0}; !bytes.Equal(smc.ReplayedS1UESecurityCapability, want) {
		t.Fatalf("replayed S1 UE security capability = % x, want % x", smc.ReplayedS1UESecurityCapability, want)
	}
}

func TestBuildSecurityModeCommandWithoutEPSNASAlgorithms(t *testing.T) {
	ue := buildTestUE(t)
	ue.SetUESecurityCapabilityForTest(amf.UESecCapForTest([]uint8{2}, []uint8{2}))
	attachTestConn(t, ue)

	raw, err := amf.BuildSecurityModeCommand(ue)
	if err != nil {
		t.Fatalf("BuildSecurityModeCommand: %v", err)
	}

	smc, err := fgs.ParseSecurityModeCommand(decryptNAS(t, ue, raw))
	if err != nil {
		t.Fatalf("parse SecurityModeCommand: %v", err)
	}

	if smc.SelectedEPSNASSecurityAlgorithms != nil || smc.ReplayedS1UESecurityCapability != nil {
		t.Fatal("EPS interworking IEs must not be sent before the UE has disclosed its EPS capability")
	}
}

// TS 24.501 §5.4.2.2, §5.4.2.3
func TestBuildEPSNASAlgorithmsSecurityModeCommand(t *testing.T) {
	ue := buildTestUE(t)
	ue.SetUESecurityCapabilityForTest(amf.UESecCapForTest([]uint8{2}, []uint8{2}))
	ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, s1NetworkCapability(0xe0, 0xe0, 0x00, 0x00))

	if _, err := amf.BuildEPSNASAlgorithmsSecurityModeCommand(ue); err == nil {
		t.Fatal("a command with nothing to deliver must not be built")
	}

	if _, ok := ue.SelectEPSNASAlgorithms([]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES}); !ok {
		t.Fatal("selection failed")
	}

	raw, err := amf.BuildEPSNASAlgorithmsSecurityModeCommand(ue)
	if err != nil {
		t.Fatalf("BuildEPSNASAlgorithmsSecurityModeCommand: %v", err)
	}

	if got := fgs.SecurityHeaderType(raw[1] & 0x0F); got != fgs.SHTIntegrityProtected {
		t.Fatalf("security header type = %v, want integrity protected", got)
	}

	smc, err := fgs.ParseSecurityModeCommand(decryptNAS(t, ue, raw))
	if err != nil {
		t.Fatalf("parse SecurityModeCommand: %v", err)
	}

	if smc.SelectedEPSNASSecurityAlgorithms == nil {
		t.Fatal("the Selected EPS NAS security algorithms IE is missing")
	}

	if smc.IMEISVRequested != nil || smc.AdditionalSecurityInformation != nil {
		t.Fatal("the follow-up command must not re-key or re-request the equipment identity")
	}

	if smc.CipheringAlgorithm != ue.NEA() || smc.IntegrityAlgorithm != ue.NIA() {
		t.Fatal("the follow-up command must re-signal the algorithms already in force")
	}
}
