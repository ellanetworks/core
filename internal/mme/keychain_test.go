// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"sync"
	"testing"
)

// TS 33.401 §7.2.8
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

// TS 33.401 §7.2.8
func TestKeyChainMutualExclusion_SecurityModeVsHandover(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	if !m.TryClaimKeyChain(ue) {
		t.Fatal("expected to claim a free key chain")
	}

	if _, _, _, ok := m.BeginPathSwitch(ue); ok {
		t.Fatal("Path Switch started while a security mode procedure held the key chain")
	}

	m.ClearKeyChainBusy(ue)

	if _, _, _, ok := m.BeginPathSwitch(ue); !ok {
		t.Fatal("Path Switch refused after the key chain was released")
	}

	if m.TryClaimKeyChain(ue) {
		t.Fatal("security mode claimed the key chain while a Path Switch held it")
	}
}

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

// TS 33.401 §7.2.8
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
