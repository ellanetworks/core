// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/upf/engine"
)

func TestResourceManagerEmptyRange(t *testing.T) {
	resourceManager, err := engine.NewFteIDResourceManager(0)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if resourceManager != nil {
		t.Fatalf("Expected nil, got %v", resourceManager)
	}
}

func TestResourceManagerNonEmptyRange(t *testing.T) {
	teIDRange := uint32(100)

	resourceManager, err := engine.NewFteIDResourceManager(teIDRange)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}

	if resourceManager == nil {
		t.Fatalf("Expected resource manager, got nil")
	}

	allocated := make(map[uint32]uint64, teIDRange)

	for i := range teIDRange {
		seID := uint64(i)

		teID, err := resourceManager.AllocateTEID(seID)
		if err != nil {
			t.Fatalf("Expected nil, got %v", err)
		}

		if teID == 0 {
			t.Fatalf("allocated TEID 0, which TS 29.281 §5.1 forbids")
		}

		if _, dup := allocated[teID]; dup {
			t.Fatalf("TEID %d allocated twice", teID)
		}

		allocated[teID] = seID
	}

	_, err = resourceManager.AllocateTEID(uint64(teIDRange))
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	for teID, seID := range allocated {
		resourceManager.ReleaseTEID(seID, teID)
	}

	reallocated := make(map[uint32]struct{}, teIDRange)

	for i := range teIDRange {
		teID, err := resourceManager.AllocateTEID(uint64(i))
		if err != nil {
			t.Fatalf("Expected nil, got %v", err)
		}

		if _, dup := reallocated[teID]; dup {
			t.Fatalf("TEID %d allocated twice after release", teID)
		}

		reallocated[teID] = struct{}{}
	}

	if _, err := resourceManager.AllocateTEID(uint64(teIDRange)); err == nil {
		t.Fatalf("Expected error after refilling the pool, got nil")
	}
}

func TestResourceManagerTEIDsAreNotSequential(t *testing.T) {
	const count = 64

	m, err := engine.NewFteIDResourceManager(count)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	teids := make([]uint32, 0, count)

	for range count {
		teid, err := m.AllocateTEID(1)
		if err != nil {
			t.Fatalf("allocate: %v", err)
		}

		teids = append(teids, teid)
	}

	sequential := true

	for i := 1; i < len(teids); i++ {
		if teids[i] != teids[i-1]+1 {
			sequential = false
			break
		}
	}

	if sequential {
		t.Fatalf("TEIDs %v are sequential, want unpredictable allocation (TS 33.250 §4.2.3.5.1)", teids)
	}

	small := 0

	for _, teid := range teids {
		if teid <= count {
			small++
		}
	}

	if small == count {
		t.Fatalf("every TEID falls in 1..%d, want values drawn from the full 32-bit space", count)
	}
}

func TestResourceManagerReleaseByOtherSessionIgnored(t *testing.T) {
	m, err := engine.NewFteIDResourceManager(1)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	teid, err := m.AllocateTEID(1)
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}

	m.ReleaseTEID(2, teid)

	if _, err := m.AllocateTEID(2); err == nil {
		t.Fatal("release by a session that does not own the TEID freed it")
	}

	m.ReleaseTEID(1, teid)

	if _, err := m.AllocateTEID(2); err != nil {
		t.Fatalf("allocate after owner release: %v", err)
	}
}

func TestResourceManagerMultipleTEIDsPerSession(t *testing.T) {
	m, err := engine.NewFteIDResourceManager(3)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	const seID = uint64(42)

	teids := make([]uint32, 0, 3)

	for range 3 {
		teid, err := m.AllocateTEID(seID)
		if err != nil {
			t.Fatalf("allocate: %v", err)
		}

		teids = append(teids, teid)
	}

	if _, err := m.AllocateTEID(seID); err == nil {
		t.Fatal("expected pool exhausted after 3 allocations")
	}

	for _, teid := range teids {
		m.ReleaseTEID(seID, teid)
		m.ReleaseTEID(seID, teid)
	}

	for range 3 {
		if _, err := m.AllocateTEID(seID); err != nil {
			t.Fatalf("pool not fully restored: %v", err)
		}
	}

	if _, err := m.AllocateTEID(seID); err == nil {
		t.Fatal("expected exactly 3 TEIDs available after release")
	}
}

func TestResourceManagerReleaseAllTEIDs(t *testing.T) {
	m, err := engine.NewFteIDResourceManager(3)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	const seID = uint64(7)

	for range 3 {
		if _, err := m.AllocateTEID(seID); err != nil {
			t.Fatalf("allocate: %v", err)
		}
	}

	m.ReleaseAllTEIDs(seID)

	for range 3 {
		if _, err := m.AllocateTEID(seID); err != nil {
			t.Fatalf("pool not restored after ReleaseAllTEIDs: %v", err)
		}
	}
}
