// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	amfContext "github.com/ellanetworks/core/internal/amf"
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
	amf := amfContext.New(nil, nil, nil)
	ue := amfContext.NewAmfUe()

	err := amf.AddAmfUeToUePool(ue)
	if err == nil {
		t.Fatal("expected error for empty SUPI, got nil")
	}
}

func TestAddAmfUeToUePool_Success(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000002")

	ue := amfContext.NewAmfUe()
	ue.Supi = supi
	ue.Log = zap.NewNop()

	if err := amf.AddAmfUeToUePool(ue); err != nil {
		t.Fatalf("AddAmfUeToUePool: %v", err)
	}

	found, ok := amf.FindAMFUEBySupi(supi)
	if !ok {
		t.Fatal("UE not found after adding")
	}

	if found != ue {
		t.Fatal("found UE does not match added UE")
	}
}

func TestFindAMFUEBySupi_NotFound(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000003")

	_, ok := amf.FindAMFUEBySupi(supi)
	if ok {
		t.Fatal("expected not found for missing UE")
	}
}

func TestFindAMFUEBySuci(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)

	addTestUE(t, amf, "001010000000004", func(ue *amfContext.AmfUe) {
		ue.Suci = "suci-0-001-01-0000-0-0-0000000004"
	})

	found, ok := amf.FindAMFUEBySuci("suci-0-001-01-0000-0-0-0000000004")
	if !ok {
		t.Fatal("expected to find UE by SUCI")
	}

	if found.Suci != "suci-0-001-01-0000-0-0-0000000004" {
		t.Fatalf("unexpected SUCI: %s", found.Suci)
	}
}

func TestDeregisterAndRemoveAMFUE(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000005")

	ue := amfContext.NewAmfUe()
	ue.Supi = supi
	ue.Log = zap.NewNop()

	if err := amf.AddAmfUeToUePool(ue); err != nil {
		t.Fatalf("AddAmfUeToUePool: %v", err)
	}

	amf.DeregisterAndRemoveAMFUE(context.Background(), ue)

	_, ok := amf.FindAMFUEBySupi(supi)
	if ok {
		t.Fatal("UE should have been removed after deregistration")
	}
}

func TestRemoveUEBySupi(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000006")

	ue := amfContext.NewAmfUe()
	ue.Supi = supi
	ue.Log = zap.NewNop()

	if err := amf.AddAmfUeToUePool(ue); err != nil {
		t.Fatalf("AddAmfUeToUePool: %v", err)
	}

	amf.RemoveUEBySupi(supi)

	_, ok := amf.FindAMFUEBySupi(supi)
	if ok {
		t.Fatal("UE should have been removed")
	}
}

func TestCountRegisteredSubscribers(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)

	if count := amf.CountRegisteredSubscribers(); count != 0 {
		t.Fatalf("expected 0 registered, got %d", count)
	}

	addTestUE(t, amf, "001010000000007", func(ue *amfContext.AmfUe) {
		ue.ForceState(amfContext.Registered)
	})

	addTestUE(t, amf, "001010000000008", func(ue *amfContext.AmfUe) {
		// default state is Deregistered
	})

	if count := amf.CountRegisteredSubscribers(); count != 1 {
		t.Fatalf("expected 1 registered, got %d", count)
	}
}

func TestFindAmfUeByGuti(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)

	tmsi, err := etsi.NewTMSI(42)
	if err != nil {
		t.Fatalf("NewTMSI: %v", err)
	}

	guti, err := etsi.NewGUTI("001", "01", "cafe00", tmsi)
	if err != nil {
		t.Fatalf("NewGUTI: %v", err)
	}

	addTestUE(t, amf, "001010000000009", func(ue *amfContext.AmfUe) {
		ue.Guti = guti
	})

	found, ok := amf.FindAmfUeByGuti(guti)
	if !ok {
		t.Fatal("expected to find UE by GUTI")
	}

	if found.Guti != guti {
		t.Fatalf("GUTI mismatch: got %v, want %v", found.Guti, guti)
	}
}

func TestFindAmfUeByGuti_InvalidGUTI(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)

	addTestUE(t, amf, "001010000000010", func(ue *amfContext.AmfUe) {})

	_, ok := amf.FindAmfUeByGuti(etsi.InvalidGUTI)
	if ok {
		t.Fatal("should not find UE with InvalidGUTI")
	}
}

func TestGetUESnapshot(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000011")

	_, ok := amf.GetUESnapshot(supi)
	if ok {
		t.Fatal("expected no snapshot for missing UE")
	}

	now := time.Now()

	addTestUE(t, amf, "001010000000011", func(ue *amfContext.AmfUe) {
		ue.ForceState(amfContext.Registered)
		ue.LastSeenAt = now
	})

	snap, ok := amf.GetUESnapshot(supi)
	if !ok {
		t.Fatal("expected snapshot for existing UE")
	}

	if snap.LastSeenAt != now {
		t.Fatalf("LastSeenAt mismatch: got %v, want %v", snap.LastSeenAt, now)
	}
}

func TestDeregisterSubscriber(t *testing.T) {
	amf := amfContext.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000012")

	addTestUE(t, amf, "001010000000012", func(ue *amfContext.AmfUe) {})

	amf.DeregisterSubscriber(context.Background(), supi)

	_, ok := amf.FindAMFUEBySupi(supi)
	if ok {
		t.Fatal("UE should have been removed after DeregisterSubscriber")
	}
}
