// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/canonical/sqlair"
)

func newClusterDatabaseAtV19(t *testing.T) *Database {
	t.Helper()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	conn, err := openSQLiteConnection(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if err := runMigrations(ctx, conn, 19); err != nil {
		t.Fatalf("runMigrations(19): %v", err)
	}

	if err := ensureFsmStateTable(ctx, conn); err != nil {
		t.Fatalf("ensure fsm_state table: %v", err)
	}

	d := new(Database)
	d.connPtr.Store(sqlair.NewDB(conn))
	d.dbPath = dbPath
	d.dataDir = filepath.Dir(dbPath)
	d.clusterEnabled = true
	d.changefeed = NewChangefeed()

	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	})

	if err := d.refreshAppliedSchema(ctx); err != nil {
		t.Fatalf("refresh applied schema: %v", err)
	}

	if err := d.PrepareStatements(); err != nil {
		t.Fatalf("prepare statements: %v", err)
	}

	RegisterMetrics(d)

	return d
}

func TestClusterMemberReadsWorkAtPreV20Schema(t *testing.T) {
	ctx := context.Background()
	d := newClusterDatabaseAtV19(t)

	_, err := d.conn().PlainDB().ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (nodeID, raftAddress, apiAddress, binaryVersion, suffrage, drainState, drainUpdatedAt) VALUES (3, '10.0.0.3:7000', '10.0.0.3:5002', 'v1.18.0', 'voter', 'active', 0)",
		ClusterMembersTableName))
	if err != nil {
		t.Fatalf("insert legacy member: %v", err)
	}

	member, err := d.GetClusterMember(ctx, "3")
	if err != nil {
		t.Fatalf("GetClusterMember at schema 19: %v", err)
	}

	if member.NodeID != "3" {
		t.Fatalf("nodeID = %q, want %q", member.NodeID, "3")
	}

	if member.RaftAddress != "10.0.0.3:7000" {
		t.Fatalf("raftAddress = %q, want %q", member.RaftAddress, "10.0.0.3:7000")
	}

	if member.DrainState != DrainStateActive {
		t.Fatalf("drainState = %q, want %q", member.DrainState, DrainStateActive)
	}

	if member.AMFPointer != 0 || member.DisplayName != "" {
		t.Fatalf("amfPointer = %d, displayName = %q, want 0 and empty", member.AMFPointer, member.DisplayName)
	}

	members, err := d.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("ListClusterMembers at schema 19: %v", err)
	}

	if len(members) != 1 || members[0].NodeID != "3" {
		t.Fatalf("members = %+v, want one row for node 3", members)
	}
}

func TestUpsertClusterMemberAtPreV20SchemaKeepsLegacyTyping(t *testing.T) {
	ctx := context.Background()
	d := newClusterDatabaseAtV19(t)

	member := &ClusterMember{
		NodeID:        "3",
		RaftAddress:   "10.0.0.3:7000",
		APIAddress:    "10.0.0.3:5002",
		BinaryVersion: "v1.19.0",
		Suffrage:      "voter",
	}

	if _, err := d.applyUpsertClusterMember(ctx, member); err != nil {
		t.Fatalf("applyUpsertClusterMember at schema 19: %v", err)
	}

	var typeOf string

	err := d.conn().PlainDB().QueryRowContext(ctx, fmt.Sprintf(
		"SELECT typeof(nodeID) FROM %s WHERE nodeID=3", ClusterMembersTableName)).Scan(&typeOf)
	if err != nil {
		t.Fatalf("read typeof(nodeID): %v", err)
	}

	if typeOf != "integer" {
		t.Fatalf("typeof(nodeID) = %q, want %q: a pre-v20 peer's changeset matches rows on old values", typeOf, "integer")
	}

	if _, err := d.applyUpsertClusterMember(ctx, member); err != nil {
		t.Fatalf("second applyUpsertClusterMember at schema 19: %v", err)
	}

	count, err := d.CountClusterMembers(ctx)
	if err != nil {
		t.Fatalf("CountClusterMembers: %v", err)
	}

	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestInsertJoinTokenAtPreV20Schema(t *testing.T) {
	ctx := context.Background()
	d := newClusterDatabaseAtV19(t)

	token := &ClusterJoinToken{
		ID:         "01890000-0000-7000-8000-000000000001",
		ClaimsJSON: `{"iss":"test"}`,
		ExpiresAt:  1,
	}

	if _, err := d.applyInsertJoinToken(ctx, token); err != nil {
		t.Fatalf("applyInsertJoinToken at schema 19: %v", err)
	}

	got, err := d.GetJoinToken(ctx, token.ID)
	if err != nil {
		t.Fatalf("GetJoinToken at schema 19: %v", err)
	}

	if got.ID != token.ID || got.ConsumedAt != 0 {
		t.Fatalf("token = %+v, want id %q unconsumed", got, token.ID)
	}
}

func TestUpsertClusterMemberAtPreV20SchemaRejectsUUIDIdentity(t *testing.T) {
	ctx := context.Background()
	d := newClusterDatabaseAtV19(t)

	member := &ClusterMember{
		NodeID:        "01890000-0000-7000-8000-000000000001",
		RaftAddress:   "10.0.0.4:7000",
		APIAddress:    "10.0.0.4:5002",
		BinaryVersion: "v1.19.0",
		Suffrage:      "voter",
	}

	if _, err := d.applyUpsertClusterMember(ctx, member); err == nil {
		t.Fatal("applyUpsertClusterMember accepted a UUID identity at schema 19; " +
			"nodeID is an INTEGER PRIMARY KEY until v20, so the leader must migrate before writing its row")
	}
}
