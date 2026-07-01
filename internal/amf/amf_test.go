// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

func newSUPI(t *testing.T, imsi string) etsi.SUPI {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		t.Fatalf("invalid IMSI %q: %v", imsi, err)
	}

	return supi
}

func TestAddUeContextToUePool_EmptySupi(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	ue := amf.NewUeContext()

	err := amfInstance.AddUeContextToPool(ue)
	if err == nil {
		t.Fatal("expected error for empty SUPI, got nil")
	}
}

func TestAddUeContextToUePool_Success(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000002")

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.Log = zap.NewNop()

	if err := amfInstance.AddUeContextToPool(ue); err != nil {
		t.Fatalf("AddUeContextToPool: %v", err)
	}

	found, ok := amfInstance.FindUeContextBySupi(supi)
	if !ok {
		t.Fatal("UE not found after adding")
	}

	if found != ue {
		t.Fatal("found UE does not match added UE")
	}
}

func TestFindAMFUEBySupi_NotFound(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000003")

	_, ok := amfInstance.FindUeContextBySupi(supi)
	if ok {
		t.Fatal("expected not found for missing UE")
	}
}

func TestDeregisterAndRemoveAMFUE(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000005")

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.Log = zap.NewNop()

	if err := amfInstance.AddUeContextToPool(ue); err != nil {
		t.Fatalf("AddUeContextToPool: %v", err)
	}

	amfInstance.DeregisterAndRemoveUeContext(context.Background(), ue)

	_, ok := amfInstance.FindUeContextBySupi(supi)
	if ok {
		t.Fatal("UE should have been removed after deregistration")
	}
}

func TestRemoveUEBySupi(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000006")

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.Log = zap.NewNop()

	if err := amfInstance.AddUeContextToPool(ue); err != nil {
		t.Fatalf("AddUeContextToPool: %v", err)
	}

	amfInstance.RemoveUEBySupi(supi)

	_, ok := amfInstance.FindUeContextBySupi(supi)
	if ok {
		t.Fatal("UE should have been removed")
	}
}

func TestCountRegisteredSubscribers(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	if count := amfInstance.CountRegisteredSubscribers(); count != 0 {
		t.Fatalf("expected 0 registered, got %d", count)
	}

	addTestUE(t, amfInstance, "001010000000007", func(ue *amf.UeContext) {
		ue.ForceState(amf.Registered)
	})

	addTestUE(t, amfInstance, "001010000000008", func(ue *amf.UeContext) {
		// default state is Deregistered
	})

	if count := amfInstance.CountRegisteredSubscribers(); count != 1 {
		t.Fatalf("expected 1 registered, got %d", count)
	}
}

func TestFindUeContextByGuti(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	tmsi, err := etsi.NewTMSI(42)
	if err != nil {
		t.Fatalf("NewTMSI: %v", err)
	}

	guti, err := etsi.NewGUTI("001", "01", "cafe00", tmsi)
	if err != nil {
		t.Fatalf("NewGUTI: %v", err)
	}

	ue := addTestUE(t, amfInstance, "001010000000009", func(ue *amf.UeContext) {})
	amfInstance.AssignGutiForTest(ue, guti)

	found, ok := amfInstance.FindUeContextByGuti(guti)
	if !ok {
		t.Fatal("expected to find UE by GUTI")
	}

	if found.GutiForTest() != guti {
		t.Fatalf("GUTI mismatch: got %v, want %v", found.GutiForTest(), guti)
	}
}

// TestGutiIndexLifecycle verifies the GUTI resolution index is maintained through
// the production reallocation window (old GUTI resolves until freed) and removal.
func TestGutiIndexLifecycle(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	guami := &models.Guami{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, AmfID: "cafe00"}

	ue := addTestUE(t, amfInstance, "001010000000011", func(ue *amf.UeContext) {})

	if err := amfInstance.ReAllocateGuti(context.Background(), ue, guami); err != nil {
		t.Fatalf("ReAllocateGuti: %v", err)
	}

	guti1 := ue.GutiForTest()
	if found, ok := amfInstance.FindUeContextByGuti(guti1); !ok || found != ue {
		t.Fatal("UE not resolvable by its GUTI after allocation")
	}

	// Reallocation: both the new and the in-flight old GUTI resolve to the UE.
	if err := amfInstance.ReAllocateGuti(context.Background(), ue, guami); err != nil {
		t.Fatalf("ReAllocateGuti (realloc): %v", err)
	}

	guti2 := ue.GutiForTest()
	if guti2 == guti1 {
		t.Fatal("reallocation should produce a new GUTI")
	}

	if found, ok := amfInstance.FindUeContextByGuti(guti2); !ok || found != ue {
		t.Fatal("UE not resolvable by its new GUTI")
	}

	if found, ok := amfInstance.FindUeContextByGuti(guti1); !ok || found != ue {
		t.Fatal("UE not resolvable by its old GUTI during the reallocation window")
	}

	// FreeOldGuti: the old GUTI stops resolving; the current one still resolves.
	amfInstance.FreeOldGuti(ue)

	if _, ok := amfInstance.FindUeContextByGuti(guti1); ok {
		t.Fatal("old GUTI must not resolve after FreeOldGuti")
	}

	if _, ok := amfInstance.FindUeContextByGuti(guti2); !ok {
		t.Fatal("current GUTI must still resolve after FreeOldGuti")
	}

	// Removal: no GUTI resolves to the removed UE.
	amfInstance.RemoveUEBySupi(ue.SupiForTest())

	if _, ok := amfInstance.FindUeContextByGuti(guti2); ok {
		t.Fatal("removed UE must not resolve by GUTI")
	}
}

func TestFindUeContextByGuti_InvalidGUTI(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	addTestUE(t, amfInstance, "001010000000010", func(ue *amf.UeContext) {})

	_, ok := amfInstance.FindUeContextByGuti(etsi.InvalidGUTI)
	if ok {
		t.Fatal("should not find UE with InvalidGUTI")
	}
}

func TestGetUESnapshot(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000011")

	_, ok := amfInstance.UESnapshot(supi)
	if ok {
		t.Fatal("expected no snapshot for missing UE")
	}

	now := time.Now()

	addTestUE(t, amfInstance, "001010000000011", func(ue *amf.UeContext) {
		ue.ForceState(amf.Registered)
		ue.SetLastSeenForTest(now, "")
	})

	snap, ok := amfInstance.UESnapshot(supi)
	if !ok {
		t.Fatal("expected snapshot for existing UE")
	}

	if !snap.LastSeenAt.Equal(now) {
		t.Fatalf("LastSeenAt mismatch: got %v, want %v", snap.LastSeenAt, now)
	}
}

func TestDeregisterSubscriber(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000012")

	addTestUE(t, amfInstance, "001010000000012", func(ue *amf.UeContext) {})

	amfInstance.DeregisterSubscriber(context.Background(), supi)

	_, ok := amfInstance.FindUeContextBySupi(supi)
	if ok {
		t.Fatal("UE should have been removed after DeregisterSubscriber")
	}
}
