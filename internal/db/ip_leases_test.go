// Copyright 2026 Ella Networks

package db_test

import (
	"context"
	"net/netip"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

// setupLeaseTestDB creates a new database with a data network, policy, and
// subscriber ready for lease testing. Returns the database, pool (data network)
// ID, and subscriber IMSI.
func setupLeaseTestDB(t *testing.T) (*db.Database, int, string) {
	t.Helper()

	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabase: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %s", err)
		}
	})

	dnn := &db.DataNetwork{
		Name:   "test-dnn",
		IPPool: "192.168.1.0/24",
		DNS:    "8.8.8.8",
		MTU:    1400,
	}

	if err := database.CreateDataNetwork(context.Background(), dnn); err != nil {
		t.Fatalf("CreateDataNetwork: %s", err)
	}

	createdDNN, err := database.GetDataNetwork(context.Background(), dnn.Name)
	if err != nil {
		t.Fatalf("GetDataNetwork: %s", err)
	}

	policy := &db.Policy{
		Name:          "test-policy",
		DataNetworkID: createdDNN.ID,
	}

	if err := database.CreatePolicy(context.Background(), policy); err != nil {
		t.Fatalf("CreatePolicy: %s", err)
	}

	createdPolicy, err := database.GetPolicy(context.Background(), policy.Name)
	if err != nil {
		t.Fatalf("GetPolicy: %s", err)
	}

	imsi := "001010123456789"
	sub := &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		PolicyID:       createdPolicy.ID,
	}

	if err := database.CreateSubscriber(context.Background(), sub); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	return database, createdDNN.ID, imsi
}

func addr(s string) netip.Addr { return netip.MustParseAddr(s) }

