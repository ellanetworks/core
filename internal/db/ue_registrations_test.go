// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

const ueRegIMSI = "001010000000001"

func setupUERegistrationsTestDB(t *testing.T) *db.Database {
	t.Helper()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %s", err)
		}
	})

	createSubscriber(t, database, ueRegIMSI)

	return database
}

func mustGetUERegistration(t *testing.T, database *db.Database, regType string) *db.UERegistration {
	t.Helper()

	reg, err := database.GetUERegistration(context.Background(), ueRegIMSI, regType)
	if err != nil {
		t.Fatalf("GetUERegistration(%s): %s", regType, err)
	}

	return reg
}

func assertUERegistration(t *testing.T, database *db.Database, regType, nodeID string, purged bool) {
	t.Helper()

	reg := mustGetUERegistration(t, database, regType)
	if reg.NodeID != nodeID || reg.Purged != purged {
		t.Fatalf("%s registration = {node %q, purged %v}, want {node %q, purged %v}", regType, reg.NodeID, reg.Purged, nodeID, purged)
	}
}

func TestUERegistration_NotFound(t *testing.T) {
	database := setupUERegistrationsTestDB(t)

	_, err := database.GetUERegistration(context.Background(), ueRegIMSI, db.UERegistrationTypeMME)
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRegisterUE_CreatesRegistration(t *testing.T) {
	database := setupUERegistrationsTestDB(t)
	ctx := context.Background()

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeMME, "node-a"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	reg := mustGetUERegistration(t, database, db.UERegistrationTypeMME)
	if reg.NodeID != "node-a" || reg.Purged || reg.RegistrationTime == 0 {
		t.Fatalf("unexpected registration %+v", reg)
	}

	if _, err := database.GetUERegistration(ctx, ueRegIMSI, db.UERegistrationTypeAMF3GPPAccess); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("expected no AMF registration, got %v", err)
	}
}

func TestRegisterUE_OtherNodeReplaces(t *testing.T) {
	database := setupUERegistrationsTestDB(t)
	ctx := context.Background()

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeAMF3GPPAccess, "node-a"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeAMF3GPPAccess, "node-b"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	assertUERegistration(t, database, db.UERegistrationTypeAMF3GPPAccess, "node-b", false)
}

func TestRegisterUE_SameNodeKeepsRegistrationTime(t *testing.T) {
	database := setupUERegistrationsTestDB(t)
	ctx := context.Background()

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeMME, "node-a"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	before := mustGetUERegistration(t, database, db.UERegistrationTypeMME)

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeMME, "node-a"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	after := mustGetUERegistration(t, database, db.UERegistrationTypeMME)
	if *after != *before {
		t.Fatalf("registration changed: before %+v, after %+v", before, after)
	}
}

