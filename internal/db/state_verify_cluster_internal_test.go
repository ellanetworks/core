// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/hashicorp/raft"
)

type verifyCluster struct {
	t   *testing.T
	tc  *ellaraft.TestCluster
	dbs []*Database
}

func openClusterNodeDB(t *testing.T, path string) *Database {
	t.Helper()

	ctx := t.Context()

	sqlConn, connector, err := openSQLitePool(ctx, path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if err := runMigrations(ctx, sqlConn, baselineVersion); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := ensureFsmStateTable(ctx, sqlConn); err != nil {
		t.Fatalf("fsm_state: %v", err)
	}

	d := new(Database)
	d.connection = sqlConn
	d.connector = connector
	d.dbPath = path
	d.dataDir = filepath.Dir(path)
	d.clusterEnabled = true
	d.migrationCeiling = baselineVersion
	d.changefeed = NewChangefeed()
	d.migrationCheckCh = make(chan struct{}, 1)

	if err := d.openReaderPool(ctx); err != nil {
		t.Fatalf("reader pool: %v", err)
	}

	if err := d.refreshAppliedSchema(ctx); err != nil {
		t.Fatalf("schema cache: %v", err)
	}

	RegisterMetrics(d)

	if err := d.InitializeLocalSettings(ctx); err != nil {
		t.Fatalf("local settings: %v", err)
	}

	return d
}

func newVerifyCluster(t *testing.T) *verifyCluster {
	t.Helper()

	vc := &verifyCluster{t: t}

	for range 3 {
		vc.dbs = append(vc.dbs, openClusterNodeDB(t, filepath.Join(t.TempDir(), DBFilename)))
	}

	t.Cleanup(func() {
		for _, d := range vc.dbs {
			d.raftManager = nil
			_ = d.Close()
		}
	})

	next := -1
	vc.tc = ellaraft.SetupTestClusterWithAppliers(t, 3, func() ellaraft.Applier {
		next++
		return vc.dbs[next]
	})

	byID := make(map[string]*Database, len(vc.dbs))

	for i, d := range vc.dbs {
		d.adoptRaftManager(vc.tc.Nodes[i])
		byID[d.RaftID()] = d
	}

	for _, d := range vc.dbs {
		d.probeStateHash = func(_ context.Context, nodeID string, _ string) (ellaraft.StateHash, error) {
			return byID[nodeID].StateHash(), nil
		}
	}

	ctx := t.Context()
	leader := vc.leader()

	current, err := leader.CurrentSchemaVersion(ctx)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	for v := current + 1; v <= SchemaVersion(); v++ {
		p := migrateSharedPayload{TargetVersion: v}

		if v == sliceDataNetworkSchema {
			ls, nat := false, true
			p.LocalSwitch = &ls
			p.NAT = &nat
		}

		if _, err := opMigrateShared.Invoke(ctx, leader, p); err != nil {
			t.Fatalf("migrate to %d: %v", v, err)
		}
	}

	if err := leader.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	if err := vc.tc.WaitForConvergence(leader.raftManager.AppliedIndex(), 20*time.Second); err != nil {
		t.Fatalf("converge: %v", err)
	}

	for _, d := range vc.dbs {
		if err := d.refreshAppliedSchema(ctx); err != nil {
			t.Fatalf("schema cache: %v", err)
		}
	}

	return vc
}

func (vc *verifyCluster) leader() *Database {
	vc.t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		for _, d := range vc.dbs {
			if d.raftManager.IsLeader() {
				return d
			}
		}

		time.Sleep(10 * time.Millisecond)
	}

	vc.t.Fatal("no leader")

	return nil
}

func (vc *verifyCluster) follower(leader *Database) *Database {
	for _, d := range vc.dbs {
		if d != leader {
			return d
		}
	}

	return nil
}

func (vc *verifyCluster) waitForServerHealth(leader *Database, want map[string]bool) {
	vc.t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	var last string

	for time.Now().Before(deadline) {
		state := leader.AutopilotState()
		if state != nil {
			ok := len(state.Servers) == len(want)

			for id, healthy := range want {
				srv := state.Servers[raft.ServerID(id)]
				if srv == nil || srv.Health.Healthy != healthy {
					ok = false
				}
			}

			allHealthy := true
			for _, h := range want {
				allHealthy = allHealthy && h
			}

			if ok && state.Healthy == allHealthy {
				return
			}

			last = ""
			for id, srv := range state.Servers {
				last += string(id) + "=" + map[bool]string{true: "healthy", false: "unhealthy"}[srv.Health.Healthy] + " "
			}
		}

		time.Sleep(20 * time.Millisecond)
	}

	vc.t.Fatalf("autopilot state never matched %v; last %s", want, last)
}

func healthExcept(dbs []*Database, unhealthy *Database) map[string]bool {
	out := make(map[string]bool, len(dbs))
	for _, d := range dbs {
		out[d.RaftID()] = d != unhealthy
	}

	return out
}

const (
	corruptMTU = "UPDATE data_networks SET mtu = mtu + 1"
	repairMTU  = "UPDATE data_networks SET mtu = mtu - 1"
)

