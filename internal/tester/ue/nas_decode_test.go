// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// amfSide is the network end of the NAS security context under test: the same
// algorithms and keys as the UE, used to protect the downlink messages the tests
// feed to DecodeNAS.
func amfSide(t *testing.T, sec *UESecurity) *nas.SecurityContext {
	t.Helper()

	return mustSecurityContext(t,
		nas.IntegrityAlgorithm(sec.IntegrityAlg), nas.CipheringAlgorithm(sec.CipheringAlg),
		sec.KnasInt, sec.KnasEnc)
}

// amfNewContext is the network end of the 5G NAS security context a SECURITY MODE
// COMMAND takes into use: 128-NIA2/128-NEA2, the pair the SECURITY MODE COMMANDs
// in these tests select.
func amfNewContext(t *testing.T, knasInt, knasEnc [16]uint8) *nas.SecurityContext {
	t.Helper()

	return amfSide(t, &UESecurity{
		IntegrityAlg: AlgIntegrity128NIA2, CipheringAlg: AlgCiphering128NEA2,
		KnasInt: knasInt, KnasEnc: knasEnc,
	})
}

// runningContextUE returns a UE whose 5G NAS security context is established with
// 128-NIA2/128-NEA2 and no downlink message accepted yet.
func runningContextUE(t *testing.T) *UE {
	t.Helper()

	sec := goldenUESecurity()
	sec.CipheringAlg = AlgCiphering128NEA2
	sec.IntegrityAlg = AlgIntegrity128NIA2

	return &UE{UeSecurity: sec}
}

// amfProtect protects plain with the AMF-side security context at the given count.
func amfProtect(t *testing.T, sec *UESecurity, count nas.Count, plain []byte) []byte {
	t.Helper()

	sc := amfSide(t, sec)

	wrapped, err := fgs.Protect(plain, fgs.SHTIntegrityProtectedCiphered, count,
		nas.DirectionDownlink, sc)
	if err != nil {
		t.Fatalf("fgs.Protect: %v", err)
	}

	return wrapped
}

func TestDecodeNASAcceptsDownlinkMessagesInOrder(t *testing.T) {
	ue := runningContextUE(t)
	sec := ue.UeSecurity

	for i := uint8(0); i < 3; i++ {
		plain := []byte{0x01, 0x02, 0x03, i}

		wrapped := amfProtect(t, sec, nas.MakeCount(0, i), plain)

		got, err := ue.DecodeNAS(wrapped)
		if err != nil {
			t.Fatalf("count %d: DecodeNAS: %v", i, err)
		}

		if string(got) != string(plain) {
			t.Errorf("count %d: plaintext = %x, want %x", i, got, plain)
		}

		want := nas.MakeCount(0, i)
		if got := ue.UeSecurity.DLRecv.LastAccepted(); got != want {
			t.Errorf("count %d: LastAccepted() = %#06x, want %#06x", i, uint32(got), uint32(want))
		}
	}
}

func TestDecodeNASSurvivesAReorderedDownlinkMessage(t *testing.T) {
	ue := runningContextUE(t)
	sec := ue.UeSecurity

	plain0 := []byte{0x01, 0x02, 0x03, 0x00}
	wrapped0 := amfProtect(t, sec, nas.MakeCount(0, 0), plain0)

	if _, err := ue.DecodeNAS(wrapped0); err != nil {
		t.Fatalf("count 0: DecodeNAS: %v", err)
	}

	plain2 := []byte{0x01, 0x02, 0x03, 0x02}
	wrapped2 := amfProtect(t, sec, nas.MakeCount(0, 2), plain2)

	if _, err := ue.DecodeNAS(wrapped2); err != nil {
		t.Fatalf("count 2: DecodeNAS: %v", err)
	}

	if got := ue.UeSecurity.DLRecv.LastAccepted(); got != nas.MakeCount(0, 2) {
		t.Fatalf("after count 2: LastAccepted() = %#06x, want %#06x",
			uint32(got), uint32(nas.MakeCount(0, 2)))
	}

	plain1 := []byte{0x01, 0x02, 0x03, 0x01}
	wrapped1 := amfProtect(t, sec, nas.MakeCount(0, 1), plain1)

	if _, err := ue.DecodeNAS(wrapped1); err == nil {
		t.Fatal("count 1: DecodeNAS succeeded, expected out-of-order rejection")
	}

	if got := ue.UeSecurity.DLRecv.LastAccepted(); got != nas.MakeCount(0, 2) {
		t.Errorf("after count 1 rejection: LastAccepted() = %#06x, want %#06x (unchanged)",
			uint32(got), uint32(nas.MakeCount(0, 2)))
	}

	plain3 := []byte{0x01, 0x02, 0x03, 0x03}
	wrapped3 := amfProtect(t, sec, nas.MakeCount(0, 3), plain3)

	if _, err := ue.DecodeNAS(wrapped3); err != nil {
		t.Fatalf("count 3: DecodeNAS: %v", err)
	}
}