func TestCreateAndGetLease(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	sessionID := 1
	lease := &db.IPLease{
		PoolID:    poolID,
		IMSI:      imsi,
		SessionID: &sessionID,
		Type:      "dynamic",
		CreatedAt: time.Now().Unix(),
	}

	if err := database.CreateLease(ctx, lease, addr("192.168.1.10")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	// Should be retrievable as a dynamic lease.
	got, err := database.GetDynamicLease(ctx, poolID, imsi)
	if err != nil {
		t.Fatalf("GetDynamicLease: %s", err)
	}

	if got.Address() != addr("192.168.1.10") {
		t.Fatalf("expected address 192.168.1.10, got %s", got.Address())
	}

	if got.IMSI != imsi {
		t.Fatalf("expected IMSI %s, got %s", imsi, got.IMSI)
	}

	if *got.SessionID != sessionID {
		t.Fatalf("expected sessionID %d, got %d", sessionID, *got.SessionID)
	}

	if got.Type != "dynamic" {
		t.Fatalf("expected type dynamic, got %s", got.Type)
	}
}

func TestCreateLease_UniqueConstraint(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	sessionID := 1
	lease := &db.IPLease{
		PoolID:    poolID,
		IMSI:      imsi,
		SessionID: &sessionID,
		Type:      "dynamic",
		CreatedAt: time.Now().Unix(),
	}

	if err := database.CreateLease(ctx, lease, addr("192.168.1.10")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	// Create a second subscriber to attempt a duplicate address.
	imsi2 := "001010123456790"
	sub2 := &db.Subscriber{
		Imsi:           imsi2,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		PolicyID:       1,
	}

	if err := database.CreateSubscriber(ctx, sub2); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	sessionID2 := 2
	dup := &db.IPLease{
		PoolID:    poolID,
		IMSI:      imsi2,
		SessionID: &sessionID2,
		Type:      "dynamic",
		CreatedAt: time.Now().Unix(),
	}

	err := database.CreateLease(ctx, dup, addr("192.168.1.10")) // same address, same pool
	if err != db.ErrAlreadyExists {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestUpdateLeaseSession(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	// Create a dynamic lease.
	sessID := 1
	lease := &db.IPLease{
		PoolID:    poolID,
		IMSI:      imsi,
		SessionID: &sessID,
		Type:      "dynamic",
		CreatedAt: time.Now().Unix(),
	}

	if err := database.CreateLease(ctx, lease, addr("192.168.1.20")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	got, err := database.GetDynamicLease(ctx, poolID, imsi)
	if err != nil {
		t.Fatalf("GetDynamicLease: %s", err)
	}

	// Update session.
	if err := database.UpdateLeaseSession(ctx, got.ID, 100); err != nil {
		t.Fatalf("UpdateLeaseSession: %s", err)
	}

	got2, err := database.GetDynamicLease(ctx, poolID, imsi)
	if err != nil {
		t.Fatalf("GetDynamicLease after update: %s", err)
	}

	if got2.SessionID == nil || *got2.SessionID != 100 {
		t.Fatalf("expected sessionID 100, got %v", got2.SessionID)
	}
}

func TestGetLeaseBySession(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	sessionID := 42
	lease := &db.IPLease{
		PoolID:    poolID,
		IMSI:      imsi,
		SessionID: &sessionID,
		Type:      "dynamic",
		CreatedAt: time.Now().Unix(),
	}

	if err := database.CreateLease(ctx, lease, addr("192.168.1.30")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	got, err := database.GetLeaseBySession(ctx, poolID, sessionID, imsi)
	if err != nil {
		t.Fatalf("GetLeaseBySession: %s", err)
	}

	if got.Address() != addr("192.168.1.30") {
		t.Fatalf("expected address 192.168.1.30, got %s", got.Address())
	}

	// Wrong session should return not found.
	_, err = database.GetLeaseBySession(ctx, poolID, 999, imsi)
	if err != db.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteDynamicLease(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	sessionID := 5
	lease := &db.IPLease{
		PoolID:    poolID,
		IMSI:      imsi,
		SessionID: &sessionID,
		Type:      "dynamic",
		CreatedAt: time.Now().Unix(),
	}

	if err := database.CreateLease(ctx, lease, addr("192.168.1.40")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	got, err := database.GetDynamicLease(ctx, poolID, imsi)
	if err != nil {
		t.Fatalf("GetDynamicLease: %s", err)
	}

	if err := database.DeleteDynamicLease(ctx, got.ID); err != nil {
		t.Fatalf("DeleteDynamicLease: %s", err)
	}

	_, err = database.GetDynamicLease(ctx, poolID, imsi)
	if err != db.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteAllDynamicLeases(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	// Create a second subscriber for a second lease.
	imsi2 := "001010123456790"
	sub2 := &db.Subscriber{
		Imsi:           imsi2,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		PolicyID:       1,
	}

	if err := database.CreateSubscriber(ctx, sub2); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	now := time.Now().Unix()
	sess1 := 10
	sess2 := 11

	// Dynamic lease 1.
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess1, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.10")); err != nil {
		t.Fatalf("CreateLease 1: %s", err)
	}

	// Dynamic lease 2.
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi2,
		SessionID: &sess2, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.11")); err != nil {
		t.Fatalf("CreateLease 2: %s", err)
	}

	count, err := database.CountLeasesByPool(ctx, poolID)
	if err != nil {
		t.Fatalf("CountLeasesByPool: %s", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 leases, got %d", count)
	}

	// Delete all dynamic leases (startup cleanup).
	if err := database.DeleteAllDynamicLeases(ctx); err != nil {
		t.Fatalf("DeleteAllDynamicLeases: %s", err)
	}

	// No leases should remain.
	count, err = database.CountLeasesByPool(ctx, poolID)
	if err != nil {
		t.Fatalf("CountLeasesByPool after cleanup: %s", err)
	}

	if count != 0 {
		t.Fatalf("expected 0 leases after cleanup, got %d", count)
	}
}

func TestListActiveLeases(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	now := time.Now().Unix()
	sess := 20

	// Active dynamic lease.
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.5")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	leases, err := database.ListActiveLeases(ctx)
	if err != nil {
		t.Fatalf("ListActiveLeases: %s", err)
	}

	if len(leases) != 1 {
		t.Fatalf("expected 1 active lease, got %d", len(leases))
	}

	if leases[0].Address() != addr("192.168.1.5") {
		t.Fatalf("expected active lease address 192.168.1.5, got %s", leases[0].Address())
	}
}

func TestListLeasesByPool(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	now := time.Now().Unix()
	sess := 30

	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.1")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	sess2 := 31
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess2, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.2")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	leases, err := database.ListLeasesByPool(ctx, poolID)
	if err != nil {
		t.Fatalf("ListLeasesByPool: %s", err)
	}

	if len(leases) != 2 {
		t.Fatalf("expected 2 leases in pool, got %d", len(leases))
	}
}

func TestListLeaseAddressesByPool(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	now := time.Now().Unix()
	sess := 40

	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.3")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	sess2 := 41
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess2, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.1")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	addrs, err := database.ListLeaseAddressesByPool(ctx, poolID)
	if err != nil {
		t.Fatalf("ListLeaseAddressesByPool: %s", err)
	}

	if len(addrs) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(addrs))
	}

	// Sorted order: 192.168.1.1 < 192.168.1.3.
	if addrs[0] != "192.168.1.1" {
		t.Fatalf("expected first address 192.168.1.1, got %s", addrs[0])
	}

	if addrs[1] != "192.168.1.3" {
		t.Fatalf("expected second address 192.168.1.3, got %s", addrs[1])
	}
}

func TestCountActiveLeases(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	count, err := database.CountActiveLeases(ctx)
	if err != nil {
		t.Fatalf("CountActiveLeases: %s", err)
	}

	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}

	now := time.Now().Unix()
	sess := 50

	// One active lease.
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.7")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	count, err = database.CountActiveLeases(ctx)
	if err != nil {
		t.Fatalf("CountActiveLeases: %s", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 active lease, got %d", count)
	}
}

func TestCountLeasesByPool(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	count, err := database.CountLeasesByPool(ctx, poolID)
	if err != nil {
		t.Fatalf("CountLeasesByPool: %s", err)
	}

	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}

	now := time.Now().Unix()
	sess := 60

	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.9")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	count, err = database.CountLeasesByPool(ctx, poolID)
	if err != nil {
		t.Fatalf("CountLeasesByPool: %s", err)
	}

	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
}

func TestOnDeleteCascade_Subscriber(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	sess := 70
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: time.Now().Unix(),
	}, addr("192.168.1.15")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	// Deleting the subscriber should cascade-delete associated leases.
	err := database.DeleteSubscriber(ctx, imsi)
	if err != nil {
		t.Fatalf("DeleteSubscriber: %s", err)
	}

	count, err := database.CountLeasesByIMSI(ctx, imsi)
	if err != nil {
		t.Fatalf("CountLeasesByIMSI: %s", err)
	}

	if count != 0 {
		t.Fatalf("expected 0 leases after subscriber delete, got %d", count)
	}
}

func TestOnDeleteCascade_DataNetwork(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	sess := 80
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: time.Now().Unix(),
	}, addr("192.168.1.16")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	// Deleting the data network should cascade-delete associated leases.
	err := database.DeleteDataNetwork(ctx, "test-dnn")
	if err != nil {
		t.Fatalf("DeleteDataNetwork: %s", err)
	}

	leases, err := database.ListLeasesByPool(ctx, poolID)
	if err != nil {
		t.Fatalf("ListLeasesByPool: %s", err)
	}

	if len(leases) != 0 {
		t.Fatalf("expected 0 leases after data network delete, got %d", len(leases))
	}
}

func TestCountLeasesByIMSI(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	count, err := database.CountLeasesByIMSI(ctx, imsi)
	if err != nil {
		t.Fatalf("CountLeasesByIMSI: %s", err)
	}

	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}

	now := time.Now().Unix()
	sess := 90

	// Add a dynamic lease.
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.20")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	count, err = database.CountLeasesByIMSI(ctx, imsi)
	if err != nil {
		t.Fatalf("CountLeasesByIMSI: %s", err)
	}

	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
}

func TestListLeasesByPoolPage(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	// Create a second subscriber.
	imsi2 := "001010123456790"
	sub2 := &db.Subscriber{
		Imsi:           imsi2,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		PolicyID:       1,
	}

	if err := database.CreateSubscriber(ctx, sub2); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	now := time.Now().Unix()
	sess1, sess2, sess3 := 1, 2, 3

	// Create 3 leases with addresses that sort as: .1, .10, .2
	// (string sort vs address sort - validates ORDER BY)
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess1, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.10")); err != nil {
		t.Fatalf("CreateLease 1: %s", err)
	}

	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess2, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.2")); err != nil {
		t.Fatalf("CreateLease 2: %s", err)
	}

	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi2,
		SessionID: &sess3, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.1")); err != nil {
		t.Fatalf("CreateLease 3: %s", err)
	}

	t.Run("first page", func(t *testing.T) {
		leases, total, err := database.ListLeasesByPoolPage(ctx, poolID, 1, 2)
		if err != nil {
			t.Fatalf("ListLeasesByPoolPage: %s", err)
		}

		if total != 3 {
			t.Fatalf("expected total 3, got %d", total)
		}

		if len(leases) != 2 {
			t.Fatalf("expected 2 leases on page 1, got %d", len(leases))
		}

		// Verify numeric ordering by address (not lexicographic).
		if leases[0].Address() != addr("192.168.1.1") {
			t.Fatalf("expected first address 192.168.1.1, got %s", leases[0].Address())
		}

		if leases[1].Address() != addr("192.168.1.2") {
			t.Fatalf("expected second address 192.168.1.2, got %s", leases[1].Address())
		}
	})

	t.Run("second page", func(t *testing.T) {
		leases, total, err := database.ListLeasesByPoolPage(ctx, poolID, 2, 2)
		if err != nil {
			t.Fatalf("ListLeasesByPoolPage: %s", err)
		}

		if total != 3 {
			t.Fatalf("expected total 3, got %d", total)
		}

		if len(leases) != 1 {
			t.Fatalf("expected 1 lease on page 2, got %d", len(leases))
		}

		if leases[0].Address() != addr("192.168.1.10") {
			t.Fatalf("expected address 192.168.1.10, got %s", leases[0].Address())
		}
	})

	t.Run("empty pool", func(t *testing.T) {
		leases, total, err := database.ListLeasesByPoolPage(ctx, 9999, 1, 25)
		if err != nil {
			t.Fatalf("ListLeasesByPoolPage: %s", err)
		}

		if total != 0 {
			t.Fatalf("expected total 0, got %d", total)
		}

		if len(leases) != 0 {
			t.Fatalf("expected 0 leases, got %d", len(leases))
		}
	})
}

