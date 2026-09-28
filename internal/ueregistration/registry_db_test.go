// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ueregistration_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/ueregistration"
	"go.uber.org/zap"
)

func newTestDatabase(t *testing.T) *db.Database {
	t.Helper()

	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %s", err)
		}
	})

	profile := &db.Profile{Name: "profile", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}
	if err := database.CreateProfile(ctx, profile); err != nil {
		t.Fatalf("CreateProfile: %s", err)
	}

	created, err := database.GetProfile(ctx, profile.Name)
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	sub := &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      created.ID,
	}
	if err := database.CreateSubscriber(ctx, sub); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	return database
}

func registerAt(t *testing.T, r *ueregistration.Registry) int64 {
	t.Helper()

	time.Sleep(2 * time.Millisecond)

	start := time.Now().UnixMilli()

	if err := r.Register(context.Background(), imsi); err != nil {
		t.Fatalf("Register: %s", err)
	}

	time.Sleep(2 * time.Millisecond)

	return start
}

func TestRegistry_UEMovesBetweenNodes(t *testing.T) {
	database := newTestDatabase(t)
	ctx := context.Background()

	amfA := ueregistration.New(database, "node-a", db.UERegistrationTypeAMF3GPPAccess, db.UERegistrationTypeMME, zap.NewNop())
	amfB := ueregistration.New(database, "node-b", db.UERegistrationTypeAMF3GPPAccess, db.UERegistrationTypeMME, zap.NewNop())

	atA := registerAt(t, amfA)
	atB := registerAt(t, amfB)

	if !amfA.Superseded(ctx, imsi, atA) {
		t.Fatal("expected node-a's context to be superseded by node-b")
	}

	if amfB.Superseded(ctx, imsi, atB) {
		t.Fatal("expected node-b's context to be kept")
	}

	amfA.Purge(ctx, imsi, func() bool { return true })

	reg, err := database.GetUERegistration(ctx, imsi, db.UERegistrationTypeAMF3GPPAccess)
	if err != nil {
		t.Fatalf("GetUERegistration: %s", err)
	}

	if reg.NodeID != "node-b" || reg.Purged {
		t.Fatalf("node-a's purge changed node-b's registration: %+v", reg)
	}
}

func TestRegistry_EPSAttachOnAnotherNodeCancels5GS(t *testing.T) {
	database := newTestDatabase(t)
	ctx := context.Background()

	amfA := ueregistration.New(database, "node-a", db.UERegistrationTypeAMF3GPPAccess, db.UERegistrationTypeMME, zap.NewNop())
	mmeA := ueregistration.New(database, "node-a", db.UERegistrationTypeMME, db.UERegistrationTypeAMF3GPPAccess, zap.NewNop())
	mmeB := ueregistration.New(database, "node-b", db.UERegistrationTypeMME, db.UERegistrationTypeAMF3GPPAccess, zap.NewNop())

	atA := registerAt(t, amfA)

	registerAt(t, mmeA)

	if amfA.Superseded(ctx, imsi, atA) {
		t.Fatal("an EPS registration on the same node is left to the in-process interworking")
	}

	registerAt(t, mmeB)

	if !amfA.Superseded(ctx, imsi, atA) {
		t.Fatal("expected node-a's 5GS context to be superseded by node-b's EPS registration")
	}
}

func TestRegistry_RestoredRegistrationDoesNotSupersedeNewerContext(t *testing.T) {
	database := newTestDatabase(t)
	ctx := context.Background()

	amfA := ueregistration.New(database, "node-a", db.UERegistrationTypeAMF3GPPAccess, db.UERegistrationTypeMME, zap.NewNop())
	amfB := ueregistration.New(database, "node-b", db.UERegistrationTypeAMF3GPPAccess, db.UERegistrationTypeMME, zap.NewNop())

	registerAt(t, amfB)

	atA := time.Now().UnixMilli() + 1000

	if amfA.Superseded(ctx, imsi, atA) {
		t.Fatal("an older registration on another node must not supersede a newer local context")
	}
}
