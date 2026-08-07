// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "testing"

// TS 33.401 §7.2.5.2.2: with a NAS security mode control procedure before the AS
// SMC, K_eNB derives from the uplink NAS COUNT of the SECURITY MODE COMPLETE. Any
// uplink message the MME receives between it and the Initial Context Setup — an
// ESM INFORMATION RESPONSE, say — advances the count but is not what the UE
// derives from.
func TestKeNBFreshnessSurvivesALaterUplinkMessage(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.SetULCountForTest(0)
	ue.CommitUplinkCount(0)
	ue.PinKeNBFreshness()

	pinned, pinnedCount, err := ue.DeriveInitialKeNB()
	if err != nil {
		t.Fatalf("derive K_eNB at the Security Mode Complete: %v", err)
	}

	if pinnedCount != 0 {
		t.Fatalf("pinned K_eNB NAS COUNT = %d, want 0", pinnedCount)
	}

	ue.CommitUplinkCount(1)

	kenb, count, err := ue.DeriveInitialKeNB()
	if err != nil {
		t.Fatalf("derive K_eNB at the Initial Context Setup: %v", err)
	}

	if count != 0 {
		t.Errorf("K_eNB NAS COUNT = %d, want the pinned 0; an intervening uplink message moved it", count)
	}

	if kenb != pinned {
		t.Error("K_eNB changed after an intervening uplink message; the UE would derive a different key")
	}
}

// A later procedure that triggers its own context setup re-pins, so a UE
// returning from idle does not derive against the attach's count.
func TestKeNBFreshnessRepinsForALaterTrigger(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.SetULCountForTest(0)
	ue.CommitUplinkCount(0)
	ue.PinKeNBFreshness()

	attach, _, err := ue.DeriveInitialKeNB()
	if err != nil {
		t.Fatalf("derive K_eNB at attach: %v", err)
	}

	ue.CommitUplinkCount(1)
	ue.PinKeNBFreshness()

	resume, count, err := ue.DeriveInitialKeNB()
	if err != nil {
		t.Fatalf("derive K_eNB on resume: %v", err)
	}

	if count != 1 {
		t.Errorf("K_eNB NAS COUNT = %d, want 1", count)
	}

	if resume == attach {
		t.Error("K_eNB unchanged across two triggers with different NAS COUNTs")
	}
}

// A fresh security context restarts the counts, and zero is the freshness it
// derives from (TS 33.401 §7.2.5.2.3).
func TestKeNBFreshnessResetsWithTheSecurityContext(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.SetULCountForTest(0)
	ue.CommitUplinkCount(4)
	ue.PinKeNBFreshness()

	if _, count, err := ue.DeriveInitialKeNB(); err != nil || count != 4 {
		t.Fatalf("K_eNB NAS COUNT = %d (err %v), want 4", count, err)
	}

	if err := ue.InstallNASSecurityContext(2, 2, MintAuthProofForSecurityMode()); err != nil {
		t.Fatal(err)
	}

	if _, count, err := ue.DeriveInitialKeNB(); err != nil || count != 0 {
		t.Fatalf("K_eNB NAS COUNT after a fresh context = %d (err %v), want 0", count, err)
	}
}
