// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/epskeys"
)

func handoverUE(kenbCount uint32) *UE {
	ue := NewUnboundUE()
	ue.kasme = make([]byte, 32)
	ue.kenbCount = kenbCount

	for i := range ue.kasme {
		ue.kasme[i] = byte(i * 9)
	}

	return ue
}

func TestNextHopForNCC1AgreesWithTheCore(t *testing.T) {
	const kenbCount = 4

	ue := handoverUE(kenbCount)

	got, err := ue.SecurityContextForHandoverToFiveGS(1)
	if err != nil {
		t.Fatalf("SecurityContextForHandoverToFiveGS: %v", err)
	}

	kenb, err := epskeys.DeriveKeNB(ue.kasme, kenbCount)
	if err != nil {
		t.Fatalf("DeriveKeNB: %v", err)
	}

	want, err := epskeys.DeriveNH(ue.kasme, kenb[:])
	if err != nil {
		t.Fatalf("DeriveNH: %v", err)
	}

	if !bytes.Equal(got.NH[:], want[:]) {
		t.Errorf("NH = % x, want the % x the MME derived", got.NH, want)
	}

	if !bytes.Equal(got.KASME[:], ue.kasme) {
		t.Errorf("K_ASME = % x, want % x", got.KASME, ue.kasme)
	}
}

func TestNextHopWalksTheChainToTheNCC(t *testing.T) {
	ue := handoverUE(0)

	kenb, err := epskeys.DeriveKeNB(ue.kasme, 0)
	if err != nil {
		t.Fatalf("DeriveKeNB: %v", err)
	}

	want := kenb

	for range 3 {
		want, err = epskeys.DeriveNH(ue.kasme, want[:])
		if err != nil {
			t.Fatalf("DeriveNH: %v", err)
		}
	}

	got, err := ue.SecurityContextForHandoverToFiveGS(3)
	if err != nil {
		t.Fatalf("SecurityContextForHandoverToFiveGS: %v", err)
	}

	if !bytes.Equal(got.NH[:], want[:]) {
		t.Errorf("NH for NCC 3 = % x, want % x", got.NH, want)
	}
}

func TestNextHopRefusesNCC0(t *testing.T) {
	if _, err := handoverUE(0).SecurityContextForHandoverToFiveGS(0); err == nil {
		t.Fatal("NCC 0 names no next hop, so there is nothing to derive K'AMF from")
	}
}