func TestListLeaseAddressesByPool_NumericOrder(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	// Create additional subscribers for unique (poolID, addressBin, imsi) combos.
	imsi2 := "001010123456790"

	sub2 := &db.Subscriber{
		Imsi:           imsi2,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		PolicyID:       1,
	}

	if err := database.CreateSubscriber(ctx, sub2); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	imsi3 := "001010123456791"

	sub3 := &db.Subscriber{
		Imsi:           imsi3,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		PolicyID:       1,
	}

	if err := database.CreateSubscriber(ctx, sub3); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	now := time.Now().Unix()
	sess1, sess2, sess3 := 1, 2, 3

	// Insert in non-numeric order: .10, .2, .1
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi,
		SessionID: &sess1, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.10")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi2,
		SessionID: &sess2, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.2")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, IMSI: imsi3,
		SessionID: &sess3, Type: "dynamic", CreatedAt: now,
	}, addr("192.168.1.1")); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	addrs, err := database.ListLeaseAddressesByPool(ctx, poolID)
	if err != nil {
		t.Fatalf("ListLeaseAddressesByPool: %s", err)
	}

	// Expect numeric sort: .1, .2, .10 (not lexicographic .1, .10, .2).
	expected := []string{"192.168.1.1", "192.168.1.2", "192.168.1.10"}

	if len(addrs) != len(expected) {
		t.Fatalf("expected %d addresses, got %d", len(expected), len(addrs))
	}

	for i, want := range expected {
		if addrs[i] != want {
			t.Fatalf("address[%d]: expected %s, got %s", i, want, addrs[i])
		}
	}
}
