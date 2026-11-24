// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"maps"
	"testing"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
)

func stateHashOf(t *testing.T, database *Database) string {
	t.Helper()

	sum, err := hashReplicatedState(t.Context(), database.reader())
	if err != nil {
		t.Fatalf("hashReplicatedState: %v", err)
	}

	return sum
}

func execSQL(t *testing.T, database *Database, stmt string) {
	t.Helper()

	if _, err := database.writer().ExecContext(t.Context(), stmt); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}

func TestStateHashIgnoresPhysicalRowOrderAndRowids(t *testing.T) {
	original := newInitializedCommandTestDB(t)
	seedDeterminismFixture(t, original)

	copied := copyDatabase(t, original)

	for _, table := range []string{IPLeasesTableName, SessionsTableName, SubscribersTableName, DataNetworksTableName} {
		reversePhysicalOrder(t, copied, table)
	}

	execSQL(t, copied, "UPDATE ip_leases SET rowid = rowid + 1000")

	if got, want := stateHashOf(t, copied), stateHashOf(t, original); got != want {
		t.Fatalf("hash changed with physical order or rowids: %s != %s", got, want)
	}
}

func TestStateHashIgnoresLocalOnlyTables(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	before := stateHashOf(t, database)

	execSQL(t, database, "INSERT INTO audit_logs (id, timestamp, level, action) VALUES ('a', 1000, 'info', 'test')")
	execSQL(t, database, "UPDATE fsm_state SET lastApplied = lastApplied + 7 WHERE id = 1")

	if got := stateHashOf(t, database); got != before {
		t.Fatalf("hash changed after local-only writes: %s != %s", got, before)
	}
}

func TestStateHashDetectsAnyColumnChange(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	seedDeterminismFixture(t, database)

	base := stateHashOf(t, database)

	for _, tc := range []struct {
		name   string
		change string
		undo   string
	}{
		{"integer value", "UPDATE data_networks SET mtu = mtu + 1", "UPDATE data_networks SET mtu = mtu - 1"},
		{"same bytes as text", "UPDATE ip_leases SET addressBin = CAST(addressBin AS TEXT) WHERE sessionID = 1", "UPDATE ip_leases SET addressBin = CAST(addressBin AS BLOB) WHERE typeof(addressBin) = 'text'"},
		{"text value", "UPDATE subscribers SET description = 'x' WHERE imsi = '" + determinismIMSI + "'", "UPDATE subscribers SET description = '' WHERE imsi = '" + determinismIMSI + "'"},
		{"null versus value", "UPDATE ip_leases SET sessionID = NULL WHERE sessionID = 1", "UPDATE ip_leases SET sessionID = 1 WHERE sessionID IS NULL AND imsi = '" + determinismIMSI + "' AND type = 'dynamic'"},
		{"blob byte", "UPDATE sessions SET token_hash = x'ff' WHERE token_hash = x'00'", "UPDATE sessions SET token_hash = x'00' WHERE token_hash = x'ff'"},
		{"extra row", "INSERT INTO retention_policies (id, category, retention_days) VALUES ('probe', 'probe', 1)", "DELETE FROM retention_policies WHERE category = 'probe'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			execSQL(t, database, tc.change)

			if got := stateHashOf(t, database); got == base {
				t.Fatalf("hash did not change after %q", tc.change)
			}

			execSQL(t, database, tc.undo)

			if got := stateHashOf(t, database); got != base {
				t.Fatalf("hash did not return to the base value after %q", tc.undo)
			}
		})
	}
}

func waitForStateHash(t *testing.T, database *Database, index uint64) ellaraft.StateHash {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		if got := database.StateHash(); got.Index >= index {
			return got
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("no state hash at index %d", index)

	return ellaraft.StateHash{}
}

func TestVerifyStateHashesTheStateAtItsIndex(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	reference := copyDatabase(t, database)

	index := nextIndex(t, database)
	verify := entry(t, index, 1, command(t, verifyStateOp, struct{}{}))
	write := entry(t, index+1, 2, command(t, "CreateProfile", &Profile{ID: newUUID(t), Name: "after-verify", UeAmbrUplink: "1 Gbps", UeAmbrDownlink: "1 Gbps"}))

	out, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{verify, write})
	if err != nil {
		t.Fatalf("ApplyEntries: %v", err)
	}

	if got := resultsOf(t, out, 0)[0]; got.err != nil || got.value != index {
		t.Fatalf("VerifyState result = %v, %v; want %d", got.value, got.err, index)
	}

	if got := resultsOf(t, out, 1)[0]; got.err != nil {
		t.Fatalf("CreateProfile: %v", got.err)
	}

	if _, err := reference.ApplyEntries(t.Context(), []ellaraft.Entry{verify}); err != nil {
		t.Fatalf("ApplyEntries on reference: %v", err)
	}

	got := waitForStateHash(t, database, index)
	if got.Index != index {
		t.Fatalf("hash index = %d, want %d", got.Index, index)
	}

	if want := stateHashOf(t, reference); got.Hash != want {
		t.Fatalf("hash at %d = %s, want the state before the following entry %s", index, got.Hash, want)
	}

	if got.Hash == stateHashOf(t, database) {
		t.Fatal("captured hash includes the entry applied after VerifyState")
	}
}

