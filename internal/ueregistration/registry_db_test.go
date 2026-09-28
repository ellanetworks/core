// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ueregistration_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/ellanetworks/core/internal/ueregistration"
	"go.uber.org/zap"
)

func newRaftTestDatabase(t *testing.T) *db.Database {
	t.Helper()

	ctx := context.Background()

	database, err := db.NewDatabase(ctx, filepath.Join(t.TempDir(), "db.sqlite3"), ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("NewDatabase: %s", err)
	}

	if err := database.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("WaitUntilReady: %s", err)
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

func bind(t *testing.T, database *db.Database, nodeID, regType, otherType string) *ueregistration.Binding {
	t.Helper()

	return ueregistration.New(database, nodeID, zap.NewNop()).Bind(regType, otherType, newFakeHolder())
}

func register(t *testing.T, b *ueregistration.Binding) int64 {
	t.Helper()

	version, err := b.Register(context.Background(), imsi)
	if err != nil {
		t.Fatalf("Register: %s", err)
	}

	return version
}

func superseded(b *ueregistration.Binding, held int64) bool {
	released := false

	b.Reconcile(context.Background(), imsi, func() int64 { return held }, func(context.Context) bool { released = true; return true })

	return released
}

func TestRegistry_UEMovesBetweenNodes(t *testing.T) {
	database := newRaftTestDatabase(t)

	amfA := bind(t, database, "node-a", amfType, mmeType)
	amfB := bind(t, database, "node-b", amfType, mmeType)

	atA := register(t, amfA)
	atB := register(t, amfB)

	if !superseded(amfA, atA) {
		t.Fatal("expected node-a's context to be superseded by node-b")
	}

	if superseded(amfB, atB) {
		t.Fatal("expected node-b's context to be kept")
	}

	if again := register(t, amfA); !superseded(amfB, atB) || superseded(amfA, again) {
		t.Fatal("expected the UE's return to node-a to supersede node-b")
	}
}

func TestRegistry_EPSAttachOnAnotherNodeCancels5GS(t *testing.T) {
	database := newRaftTestDatabase(t)

	amfA := bind(t, database, "node-a", amfType, mmeType)
	mmeA := bind(t, database, "node-a", mmeType, amfType)
	mmeB := bind(t, database, "node-b", mmeType, amfType)

	atA := register(t, amfA)

	register(t, mmeA)

	if superseded(amfA, atA) {
		t.Fatal("an EPS registration on the same node is left to the in-process interworking")
	}

	register(t, mmeB)

	if !superseded(amfA, atA) {
		t.Fatal("expected node-a's 5GS context to be superseded by node-b's EPS registration")
	}
}

func TestRegistry_OlderRegistrationElsewhereDoesNotSupersede(t *testing.T) {
	database := newRaftTestDatabase(t)

	amfA := bind(t, database, "node-a", amfType, mmeType)
	amfB := bind(t, database, "node-b", amfType, mmeType)

	atB := register(t, amfB)

	if superseded(amfA, atB+1000) {
		t.Fatal("a registration older than the local context, as after a restore, must not supersede it")
	}
}

func TestRegistry_PurgedRegistrationElsewhereDoesNotSupersede(t *testing.T) {
	database := newRaftTestDatabase(t)
	ctx := context.Background()

	amfA := bind(t, database, "node-a", amfType, mmeType)
	amfB := bind(t, database, "node-b", amfType, mmeType)

	atA := register(t, amfA)
	register(t, amfB)

	if err := database.PurgeUERegistration(ctx, imsi, amfType, "node-b"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	if superseded(amfA, atA) {
		t.Fatal("a purged registration on another node must not supersede a live context")
	}
}
