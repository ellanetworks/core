// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

// setupAdapterTestDB creates a database with a data network, profile,
// slice, policy, and subscriber, returning the adapter under test, the
// data network name, its pool ID, and the subscriber IMSI.
func setupAdapterTestDB(t *testing.T) (*smfDBAdapter, string, string, string) {
	t.Helper()

	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabase: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %s", err)
		}
	})

	dnn := &db.DataNetwork{Name: "test-dnn", IPv4Pool: "192.168.1.0/24", DNS: "8.8.8.8", MTU: 1400}
	if err := database.CreateDataNetwork(ctx, dnn); err != nil {
		t.Fatalf("CreateDataNetwork: %s", err)
	}

	createdDNN, err := database.GetDataNetwork(ctx, dnn.Name)
	if err != nil {
		t.Fatalf("GetDataNetwork: %s", err)
	}

	profile := &db.Profile{Name: "test-profile", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}
	if err := database.CreateProfile(ctx, profile); err != nil {
		t.Fatalf("CreateProfile: %s", err)
	}

	createdProfile, err := database.GetProfile(ctx, profile.Name)
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	slice := &db.NetworkSlice{Name: "test-slice", Sst: 1}
	if err := database.CreateNetworkSlice(ctx, slice); err != nil {
		t.Fatalf("CreateNetworkSlice: %s", err)
	}

	createdSlice, err := database.GetNetworkSlice(ctx, slice.Name)
	if err != nil {
		t.Fatalf("GetNetworkSlice: %s", err)
	}

	policy := &db.Policy{Name: "test-policy", DataNetworkID: createdDNN.ID, ProfileID: createdProfile.ID, SliceID: createdSlice.ID}
	if err := database.CreatePolicy(ctx, policy); err != nil {
		t.Fatalf("CreatePolicy: %s", err)
	}

	imsi := "001010123456789"
	sub := &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      createdProfile.ID,
	}

	if err := database.CreateSubscriber(ctx, sub); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	return &smfDBAdapter{db: database}, dnn.Name, createdDNN.ID, imsi
}

func TestReleaseIP_StaticKeepsReservation(t *testing.T) {
	adapter, dnn, poolID, imsi := setupAdapterTestDB(t)
	ctx := context.Background()

	pinned := netip.MustParseAddr("192.168.1.50")
	if err := adapter.db.CreateStaticLease(ctx, imsi, poolID, "ipv4", pinned); err != nil {
		t.Fatalf("CreateStaticLease: %s", err)
	}

	got, err := adapter.AllocateIP(ctx, imsi, dnn, 7)
	if err != nil {
		t.Fatalf("AllocateIP: %s", err)
	}

	if got != pinned {
		t.Fatalf("expected pinned address %s, got %s", pinned, got)
	}

	released, err := adapter.ReleaseIP(ctx, imsi, dnn, 7)
	if err != nil {
		t.Fatalf("ReleaseIP: %s", err)
	}

	if released != pinned {
		t.Fatalf("expected released address %s, got %s", pinned, released)
	}

	// Reservation row persists, returned to reserved state.
	reserved, err := adapter.db.GetStaticLease(ctx, poolID, "ipv4", imsi)
	if err != nil {
		t.Fatalf("GetStaticLease after release: %s", err)
	}

	if reserved.SessionID != nil {
		t.Fatalf("expected reserved lease after release, got sessionID %v", *reserved.SessionID)
	}

	if reserved.Address() != pinned {
		t.Fatalf("expected reservation to persist at %s, got %s", pinned, reserved.Address())
	}
}

func TestReleaseIP_DynamicDeletesLease(t *testing.T) {
	adapter, dnn, poolID, imsi := setupAdapterTestDB(t)
	ctx := context.Background()

	got, err := adapter.AllocateIP(ctx, imsi, dnn, 9)
	if err != nil {
		t.Fatalf("AllocateIP: %s", err)
	}

	if _, err := adapter.db.GetLeaseBySession(ctx, poolID, "ipv4", 9, imsi); err != nil {
		t.Fatalf("expected dynamic lease for session 9, got %v", err)
	}

	released, err := adapter.ReleaseIP(ctx, imsi, dnn, 9)
	if err != nil {
		t.Fatalf("ReleaseIP: %s", err)
	}

	if released != got {
		t.Fatalf("expected released address %s, got %s", got, released)
	}

	if _, err := adapter.db.GetLeaseBySession(ctx, poolID, "ipv4", 9, imsi); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("expected dynamic lease deleted (ErrNotFound), got %v", err)
	}
}
