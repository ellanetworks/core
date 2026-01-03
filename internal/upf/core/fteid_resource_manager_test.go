package core_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/upf/core"
)

func TestResourceManagerEmptyRange(t *testing.T) {
	resourceManager, err := core.NewFteIDResourceManager(0)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if resourceManager != nil {
		t.Fatalf("Expected nil, got %v", resourceManager)
	}
}

func TestResourceManagerNonEmptyRange(t *testing.T) {
	teIDRange := uint32(100)

	resourceManager, err := core.NewFteIDResourceManager(teIDRange)
	if err != nil {
		t.Fatalf("Expected nil, got %v", err)
	}

	if resourceManager == nil {
		t.Fatalf("Expected resource manager, got nil")
	}

	// Allocate all resources
	for i := uint32(0); i < teIDRange; i++ {
		seID := uint64(i)
		pdrID := i

		teID, err := resourceManager.AllocateTEID(seID, pdrID)
		if err != nil {
			t.Fatalf("Expected nil, got %v", err)
		}

		if teID != i+1 {
			t.Fatalf("Expected %d, got %d", i+1, teID)
		}
	}

	// Try to allocate one more resource
	_, err = resourceManager.AllocateTEID(uint64(teIDRange), teIDRange)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	// Release all resources
	for i := uint32(0); i < teIDRange; i++ {
		resourceManager.ReleaseTEID(uint64(i))
	}

	// Allocate all resources again
	for i := uint32(0); i < teIDRange; i++ {
		seID := uint64(i)
		pdrID := i

		teID, err := resourceManager.AllocateTEID(seID, pdrID)
		if err != nil {
			t.Fatalf("Expected nil, got %v", err)
		}

		if teID != i+1 {
			t.Fatalf("Expected %d, got %d", i+1, teID)
		}
	}
}