func TestRegisterUE_ClearsPurged(t *testing.T) {
	database := setupUERegistrationsTestDB(t)
	ctx := context.Background()

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeMME, "node-a"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if err := database.PurgeUERegistration(ctx, ueRegIMSI, db.UERegistrationTypeMME, "node-a"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	assertUERegistration(t, database, db.UERegistrationTypeMME, "node-a", true)

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeMME, "node-a"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	assertUERegistration(t, database, db.UERegistrationTypeMME, "node-a", false)
}

func TestPurgeUERegistration_OnlyByRegisteredNode(t *testing.T) {
	database := setupUERegistrationsTestDB(t)
	ctx := context.Background()

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeMME, "node-b"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if err := database.PurgeUERegistration(ctx, ueRegIMSI, db.UERegistrationTypeMME, "node-a"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	assertUERegistration(t, database, db.UERegistrationTypeMME, "node-b", false)
}

func TestPurgeUERegistration_NoRegistration(t *testing.T) {
	database := setupUERegistrationsTestDB(t)
	ctx := context.Background()

	if err := database.PurgeUERegistration(ctx, ueRegIMSI, db.UERegistrationTypeMME, "node-a"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	if _, err := database.GetUERegistration(ctx, ueRegIMSI, db.UERegistrationTypeMME); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRegisterUE_CancelsOtherTypeOnAnyNode(t *testing.T) {
	database := setupUERegistrationsTestDB(t)
	ctx := context.Background()

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeAMF3GPPAccess, "node-a"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeMME, "node-b", db.UERegistrationTypeAMF3GPPAccess); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	assertUERegistration(t, database, db.UERegistrationTypeMME, "node-b", false)
	assertUERegistration(t, database, db.UERegistrationTypeAMF3GPPAccess, "node-a", true)
}

func TestRegisterUE_CancelWithoutOtherRegistration(t *testing.T) {
	database := setupUERegistrationsTestDB(t)
	ctx := context.Background()

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeMME, "node-a", db.UERegistrationTypeAMF3GPPAccess); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if _, err := database.GetUERegistration(ctx, ueRegIMSI, db.UERegistrationTypeAMF3GPPAccess); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRegisterUE_InvalidInput(t *testing.T) {
	database := setupUERegistrationsTestDB(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		regType string
		nodeID  string
		cancel  []string
	}{
		{"unknown type", "smsf-3gpp-access", "node-a", nil},
		{"empty node", db.UERegistrationTypeMME, "", nil},
		{"cancel own type", db.UERegistrationTypeMME, "node-a", []string{db.UERegistrationTypeMME}},
		{"cancel unknown type", db.UERegistrationTypeMME, "node-a", []string{"sgsn"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := database.RegisterUE(ctx, ueRegIMSI, tc.regType, tc.nodeID, tc.cancel...); err == nil {
				t.Fatal("expected error")
			}
		})
	}

	if err := database.PurgeUERegistration(ctx, ueRegIMSI, "sgsn", "node-a"); err == nil {
		t.Fatal("expected error purging unknown type")
	}
}

func TestRegisterUE_UnknownSubscriber(t *testing.T) {
	database := setupUERegistrationsTestDB(t)

	err := database.RegisterUE(context.Background(), "001019999999999", db.UERegistrationTypeMME, "node-a")
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteSubscriber_DeletesUERegistrations(t *testing.T) {
	database := setupUERegistrationsTestDB(t)
	ctx := context.Background()

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeMME, "node-a"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeAMF3GPPAccess, "node-a"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if err := database.DeleteSubscriber(ctx, ueRegIMSI); err != nil {
		t.Fatalf("DeleteSubscriber: %s", err)
	}

	for _, regType := range []string{db.UERegistrationTypeMME, db.UERegistrationTypeAMF3GPPAccess} {
		if _, err := database.GetUERegistration(ctx, ueRegIMSI, regType); !errors.Is(err, db.ErrNotFound) {
			t.Fatalf("%s: expected ErrNotFound after subscriber deletion, got %v", regType, err)
		}
	}
}

func TestDeleteClusterMember_PurgesItsUERegistrations(t *testing.T) {
	database := setupUERegistrationsTestDB(t)
	ctx := context.Background()

	const otherIMSI = "001010000000002"

	createSubscriber(t, database, otherIMSI)

	for _, nodeID := range []string{"1", "2"} {
		if err := database.UpsertClusterMember(ctx, &db.ClusterMember{NodeID: nodeID, APIAddress: "10.0.0." + nodeID + ":8443"}); err != nil {
			t.Fatalf("UpsertClusterMember: %s", err)
		}
	}

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeMME, "1"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if err := database.RegisterUE(ctx, ueRegIMSI, db.UERegistrationTypeAMF3GPPAccess, "1"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if err := database.RegisterUE(ctx, otherIMSI, db.UERegistrationTypeMME, "2"); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if err := database.DeleteClusterMember(ctx, "1"); err != nil {
		t.Fatalf("DeleteClusterMember: %s", err)
	}

	assertUERegistration(t, database, db.UERegistrationTypeMME, "1", true)
	assertUERegistration(t, database, db.UERegistrationTypeAMF3GPPAccess, "1", true)

	other, err := database.GetUERegistration(ctx, otherIMSI, db.UERegistrationTypeMME)
	if err != nil {
		t.Fatalf("GetUERegistration: %s", err)
	}

	if other.NodeID != "2" || other.Purged {
		t.Fatalf("other node's registration changed: %+v", other)
	}
}