func TestDecodeNASRejectsAReplayedDownlinkMessage(t *testing.T) {
	ue := runningContextUE(t)
	sec := ue.UeSecurity

	plain1 := []byte{0x01, 0x02, 0x03, 0x01}
	wrapped1 := amfProtect(t, sec, nas.MakeCount(0, 1), plain1)

	if _, err := ue.DecodeNAS(wrapped1); err != nil {
		t.Fatalf("count 1: DecodeNAS: %v", err)
	}

	if _, err := ue.DecodeNAS(wrapped1); err == nil {
		t.Fatal("replayed count 1: DecodeNAS succeeded, expected replay rejection")
	}

	plain2 := []byte{0x01, 0x02, 0x03, 0x02}
	wrapped2 := amfProtect(t, sec, nas.MakeCount(0, 2), plain2)

	if _, err := ue.DecodeNAS(wrapped2); err != nil {
		t.Fatalf("count 2 after replay: DecodeNAS: %v", err)
	}
}

func TestDecodeNASAcceptsWrapAroundOfTheSequenceNumber(t *testing.T) {
	sec := goldenUESecurity()
	sec.CipheringAlg = AlgCiphering128NEA2
	sec.IntegrityAlg = AlgIntegrity128NIA2
	sec.DLRecv = nas.NewUplinkCounter(nas.MakeCount(0, 255))

	ue := &UE{UeSecurity: sec}

	plain := []byte{0x01, 0x02, 0x03, 0x00}
	wrapped := amfProtect(t, sec, nas.MakeCount(1, 0), plain)

	if _, err := ue.DecodeNAS(wrapped); err != nil {
		t.Fatalf("wrap-around: DecodeNAS: %v", err)
	}

	want := nas.MakeCount(1, 0)
	if got := ue.UeSecurity.DLRecv.LastAccepted(); got != want {
		t.Errorf("LastAccepted() = %#06x, want %#06x", uint32(got), uint32(want))
	}
}

