// Copyright 2026 Ella Networks

package db_test

import (
	"context"
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

func TestCreateAndGetLease(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	sessionID := 1
	lease := &db.IPLease{
		PoolID:    poolID,
		Address:   "192.168.1.10",
		IMSI:      imsi,
		SessionID: &sessionID,
		Type:      "dynamic",
		CreatedAt: time.Now().Unix(),
	}

	if err := database.CreateLease(ctx, lease); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	// Should be retrievable as a dynamic lease.
	got, err := database.GetDynamicLease(ctx, poolID, imsi)
	if err != nil {
		t.Fatalf("GetDynamicLease: %s", err)
	}

	if got.Address != lease.Address {
		t.Fatalf("expected address %s, got %s", lease.Address, got.Address)
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

	// Static lease should not be found.
	_, err = database.GetStaticLease(ctx, poolID, imsi)
	if err != db.ErrNotFound {
		t.Fatalf("expected ErrNotFound for static lease, got %v", err)
	}
}

func TestCreateLease_UniqueConstraint(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	sessionID := 1
	lease := &db.IPLease{
		PoolID:    poolID,
		Address:   "192.168.1.10",
		IMSI:      imsi,
		SessionID: &sessionID,
		Type:      "dynamic",
		CreatedAt: time.Now().Unix(),
	}

	if err := database.CreateLease(ctx, lease); err != nil {
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
		Address:   "192.168.1.10", // same address, same pool
		IMSI:      imsi2,
		SessionID: &sessionID2,
		Type:      "dynamic",
		CreatedAt: time.Now().Unix(),
	}

	err := database.CreateLease(ctx, dup)
	if err != db.ErrAlreadyExists {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestStaticLease(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	// Create a static reservation (no sessionID).
	lease := &db.IPLease{
		PoolID:    poolID,
		Address:   "192.168.1.50",
		IMSI:      imsi,
		SessionID: nil,
		Type:      "static",
		CreatedAt: time.Now().Unix(),
	}

	if err := database.CreateLease(ctx, lease); err != nil {
		t.Fatalf("CreateLease (static): %s", err)
	}

	// Should be retrievable as a static lease.
	got, err := database.GetStaticLease(ctx, poolID, imsi)
	if err != nil {
		t.Fatalf("GetStaticLease: %s", err)
	}

	if got.Type != "static" {
		t.Fatalf("expected type static, got %s", got.Type)
	}

	if got.SessionID != nil {
		t.Fatalf("expected nil sessionID for static reservation, got %v", *got.SessionID)
	}

	// Dynamic lease should not be found.
	_, err = database.GetDynamicLease(ctx, poolID, imsi)
	if err != db.ErrNotFound {
		t.Fatalf("expected ErrNotFound for dynamic lease, got %v", err)
	}
}

func TestUpdateLeaseSession(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	// Create a static lease with no session.
	lease := &db.IPLease{
		PoolID:    poolID,
		Address:   "192.168.1.20",
		IMSI:      imsi,
		SessionID: nil,
		Type:      "static",
		CreatedAt: time.Now().Unix(),
	}

	if err := database.CreateLease(ctx, lease); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	got, err := database.GetStaticLease(ctx, poolID, imsi)
	if err != nil {
		t.Fatalf("GetStaticLease: %s", err)
	}

	// Attach a session.
	if err := database.UpdateLeaseSession(ctx, got.ID, 100); err != nil {
		t.Fatalf("UpdateLeaseSession: %s", err)
	}

	got2, err := database.GetStaticLease(ctx, poolID, imsi)
	if err != nil {
		t.Fatalf("GetStaticLease after update: %s", err)
	}

	if got2.SessionID == nil || *got2.SessionID != 100 {
		t.Fatalf("expected sessionID 100, got %v", got2.SessionID)
	}

	// Clear session.
	if err := database.ClearLeaseSession(ctx, got.ID); err != nil {
		t.Fatalf("ClearLeaseSession: %s", err)
	}

	got3, err := database.GetStaticLease(ctx, poolID, imsi)
	if err != nil {
		t.Fatalf("GetStaticLease after clear: %s", err)
	}

	if got3.SessionID != nil {
		t.Fatalf("expected nil sessionID after clear, got %v", *got3.SessionID)
	}
}

func TestGetLeaseBySession(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	sessionID := 42
	lease := &db.IPLease{
		PoolID:    poolID,
		Address:   "192.168.1.30",
		IMSI:      imsi,
		SessionID: &sessionID,
		Type:      "dynamic",
		CreatedAt: time.Now().Unix(),
	}

	if err := database.CreateLease(ctx, lease); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	got, err := database.GetLeaseBySession(ctx, poolID, sessionID, imsi)
	if err != nil {
		t.Fatalf("GetLeaseBySession: %s", err)
	}

	if got.Address != "192.168.1.30" {
		t.Fatalf("expected address 192.168.1.30, got %s", got.Address)
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
		Address:   "192.168.1.40",
		IMSI:      imsi,
		SessionID: &sessionID,
		Type:      "dynamic",
		CreatedAt: time.Now().Unix(),
	}

	if err := database.CreateLease(ctx, lease); err != nil {
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

func TestDeleteDynamicLease_DoesNotAffectStatic(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	// Create a static lease.
	staticLease := &db.IPLease{
		PoolID:    poolID,
		Address:   "192.168.1.60",
		IMSI:      imsi,
		SessionID: nil,
		Type:      "static",
		CreatedAt: time.Now().Unix(),
	}

	if err := database.CreateLease(ctx, staticLease); err != nil {
		t.Fatalf("CreateLease (static): %s", err)
	}

	got, err := database.GetStaticLease(ctx, poolID, imsi)
	if err != nil {
		t.Fatalf("GetStaticLease: %s", err)
	}

	// DeleteDynamicLease should be a no-op for static leases.
	if err := database.DeleteDynamicLease(ctx, got.ID); err != nil {
		t.Fatalf("DeleteDynamicLease: %s", err)
	}

	// Static lease should still exist.
	_, err = database.GetStaticLease(ctx, poolID, imsi)
	if err != nil {
		t.Fatalf("static lease should survive DeleteDynamicLease, got %v", err)
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
		PoolID: poolID, Address: "192.168.1.10", IMSI: imsi,
		SessionID: &sess1, Type: "dynamic", CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateLease 1: %s", err)
	}

	// Dynamic lease 2.
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, Address: "192.168.1.11", IMSI: imsi2,
		SessionID: &sess2, Type: "dynamic", CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateLease 2: %s", err)
	}

	// Static lease.
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, Address: "192.168.1.200", IMSI: imsi,
		SessionID: nil, Type: "static", CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateLease static: %s", err)
	}

	count, err := database.CountLeasesByPool(ctx, poolID)
	if err != nil {
		t.Fatalf("CountLeasesByPool: %s", err)
	}

	if count != 3 {
		t.Fatalf("expected 3 leases, got %d", count)
	}

	// Delete all dynamic leases (startup cleanup).
	if err := database.DeleteAllDynamicLeases(ctx); err != nil {
		t.Fatalf("DeleteAllDynamicLeases: %s", err)
	}

	// Only the static lease should remain.
	count, err = database.CountLeasesByPool(ctx, poolID)
	if err != nil {
		t.Fatalf("CountLeasesByPool after cleanup: %s", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 lease (static) after cleanup, got %d", count)
	}

	got, err := database.GetStaticLease(ctx, poolID, imsi)
	if err != nil {
		t.Fatalf("GetStaticLease after cleanup: %s", err)
	}

	if got.Address != "192.168.1.200" {
		t.Fatalf("expected static address 192.168.1.200, got %s", got.Address)
	}
}

func TestListActiveLeases(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	now := time.Now().Unix()
	sess := 20

	// Active dynamic lease.
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, Address: "192.168.1.5", IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	// Inactive static reservation (no session).
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, Address: "192.168.1.100", IMSI: imsi,
		SessionID: nil, Type: "static", CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateLease static: %s", err)
	}

	leases, err := database.ListActiveLeases(ctx)
	if err != nil {
		t.Fatalf("ListActiveLeases: %s", err)
	}

	if len(leases) != 1 {
		t.Fatalf("expected 1 active lease, got %d", len(leases))
	}

	if leases[0].Address != "192.168.1.5" {
		t.Fatalf("expected active lease address 192.168.1.5, got %s", leases[0].Address)
	}
}

func TestListLeasesByPool(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	now := time.Now().Unix()
	sess := 30

	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, Address: "192.168.1.1", IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, Address: "192.168.1.2", IMSI: imsi,
		SessionID: nil, Type: "static", CreatedAt: now,
	}); err != nil {
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
		PoolID: poolID, Address: "192.168.1.3", IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, Address: "192.168.1.1", IMSI: imsi,
		SessionID: nil, Type: "static", CreatedAt: now,
	}); err != nil {
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
		PoolID: poolID, Address: "192.168.1.7", IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	// One inactive static reservation.
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, Address: "192.168.1.8", IMSI: imsi,
		SessionID: nil, Type: "static", CreatedAt: now,
	}); err != nil {
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
		PoolID: poolID, Address: "192.168.1.9", IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: now,
	}); err != nil {
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

func TestOnDeleteRestrict_Subscriber(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	sess := 70
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, Address: "192.168.1.15", IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	// Deleting the subscriber should fail because of ON DELETE RESTRICT.
	err := database.DeleteSubscriber(ctx, imsi)
	if err == nil {
		t.Fatal("expected error deleting subscriber with active lease, got nil")
	}
}

func TestOnDeleteRestrict_DataNetwork(t *testing.T) {
	database, poolID, imsi := setupLeaseTestDB(t)
	ctx := context.Background()

	sess := 80
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, Address: "192.168.1.16", IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	// Deleting the data network should fail because of ON DELETE RESTRICT
	// on ip_leases.poolID → data_networks(id).
	err := database.DeleteDataNetwork(ctx, "test-dnn")
	if err == nil {
		t.Fatal("expected error deleting data network with active lease, got nil")
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
		PoolID: poolID, Address: "192.168.1.20", IMSI: imsi,
		SessionID: &sess, Type: "dynamic", CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	// Add a static lease.
	if err := database.CreateLease(ctx, &db.IPLease{
		PoolID: poolID, Address: "192.168.1.21", IMSI: imsi,
		SessionID: nil, Type: "static", CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateLease: %s", err)
	}

	count, err = database.CountLeasesByIMSI(ctx, imsi)
	if err != nil {
		t.Fatalf("CountLeasesByIMSI: %s", err)
	}

	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}
}
