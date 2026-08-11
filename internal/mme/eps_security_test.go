// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/epskeys"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func relocatedContext() interworking.EPSSecurityContext {
	in := interworking.EPSSecurityContext{
		EKSI:                 nas.KeySetIdentifier{Value: 4, Mapped: true},
		ULNASCount:           nas.MakeCount(0, 3),
		DLNASCount:           nas.MakeCount(0, 6),
		Algorithms:           interworking.EPSNASAlgorithms{Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES},
		UESecurityCapability: eps.UESecurityCapability{EEA: 0xe0, EIA: 0xe0, HasUMTS: true, UEA: 0x80, UIA: 0x40},
		NCC:                  2,
	}

	for i := range in.KASME {
		in.KASME[i] = byte(i + 1)
		in.NH[i] = byte(i + 2)
	}

	return in
}

// TS 33.501 §8.3.2 step 4
func TestInstallRelocatedSecurityContext(t *testing.T) {
	in := relocatedContext()
	ue := mme.NewUeContext()

	if err := ue.InstallRelocatedSecurityContext(in, mme.MintAuthProofForInterworking()); err != nil {
		t.Fatalf("InstallRelocatedSecurityContext: %v", err)
	}

	wantEnc, err := epskeys.DeriveKNASEnc(in.KASME[:], in.Algorithms.Ciphering)
	if err != nil {
		t.Fatalf("DeriveKNASEnc: %v", err)
	}

	wantInt, err := epskeys.DeriveKNASInt(in.KASME[:], in.Algorithms.Integrity)
	if err != nil {
		t.Fatalf("DeriveKNASInt: %v", err)
	}

	if ue.KnasEncForTest() != wantEnc || ue.KnasIntForTest() != wantInt {
		t.Fatal("the NAS keys must come from the received K'ASME")
	}

	if !ue.Secured() || !ue.HasKASME() {
		t.Fatal("the relocated context must be current")
	}

	if ue.Eksi() != 4 {
		t.Fatalf("eKSI = %d, want the peer's 4", ue.Eksi())
	}

	if ue.EEA() != nas.CipheringAES || ue.EIA() != nas.IntegrityAES {
		t.Fatalf("algorithms = (%s, %s), want the peer's pair", ue.EEA(), ue.EIA())
	}
}

func TestInstallRelocatedSecurityContextKeepsTheKeyChain(t *testing.T) {
	in := relocatedContext()
	ue := mme.NewUeContext()

	if err := ue.InstallRelocatedSecurityContext(in, mme.MintAuthProofForInterworking()); err != nil {
		t.Fatalf("InstallRelocatedSecurityContext: %v", err)
	}

	nh, ncc := ue.NHForTest(), ue.NCCForTest()
	if !bytes.Equal(nh[:], in.NH[:]) || ncc != 2 {
		t.Fatalf("key chain = (%x, %d), want the received pair at NCC 2", nh, ncc)
	}
}

// TS 33.501 §8.6.1
func TestInstallRelocatedSecurityContextKeepsTheCounts(t *testing.T) {
	in := relocatedContext()
	ue := mme.NewUeContext()

	if err := ue.InstallRelocatedSecurityContext(in, mme.MintAuthProofForInterworking()); err != nil {
		t.Fatalf("InstallRelocatedSecurityContext: %v", err)
	}

	if got := ue.NextDownlinkCountForTest(); got != in.DLNASCount {
		t.Fatalf("next downlink NAS COUNT = %d, want the peer's %d", got, in.DLNASCount)
	}
}

func TestInstallRelocatedSecurityContextCapability(t *testing.T) {
	in := relocatedContext()
	ue := mme.NewUeContext()

	if err := ue.InstallRelocatedSecurityContext(in, mme.MintAuthProofForInterworking()); err != nil {
		t.Fatalf("InstallRelocatedSecurityContext: %v", err)
	}

	got := ue.UeNetCap()
	if got.EEA != in.UESecurityCapability.EEA || got.EIA != in.UESecurityCapability.EIA {
		t.Fatalf("EPS algorithms = (%08b, %08b), want the peer's", got.EEA, got.EIA)
	}

	if got.UEA != in.UESecurityCapability.UEA || got.UIA != in.UESecurityCapability.UIA {
		t.Fatal("the UMTS algorithms must carry over")
	}

	if len(got.Rest) != 0 {
		t.Fatal("a security capability carries no feature bits")
	}
}
