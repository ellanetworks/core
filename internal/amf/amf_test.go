// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
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

func TestAddAmfUeToUePool_EmptySupi(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	ue := amf.NewAmfUe()

	err := amfInstance.AddAmfUeToUePool(ue)
	if err == nil {
		t.Fatal("expected error for empty SUPI, got nil")
	}
}

func TestAddAmfUeToUePool_Success(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000002")

	ue := amf.NewAmfUe()
	ue.Supi = supi
	ue.Log = zap.NewNop()

	if err := amfInstance.AddAmfUeToUePool(ue); err != nil {
		t.Fatalf("AddAmfUeToUePool: %v", err)
	}

	found, ok := amfInstance.FindAMFUEBySupi(supi)
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

	_, ok := amfInstance.FindAMFUEBySupi(supi)
	if ok {
		t.Fatal("expected not found for missing UE")
	}
}

func TestFindAMFUEBySuci(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	addTestUE(t, amfInstance, "001010000000004", func(ue *amf.AmfUe) {
		ue.Suci = "suci-0-001-01-0000-0-0-0000000004"
	})

	found, ok := amfInstance.FindAMFUEBySuci("suci-0-001-01-0000-0-0-0000000004")
	if !ok {
		t.Fatal("expected to find UE by SUCI")
	}

	if found.Suci != "suci-0-001-01-0000-0-0-0000000004" {
		t.Fatalf("unexpected SUCI: %s", found.Suci)
	}
}

func TestDeregisterAndRemoveAMFUE(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000005")

	ue := amf.NewAmfUe()
	ue.Supi = supi
	ue.Log = zap.NewNop()

	if err := amfInstance.AddAmfUeToUePool(ue); err != nil {
		t.Fatalf("AddAmfUeToUePool: %v", err)
	}

	amfInstance.DeregisterAndRemoveAMFUE(context.Background(), ue)

	_, ok := amfInstance.FindAMFUEBySupi(supi)
	if ok {
		t.Fatal("UE should have been removed after deregistration")
	}
}

func TestRemoveUEBySupi(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000006")

	ue := amf.NewAmfUe()
	ue.Supi = supi
	ue.Log = zap.NewNop()

	if err := amfInstance.AddAmfUeToUePool(ue); err != nil {
		t.Fatalf("AddAmfUeToUePool: %v", err)
	}

	amfInstance.RemoveUEBySupi(supi)

	_, ok := amfInstance.FindAMFUEBySupi(supi)
	if ok {
		t.Fatal("UE should have been removed")
	}
}

func TestCountRegisteredSubscribers(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	if count := amfInstance.CountRegisteredSubscribers(); count != 0 {
		t.Fatalf("expected 0 registered, got %d", count)
	}

	addTestUE(t, amfInstance, "001010000000007", func(ue *amf.AmfUe) {
		ue.ForceState(amf.Registered)
	})

	addTestUE(t, amfInstance, "001010000000008", func(ue *amf.AmfUe) {
		// default state is Deregistered
	})

	if count := amfInstance.CountRegisteredSubscribers(); count != 1 {
		t.Fatalf("expected 1 registered, got %d", count)
	}
}

func TestFindAmfUeByGuti(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	tmsi, err := etsi.NewTMSI(42)
	if err != nil {
		t.Fatalf("NewTMSI: %v", err)
	}

	guti, err := etsi.NewGUTI("001", "01", "cafe00", tmsi)
	if err != nil {
		t.Fatalf("NewGUTI: %v", err)
	}

	addTestUE(t, amfInstance, "001010000000009", func(ue *amf.AmfUe) {
		ue.Guti = guti
	})

	found, ok := amfInstance.FindAmfUeByGuti(guti)
	if !ok {
		t.Fatal("expected to find UE by GUTI")
	}

	if found.Guti != guti {
		t.Fatalf("GUTI mismatch: got %v, want %v", found.Guti, guti)
	}
}

func TestFindAmfUeByGuti_InvalidGUTI(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	addTestUE(t, amfInstance, "001010000000010", func(ue *amf.AmfUe) {})

	_, ok := amfInstance.FindAmfUeByGuti(etsi.InvalidGUTI)
	if ok {
		t.Fatal("should not find UE with InvalidGUTI")
	}
}

func TestGetUESnapshot(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000011")

	_, ok := amfInstance.GetUESnapshot(supi)
	if ok {
		t.Fatal("expected no snapshot for missing UE")
	}

	now := time.Now()

	addTestUE(t, amfInstance, "001010000000011", func(ue *amf.AmfUe) {
		ue.ForceState(amf.Registered)
		ue.LastSeenAt = now
	})

	snap, ok := amfInstance.GetUESnapshot(supi)
	if !ok {
		t.Fatal("expected snapshot for existing UE")
	}

	if snap.LastSeenAt != now {
		t.Fatalf("LastSeenAt mismatch: got %v, want %v", snap.LastSeenAt, now)
	}
}

func TestDeregisterSubscriber(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000012")

	addTestUE(t, amfInstance, "001010000000012", func(ue *amf.AmfUe) {})

	amfInstance.DeregisterSubscriber(context.Background(), supi)

	_, ok := amfInstance.FindAMFUEBySupi(supi)
	if ok {
		t.Fatal("UE should have been removed after DeregisterSubscriber")
	}
}