func TestDecodeNASInstallsTheContextFromASecurityModeCommandAtCountZero(t *testing.T) {
	sec := goldenUESecurity()
	sec.CipheringAlg = AlgCiphering128NEA0 // stale
	sec.IntegrityAlg = AlgIntegrity128NIA0 // stale

	sec.Kamf = make([]uint8, 32)
	for i := range sec.Kamf {
		sec.Kamf[i] = byte(i + 1)
	}

	sec.DLRecv.Reset()
	sec.ULCount = 3
	sec.contextFromAuthentication = true

	ue := &UE{UeSecurity: sec}

	var knasEnc, knasInt [16]uint8
	if err := AlgorithmKeyDerivation(AlgCiphering128NEA2, sec.Kamf,
		&knasEnc, AlgIntegrity128NIA2, &knasInt); err != nil {
		t.Fatalf("key derivation: %v", err)
	}

	smc := &fgs.SecurityModeCommand{
		CipheringAlgorithm:           nas.CipheringAlgorithm(AlgCiphering128NEA2),
		IntegrityAlgorithm:           nas.IntegrityAlgorithm(AlgIntegrity128NIA2),
		NgKSI:                        nas.KeySetIdentifier{Value: 0},
		ReplayedUESecurityCapability: sec.UeSecurityCapability,
	}

	plain, err := smc.MarshalBinary()
	if err != nil {
		t.Fatalf("SMC marshal: %v", err)
	}

	wrapped, err := fgs.Protect(plain, fgs.SHTIntegrityProtectedNewContext,
		nas.MakeCount(0, 0), nas.DirectionDownlink,
		amfNewContext(t, knasInt, knasEnc))
	if err != nil {
		t.Fatalf("fgs.Protect: %v", err)
	}

	got, err := ue.DecodeNAS(wrapped)
	if err != nil {
		t.Fatalf("DecodeNAS: %v", err)
	}

	if string(got) != string(plain) {
		t.Errorf("plaintext = %x, want %x", got, plain)
	}

	if ue.UeSecurity.CipheringAlg != AlgCiphering128NEA2 {
		t.Errorf("CipheringAlg = %d, want %d", ue.UeSecurity.CipheringAlg, AlgCiphering128NEA2)
	}

	if ue.UeSecurity.IntegrityAlg != AlgIntegrity128NIA2 {
		t.Errorf("IntegrityAlg = %d, want %d", ue.UeSecurity.IntegrityAlg, AlgIntegrity128NIA2)
	}

	if ue.UeSecurity.KnasEnc != knasEnc {
		t.Errorf("KnasEnc = %x, want %x", ue.UeSecurity.KnasEnc, knasEnc)
	}

	if ue.UeSecurity.KnasInt != knasInt {
		t.Errorf("KnasInt = %x, want %x", ue.UeSecurity.KnasInt, knasInt)
	}

	want := nas.MakeCount(0, 0)
	if got := ue.UeSecurity.DLRecv.LastAccepted(); got != want {
		t.Errorf("LastAccepted() = %#06x, want %#06x", uint32(got), uint32(want))
	}

	if ue.UeSecurity.ULCount != 0 {
		t.Errorf("ULCount = %d, want 0", ue.UeSecurity.ULCount)
	}

	if ue.UeSecurity.contextFromAuthentication {
		t.Error("contextFromAuthentication = true, want false")
	}
}

func TestDecodeNASVerifiesAnAlgorithmChangeSecurityModeCommandAgainstTheRunningCount(t *testing.T) {
	// TS 24.501 §5.4.2.2: an algorithm-change SECURITY MODE COMMAND on a context
	// already in use carries the next count of the running context (not reset to 0).
	sec := goldenUESecurity()
	sec.CipheringAlg = AlgCiphering128NEA2
	sec.IntegrityAlg = AlgIntegrity128NIA2

	sec.Kamf = make([]uint8, 32)
	for i := range sec.Kamf {
		sec.Kamf[i] = byte(i + 1)
	}

	sec.DLRecv = nas.NewUplinkCounter(nas.MakeCount(0, 4))
	sec.contextFromAuthentication = false

	ue := &UE{UeSecurity: sec}

	var knasEnc, knasInt [16]uint8
	if err := AlgorithmKeyDerivation(AlgCiphering128NEA2, sec.Kamf,
		&knasEnc, AlgIntegrity128NIA2, &knasInt); err != nil {
		t.Fatalf("key derivation: %v", err)
	}

	smc := &fgs.SecurityModeCommand{
		CipheringAlgorithm:           nas.CipheringAlgorithm(AlgCiphering128NEA2),
		IntegrityAlgorithm:           nas.IntegrityAlgorithm(AlgIntegrity128NIA2),
		NgKSI:                        nas.KeySetIdentifier{Value: 0},
		ReplayedUESecurityCapability: sec.UeSecurityCapability,
	}

	plain, err := smc.MarshalBinary()
	if err != nil {
		t.Fatalf("SMC marshal: %v", err)
	}

	wrapped, err := fgs.Protect(plain, fgs.SHTIntegrityProtectedNewContext,
		nas.MakeCount(0, 5), nas.DirectionDownlink,
		amfNewContext(t, knasInt, knasEnc))
	if err != nil {
		t.Fatalf("fgs.Protect: %v", err)
	}

	if _, err := ue.DecodeNAS(wrapped); err != nil {
		t.Fatalf("DecodeNAS: %v", err)
	}

	want := nas.MakeCount(0, 5)
	if got := ue.UeSecurity.DLRecv.LastAccepted(); got != want {
		t.Errorf("LastAccepted() = %#06x, want %#06x", uint32(got), uint32(want))
	}
}

