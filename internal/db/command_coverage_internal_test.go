// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func TestDeleteSMSWaitingRemovesTheRecord(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	ctx := t.Context()

	createTestSubscriber(t, database, determinismIMSI)

	if err := database.RecordSMSWaiting(ctx, SMSWaitingUpdate{IMSI: determinismIMSI, ServiceCentre: "15550001", MemoryFull: true}); err != nil {
		t.Fatalf("RecordSMSWaiting: %v", err)
	}

	if _, err := database.GetSMSWaiting(ctx, determinismIMSI); err != nil {
		t.Fatalf("GetSMSWaiting before delete: %v", err)
	}

	if err := database.DeleteSMSWaiting(ctx, determinismIMSI); err != nil {
		t.Fatalf("DeleteSMSWaiting: %v", err)
	}

	if _, err := database.GetSMSWaiting(ctx, determinismIMSI); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSMSWaiting after delete: err = %v, want ErrNotFound", err)
	}
}

func TestReplacePCSCFAddressesAppliesAsACommand(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	ctx := t.Context()

	payload, err := json.Marshal(&pcscfAddressesPayload{Addresses: []string{"10.0.0.21", "10.0.0.22"}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if _, err := database.proposeCommand(ctx, "ReplacePCSCFAddresses", payload); err != nil {
		t.Fatalf("ReplacePCSCFAddresses: %v", err)
	}

	got, err := database.ListPCSCFAddresses(ctx)
	if err != nil {
		t.Fatalf("ListPCSCFAddresses: %v", err)
	}

	slices.Sort(got)

	if want := []string{"10.0.0.21", "10.0.0.22"}; !slices.Equal(got, want) {
		t.Fatalf("P-CSCF addresses = %v, want %v", got, want)
	}
}

func TestLegacySMSCPeerCommandsAreRejectedWithoutHalting(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	peer := &legacySMSCPeer{ID: "legacy", DiameterIdentity: "legacy.example.org", Address: "10.0.0.10", Port: 3868, ServiceCentres: []string{"15550005"}}

	index := nextIndex(t, database)

	results := applyOne(t, database, index, database.appliedLeaderTime.Load()+1,
		command(t, "CreateSMSCPeer", peer),
		command(t, "UpdateSMSCPeer", peer),
		command(t, "DeleteSMSCPeer", &stringPayload{Value: "legacy"}),
		command(t, "SetRetentionPolicy", &RetentionPolicy{Category: CategoryAuditLogs, Days: 3}),
	)

	for i, op := range []string{"CreateSMSCPeer", "UpdateSMSCPeer", "DeleteSMSCPeer"} {
		if !errors.Is(results[i].err, ErrRetiredOperation) {
			t.Fatalf("%s: err = %v, want ErrRetiredOperation", op, results[i].err)
		}

		if code := ForwardCodeFor(results[i].err); code != ellaraft.ForwardCodeRetiredOp {
			t.Fatalf("%s: forward code = %q, want %q", op, code, ellaraft.ForwardCodeRetiredOp)
		}
	}

	if results[3].err != nil {
		t.Fatalf("a command after the rejected ones failed: %v", results[3].err)
	}

	if got := fsmState(t, database).lastApplied; got != index {
		t.Fatalf("lastApplied = %d, want %d", got, index)
	}
}

func TestMigrateSharedStillAppliesAfterTheCommandSchema(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := t.Context()

	if !database.commandMode(ctx) {
		t.Fatal("a fresh standalone database is not in command mode")
	}

	if err := database.SetRetentionPolicy(ctx, &RetentionPolicy{Category: CategoryAuditLogs, Days: 4}); err != nil {
		t.Fatalf("SetRetentionPolicy before MigrateShared: %v", err)
	}

	before := fsmState(t, database)

	if _, err := opMigrateShared.Invoke(ctx, database, &migrateSharedPayload{TargetVersion: SchemaVersion()}); err != nil {
		t.Fatalf("MigrateShared(%d) after the cut-over: %v", SchemaVersion(), err)
	}

	middle := fsmState(t, database)
	if middle.lastApplied <= before.lastApplied || middle.leaderTime != before.leaderTime {
		t.Fatalf("fsm_state %+v -> %+v: MigrateShared must apply as its own legacy entry", before, middle)
	}

	schema, err := readSchemaVersion(ctx, database.writer())
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	if schema != SchemaVersion() {
		t.Fatalf("schema = %d after a replayed MigrateShared, want %d", schema, SchemaVersion())
	}

	if err := database.SetRetentionPolicy(ctx, &RetentionPolicy{Category: CategoryAuditLogs, Days: 5}); err != nil {
		t.Fatalf("SetRetentionPolicy after MigrateShared: %v", err)
	}

	if after := fsmState(t, database); after.leaderTime <= middle.leaderTime || after.lastApplied <= middle.lastApplied {
		t.Fatalf("fsm_state %+v -> %+v: a write after MigrateShared must still be a command", middle, after)
	}

	policy, err := database.GetRetentionPolicy(ctx, CategoryAuditLogs)
	if err != nil {
		t.Fatalf("GetRetentionPolicy: %v", err)
	}

	if policy != 5 {
		t.Fatalf("retention = %d days, want 5", policy)
	}
}