func TestVerifyStateReturnsItsIndexOnTheLocalPath(t *testing.T) {
	database := newInitializedCommandTestDB(t)

	index, err := opVerifyState.Invoke(t.Context(), database, &struct{}{})
	if err != nil {
		t.Fatalf("VerifyState: %v", err)
	}

	got := waitForStateHash(t, database, index)
	if got.Index != index || got.Hash != stateHashOf(t, database) {
		t.Fatalf("state hash = %+v, want index %d and the current hash", got, index)
	}
}

func TestVerifyStateRequiresTheCommandSchema(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	useChangesetPath(t, database)

	if _, err := opVerifyState.Invoke(t.Context(), database, &struct{}{}); err == nil {
		t.Fatal("VerifyState was accepted below the command schema")
	}
}

func TestDivergedNodesAreUnhealthyInTheAutopilotState(t *testing.T) {
	healthy := func(id string) *autopilot.ServerState {
		return &autopilot.ServerState{Server: autopilot.Server{ID: raft.ServerID(id)}, Health: autopilot.ServerHealth{Healthy: true}}
	}

	state := &autopilot.State{
		Healthy:          true,
		FailureTolerance: 1,
		Voters:           []raft.ServerID{"1", "2", "3"},
		Servers:          map[raft.ServerID]*autopilot.ServerState{"1": healthy("1"), "2": healthy("2"), "3": healthy("3")},
	}

	var d divergedNodes

	if got := d.overlay(state); got != state {
		t.Fatal("overlay without diverged nodes must return the state unchanged")
	}

	d.set("2", true)

	got := d.overlay(state)
	if got.Healthy || got.FailureTolerance != 0 || got.Servers["2"].Health.Healthy || !got.Servers["1"].Health.Healthy {
		t.Fatalf("overlay = healthy %v, tolerance %d, node 2 healthy %v", got.Healthy, got.FailureTolerance, got.Servers["2"].Health.Healthy)
	}

	if !state.Healthy || !state.Servers["2"].Health.Healthy {
		t.Fatal("overlay mutated the autopilot state")
	}

	d.set("2", false)

	if got := d.overlay(state); !got.Healthy {
		t.Fatal("cleared node still reported unhealthy")
	}
}

func flagged(database *Database, ids ...string) map[string]bool {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = database.diverged.has(id)
	}

	return out
}

func TestJudgeStateHashesUsesTheVoterMajority(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	ctx := context.Background()
	voters := map[string]bool{"1": true, "2": true, "3": true, "4": false, "5": false}

	selfDiverged := database.judgeStateHashes(ctx, 10, "1", map[string]string{"1": "a", "2": "a", "3": "b", "4": "b", "5": "b"}, voters)
	if selfDiverged {
		t.Fatal("leader in the voter majority reported as diverged")
	}

	want := map[string]bool{"1": false, "2": false, "3": true, "4": true, "5": true}
	if got := flagged(database, "1", "2", "3", "4", "5"); !maps.Equal(got, want) {
		t.Fatalf("flagged = %v, want %v (non-voters must not form the majority)", got, want)
	}

	if !database.judgeStateHashes(ctx, 11, "3", map[string]string{"1": "a", "2": "a", "3": "b"}, voters) {
		t.Fatal("leader in the voter minority must give up leadership")
	}
}

func TestJudgeStateHashesFlagsNoNodeWithoutAMajority(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	ctx := context.Background()
	voters := map[string]bool{"1": true, "2": true, "3": false}

	database.diverged.set("9", true)

	if database.judgeStateHashes(ctx, 10, "1", map[string]string{"1": "a", "2": "b", "3": "a"}, voters) {
		t.Fatal("leader must not give up leadership without a majority")
	}

	want := map[string]bool{"1": false, "2": false, "3": false, "9": true}
	if got := flagged(database, "1", "2", "3", "9"); !maps.Equal(got, want) {
		t.Fatalf("flagged = %v, want %v", got, want)
	}

	database.judgeStateHashes(ctx, 11, "1", map[string]string{"1": "a", "2": "a", "3": "b"}, voters)

	if !database.diverged.has("3") || !database.diverged.has("9") {
		t.Fatal("with a majority, a differing node must be flagged and a node that did not report must keep its flag")
	}
}

func TestStateHashCoversLeaderTimeAndStructure(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	seedDeterminismFixture(t, database)

	base := stateHashOf(t, database)

	for _, tc := range []struct {
		name   string
		change string
		undo   string
	}{
		{"leader time", "UPDATE fsm_state SET leaderTime = leaderTime + 1 WHERE id = 1", "UPDATE fsm_state SET leaderTime = leaderTime - 1 WHERE id = 1"},
		{"missing index", "DROP INDEX idx_leases_pool_latest", createLatestLeaseIndexSQL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			execSQL(t, database, tc.change)

			if got := stateHashOf(t, database); got == base {
				t.Fatalf("hash unchanged after %s", tc.change)
			}

			execSQL(t, database, tc.undo)

			if got := stateHashOf(t, database); got != base {
				t.Fatalf("hash not restored after %s", tc.undo)
			}
		})
	}
}