func TestDecodeNASAcceptsARetransmittedSecurityModeCommand(t *testing.T) {
	sec := goldenUESecurity()

	sec.Kamf = make([]uint8, 32)
	for i := range sec.Kamf {
		sec.Kamf[i] = byte(i + 1)
	}

	sec.DLRecv.Reset()
	sec.ULCount = 3
	sec.contextFromAuthentication = true

	ue := &UE{UeSecurity: sec}

	var knasEnc, knasInt [16]uint8
	if err := AlgorithmKeyDerivation(AlgCiphering128NEA2, sec.Kamf,
		&knasEnc, AlgIntegrity128NIA2, &knasInt); err != nil {
		t.Fatalf("key derivation: %v", err)
	}

	smc := &fgs.SecurityModeCommand{
		CipheringAlgorithm:           nas.CipheringAlgorithm(AlgCiphering128NEA2),
		IntegrityAlgorithm:           nas.IntegrityAlgorithm(AlgIntegrity128NIA2),
		NgKSI:                        nas.KeySetIdentifier{Value: 0},
		ReplayedUESecurityCapability: sec.UeSecurityCapability,
	}

	plain, err := smc.MarshalBinary()
	if err != nil {
		t.Fatalf("SMC marshal: %v", err)
	}

	amf := amfNewContext(t, knasInt, knasEnc)

	// First SMC at count 0.
	wrapped0, err := fgs.Protect(plain, fgs.SHTIntegrityProtectedNewContext,
		nas.MakeCount(0, 0), nas.DirectionDownlink, amf)
	if err != nil {
		t.Fatalf("fgs.Protect count 0: %v", err)
	}

	if _, err := ue.DecodeNAS(wrapped0); err != nil {
		t.Fatalf("DecodeNAS count 0: %v", err)
	}

	// Set ULCount to a non-zero value after the first SMC to verify it is not
	// reset a second time by the retransmission.
	ue.UeSecurity.ULCount = nas.MakeCount(0, 7)

	// Retransmission at count 1 (same plaintext, fresh count).
	wrapped1, err := fgs.Protect(plain, fgs.SHTIntegrityProtectedNewContext,
		nas.MakeCount(0, 1), nas.DirectionDownlink, amf)
	if err != nil {
		t.Fatalf("fgs.Protect count 1: %v", err)
	}

	if _, err := ue.DecodeNAS(wrapped1); err != nil {
		t.Fatalf("DecodeNAS count 1 (retransmission): %v", err)
	}

	want := nas.MakeCount(0, 1)
	if got := ue.UeSecurity.DLRecv.LastAccepted(); got != want {
		t.Errorf("LastAccepted() = %#06x, want %#06x", uint32(got), uint32(want))
	}

	// ULCount must not have been reset again; the UE should send SECURITY MODE
	// COMPLETE with the next uplink count, not replay count 0.
	if ue.UeSecurity.ULCount != nas.MakeCount(0, 7) {
		t.Errorf("ULCount = %#06x, want %#06x (not reset on retransmission)",
			uint32(ue.UeSecurity.ULCount), uint32(nas.MakeCount(0, 7)))
	}
}

