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

	for i := range teIDRange {
		seID := uint64(i)

		teID, err := resourceManager.AllocateTEID(seID)
		if err != nil {
			t.Fatalf("Expected nil, got %v", err)
		}

		if teID != i+1 {
			t.Fatalf("Expected %d, got %d", i+1, teID)
		}
	}

	_, err = resourceManager.AllocateTEID(uint64(teIDRange))
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	for i := range teIDRange {
		resourceManager.ReleaseTEID(uint64(i), i+1)
	}

	for i := range teIDRange {
		seID := uint64(i)

		teID, err := resourceManager.AllocateTEID(seID)
		if err != nil {
			t.Fatalf("Expected nil, got %v", err)
		}

		if teID != i+1 {
			t.Fatalf("Expected %d, got %d", i+1, teID)
		}
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
