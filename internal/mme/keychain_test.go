// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"sync"
	"testing"
)

// TestPathSwitchNHDerivationRaceFree runs the path-switch NH derivation
// concurrently with a security-context reinstall so the race detector proves
// kasme and the {NH,NCC} chain are read under UeContext.mu, never unlocked
// (TS 33.401 §7.2.8). It exercises the fix; run with -race.
func TestPathSwitchNHDerivationRaceFree(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	var wg sync.WaitGroup
	for range 200 {
		wg.Add(2)

		go func() {
			defer wg.Done()

			_, _ = m.AdvancePathSwitchNH(ue, [32]byte{})
		}()

		go func() {
			defer wg.Done()

			ue.SetKASME(make([]byte, 32))
		}()
	}

	wg.Wait()
}

// TestKeyChainMutualExclusion_SecurityModeVsHandover asserts the TS 33.501
// §6.9.5.1 / TS 33.401 §7.2.8 invariant: a NAS security mode procedure (which
// claims the {NH,NCC} key chain via TryClaimKeyChain) and a key-changing
// handover / Path Switch are mutually exclusive, so they cannot re-key the AS
// context from the same base concurrently. Both directions are checked.
func TestKeyChainMutualExclusion_SecurityModeVsHandover(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	// A security mode procedure claims the key chain.
	if !m.TryClaimKeyChain(ue) {
		t.Fatal("expected to claim a free key chain")
	}

	// A Path Switch / S1 handover must refuse while the security mode holds it.
	if _, _, _, ok := m.BeginPathSwitch(ue); ok {
		t.Fatal("Path Switch started while a security mode procedure held the key chain")
	}

	// SECURITY MODE COMPLETE (or connection release) frees the chain.
	m.ClearKeyChainBusy(ue)

	// A Path Switch now proceeds — and itself claims the chain.
	if _, _, _, ok := m.BeginPathSwitch(ue); !ok {
		t.Fatal("Path Switch refused after the key chain was released")
	}

	// Reverse direction: a security mode must refuse while the Path Switch holds it.
	if m.TryClaimKeyChain(ue) {
		t.Fatal("security mode claimed the key chain while a Path Switch held it")
	}
}

// TestKeyChain_TracksDistinctProcedureType asserts the registry tracks which
// key-changing procedure holds the chain (not just a busy flag), so the
// ongoing-procedures view is informative and symmetric with the AMF.
func TestKeyChain_TracksDistinctProcedureType(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	if !m.TryClaimKeyChain(ue) {
		t.Fatal("expected to claim a free key chain")
	}

	if got := ue.ActiveProceduresForTest(); len(got) != 1 || got[0] != "SecurityMode" {
		t.Fatalf("active procedures = %v, want [SecurityMode]", got)
	}

	m.ClearKeyChainBusy(ue)

	if got := ue.ActiveProceduresForTest(); len(got) != 0 {
		t.Fatalf("active procedures = %v, want none after release", got)
	}

	if _, _, _, ok := m.BeginPathSwitch(ue); !ok {
		t.Fatal("Path Switch refused after release")
	}

	if got := ue.ActiveProceduresForTest(); len(got) != 1 || got[0] != "PathSwitch" {
		t.Fatalf("active procedures = %v, want [PathSwitch]", got)
	}
}

// TestKeyChainBusy_ClearedOnConnectionRelease asserts a key-chain claim does not
// outlive the connection: an in-flight security mode whose Complete never arrives
// must not leave the chain busy and block a later procedure (TS 33.401 §7.2.8).
func TestKeyChainBusy_ClearedOnConnectionRelease(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	if !m.TryClaimKeyChain(ue) {
		t.Fatal("expected to claim a free key chain")
	}

	m.FreeUeConn(ue)

	if !m.TryClaimKeyChain(ue) {
		t.Fatal("key chain still busy after the connection was released")
	}
}