func TestDecodeNASKeepsTheContextWhenASecurityModeCommandFailsItsMAC(t *testing.T) {
	sec := goldenUESecurity()

	sec.Kamf = make([]uint8, 32)
	for i := range sec.Kamf {
		sec.Kamf[i] = byte(i + 1)
	}

	sec.DLRecv.Reset()
	sec.ULCount = 3
	sec.contextFromAuthentication = true
	sec.IntegrityAlg = AlgIntegrity128NIA0 // stale, and not what the SMC selects
	staleEnc := sec.KnasEnc
	staleInt := sec.KnasInt

	ue := &UE{UeSecurity: sec}

	// Derive keys with a different integrity key so the MAC fails.
	var knasEnc, knasInt [16]uint8

	dummyKamf := make([]uint8, 32)
	for i := range dummyKamf {
		dummyKamf[i] = byte(i + 100)
	}

	if err := AlgorithmKeyDerivation(AlgCiphering128NEA2, dummyKamf,
		&knasEnc, AlgIntegrity128NIA2, &knasInt); err != nil {
		t.Fatalf("key derivation: %v", err)
	}

	smc := &fgs.SecurityModeCommand{
		CipheringAlgorithm:           nas.CipheringAlgorithm(AlgCiphering128NEA2),
		IntegrityAlgorithm:           nas.IntegrityAlgorithm(AlgIntegrity128NIA2),
		NgKSI:                        nas.KeySetIdentifier{Value: 0},
		ReplayedUESecurityCapability: sec.UeSecurityCapability,
	}

	plain, err := smc.MarshalBinary()
	if err != nil {
		t.Fatalf("SMC marshal: %v", err)
	}

	// Protected with keys derived from a different K_AMF, so the NAS-MAC the UE
	// computes over the message with its own derivation cannot match.
	wrapped, err := fgs.Protect(plain, fgs.SHTIntegrityProtectedNewContext,
		nas.MakeCount(0, 0), nas.DirectionDownlink,
		amfNewContext(t, knasInt, knasEnc))
	if err != nil {
		t.Fatalf("fgs.Protect: %v", err)
	}

	if _, err := ue.DecodeNAS(wrapped); err == nil {
		t.Fatal("DecodeNAS with bad MAC succeeded, expected failure")
	}

	// The context must be untouched.
	if ue.UeSecurity.CipheringAlg != AlgCiphering128NEA0 {
		t.Errorf("CipheringAlg = %d, want %d (unchanged)",
			ue.UeSecurity.CipheringAlg, AlgCiphering128NEA0)
	}

	if ue.UeSecurity.IntegrityAlg != AlgIntegrity128NIA0 {
		t.Errorf("IntegrityAlg = %d, want %d (unchanged)",
			ue.UeSecurity.IntegrityAlg, AlgIntegrity128NIA0)
	}

	if ue.UeSecurity.KnasEnc != staleEnc {
		t.Errorf("KnasEnc changed after bad MAC")
	}

	if ue.UeSecurity.KnasInt != staleInt {
		t.Errorf("KnasInt changed after bad MAC")
	}

	if ue.UeSecurity.ULCount != 3 {
		t.Errorf("ULCount = %d, want 3 (unchanged)", ue.UeSecurity.ULCount)
	}

	if !ue.UeSecurity.contextFromAuthentication {
		t.Error("contextFromAuthentication = false, want true (unchanged)")
	}

	if ue.UeSecurity.DLRecv.Accepted() {
		t.Error("DLRecv.Accepted() = true, want false (no message accepted)")
	}

	// Feed the correct SMC at count 0 and assert it is accepted — proving the
	// failure cost one message, not the context.
	correctEnc, correctInt := [16]uint8{}, [16]uint8{}
	if err := AlgorithmKeyDerivation(AlgCiphering128NEA2, sec.Kamf,
		&correctEnc, AlgIntegrity128NIA2, &correctInt); err != nil {
		t.Fatalf("key derivation: %v", err)
	}

	correctWrapped, err := fgs.Protect(plain, fgs.SHTIntegrityProtectedNewContext,
		nas.MakeCount(0, 0), nas.DirectionDownlink,
		amfNewContext(t, correctInt, correctEnc))
	if err != nil {
		t.Fatalf("fgs.Protect correct: %v", err)
	}

	if _, err := ue.DecodeNAS(correctWrapped); err != nil {
		t.Fatalf("DecodeNAS correct SMC: %v", err)
	}
}