func TestVerifyStateFlagsADivergedFollower(t *testing.T) {
	vc := newVerifyCluster(t)
	ctx := t.Context()
	leader := vc.leader()
	follower := vc.follower(leader)

	leader.verifyState(ctx)

	want := leader.StateHash()

	for _, d := range vc.dbs {
		if leader.diverged.has(d.RaftID()) {
			t.Fatalf("node %s flagged before any corruption", d.RaftID())
		}

		if got := d.StateHash(); got != want {
			t.Fatalf("node %s state hash %+v, leader %+v", d.RaftID(), got, want)
		}
	}

	vc.waitForServerHealth(leader, healthExcept(vc.dbs, nil))

	execSQL(t, follower, corruptMTU)

	leader.verifyState(ctx)

	if !leader.diverged.has(follower.RaftID()) {
		t.Fatalf("follower %s not flagged after corruption", follower.RaftID())
	}

	if !leader.raftManager.IsLeader() {
		t.Fatal("leader gave up leadership although only a follower diverged")
	}

	vc.waitForServerHealth(leader, healthExcept(vc.dbs, follower))

	execSQL(t, follower, repairMTU)

	leader.verifyState(ctx)

	if leader.diverged.has(follower.RaftID()) {
		t.Fatal("follower still flagged after repair")
	}

	vc.waitForServerHealth(leader, healthExcept(vc.dbs, nil))
}

func TestVerifyStateMovesLeadershipOffADivergedLeader(t *testing.T) {
	vc := newVerifyCluster(t)
	ctx := t.Context()
	old := vc.leader()

	execSQL(t, old, corruptMTU)

	old.verifyState(ctx)

	deadline := time.Now().Add(10 * time.Second)
	for old.raftManager.IsLeader() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if old.raftManager.IsLeader() {
		t.Fatal("diverged leader kept leadership")
	}

	leader := vc.leader()
	if leader == old {
		t.Fatal("leadership came back to the diverged node")
	}

	leader.verifyState(ctx)

	if !leader.diverged.has(old.RaftID()) {
		t.Fatalf("new leader did not flag the old leader %s", old.RaftID())
	}

	vc.waitForServerHealth(leader, healthExcept(vc.dbs, old))

	execSQL(t, old, repairMTU)

	leader.verifyState(ctx)

	if leader.diverged.has(old.RaftID()) {
		t.Fatal("old leader still flagged after repair")
	}

	vc.waitForServerHealth(leader, healthExcept(vc.dbs, nil))
}

func TestVerifyStateFlagsNonvotersAndNoNodeWithoutAMajority(t *testing.T) {
	vc := newVerifyCluster(t)
	ctx := t.Context()
	leader := vc.leader()

	var voter, nonvoter *Database

	for _, d := range vc.dbs {
		switch {
		case d == leader:
		case voter == nil:
			voter = d
		default:
			nonvoter = d
		}
	}

	servers := leader.raftServers
	leader.raftServers = func() []ellaraft.Server {
		out := servers()
		for i := range out {
			if out[i].NodeID == nonvoter.RaftID() {
				out[i].Suffrage = "nonvoter"
			}
		}

		return out
	}

	vc.waitForServerHealth(leader, healthExcept(vc.dbs, nil))

	execSQL(t, nonvoter, corruptMTU)
	leader.verifyState(ctx)

	if !leader.diverged.has(nonvoter.RaftID()) || leader.diverged.has(voter.RaftID()) || leader.diverged.has(leader.RaftID()) {
		t.Fatal("only the corrupted non-voter must be flagged")
	}

	vc.waitForServerHealth(leader, healthExcept(vc.dbs, nonvoter))

	execSQL(t, nonvoter, repairMTU)
	leader.verifyState(ctx)

	if leader.diverged.has(nonvoter.RaftID()) {
		t.Fatal("non-voter still flagged after repair")
	}

	vc.waitForServerHealth(leader, healthExcept(vc.dbs, nil))

	execSQL(t, voter, corruptMTU)
	leader.verifyState(ctx)

	if leader.diverged.has(voter.RaftID()) || leader.diverged.has(nonvoter.RaftID()) || leader.diverged.has(leader.RaftID()) {
		t.Fatal("without a voter majority, no node must be flagged")
	}

	if !leader.raftManager.IsLeader() {
		t.Fatal("leader gave up leadership without a majority against it")
	}

	vc.waitForServerHealth(leader, healthExcept(vc.dbs, nil))

	execSQL(t, voter, repairMTU)
	leader.verifyState(ctx)

	if leader.diverged.has(voter.RaftID()) {
		t.Fatal("voter still flagged after repair")
	}

	vc.waitForServerHealth(leader, healthExcept(vc.dbs, nil))
}

func TestClusterHoldsTheCommandSchemaWhileRowsBreakAForeignKey(t *testing.T) {
	vc := newVerifyCluster(t)
	leader := vc.leader()

	seedOrphanSubscriber(t, leader)

	blocked, violations, err := leader.commandSchemaBlocked(t.Context(), commandSchema-1, commandSchema)
	if err != nil {
		t.Fatalf("commandSchemaBlocked: %v", err)
	}

	if !blocked || len(violations) != 1 {
		t.Fatalf("blocked = %v, violations = %v; want the orphaned subscriber to hold migration %d", blocked, violations, commandSchema)
	}

	if blocked, _, _ := leader.commandSchemaBlocked(t.Context(), commandSchema, commandSchema); blocked {
		t.Fatal("a cluster already at the command schema has nothing to hold")
	}
}
