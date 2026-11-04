// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	hraft "github.com/hashicorp/raft"
)

func countingApplier(t *testing.T) (*testApplier, *atomic.Int32) {
	t.Helper()

	var restores atomic.Int32

	a := newTestApplier(t)
	a.writeRows = true
	a.reopenHook = func() { restores.Add(1) }

	return a, &restores
}

func appliedState(t *testing.T, a *testApplier) AppliedState {
	t.Helper()

	state, err := readAppliedState(context.Background(), a.PlainDB())
	if err != nil {
		t.Fatalf("read fsm_state: %v", err)
	}

	return state
}

func rowCount(t *testing.T, a *testApplier) int {
	t.Helper()

	var n int
	if err := a.PlainDB().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM t").Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}

	return n
}

func TestRestartAfterSnapshotsNeverRestores(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dataDir := t.TempDir()
	a, restores := countingApplier(t)

	m, err := NewManager(ctx, FastTestConfig(), a, dataDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := m.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("barrier: %v", err)
	}

	proposeSome(t, m, 3)

	for boot := 1; boot <= 3; boot++ {
		if err := m.Barrier(5 * time.Second); err != nil {
			t.Fatalf("boot %d barrier: %v", boot, err)
		}

		if err := m.Snapshot(); err != nil {
			t.Fatalf("boot %d snapshot: %v", boot, err)
		}

		snapIndex, err := newestSnapshotIndex(m.snaps)
		if err != nil {
			t.Fatalf("boot %d list snapshots: %v", boot, err)
		}

		if got := appliedState(t, a).Index; got < snapIndex {
			t.Fatalf("boot %d: fsm_state.lastApplied %d is behind the snapshot index %d", boot, got, snapIndex)
		}

		if err := m.Shutdown(); err != nil {
			t.Fatalf("boot %d shutdown: %v", boot, err)
		}

		m, err = NewManager(ctx, FastTestConfig(), a, dataDir)
		if err != nil {
			t.Fatalf("boot %d restart: %v", boot, err)
		}

		if err := m.WaitForLeaderBarrier(ctx); err != nil {
			t.Fatalf("boot %d restart barrier: %v", boot, err)
		}
	}

	t.Cleanup(func() { _ = m.Shutdown() })

	if n := restores.Load(); n != 0 {
		t.Fatalf("restarts restored the database %d times; the database already holds every applied entry", n)
	}

	if n := rowCount(t, a); n != 3 {
		t.Fatalf("want 3 rows, got %d", n)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "raft", legacySnapshotsDir)); !os.IsNotExist(err) {
		t.Fatalf("snapshots must not be written to disk: %v", err)
	}
}

func TestLocalSnapshotRecordsRaftPosition(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	a, _ := countingApplier(t)

	m, err := NewManager(ctx, FastTestConfig(), a, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	t.Cleanup(func() { _ = m.Shutdown() })

	if err := m.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("barrier: %v", err)
	}

	proposeSome(t, m, 2)

	if err := m.Barrier(5 * time.Second); err != nil {
		t.Fatalf("barrier: %v", err)
	}

	if err := m.Snapshot(); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	last := m.raft.LastIndex()
	state := appliedState(t, a)

	if state.Index != last {
		t.Fatalf("fsm_state.lastApplied = %d, want the snapshot index %d", state.Index, last)
	}

	if state.Term != m.raft.CurrentTerm() {
		t.Fatalf("fsm_state.lastAppliedTerm = %d, want %d", state.Term, m.raft.CurrentTerm())
	}

	configuration := hraft.DecodeConfiguration(state.Configuration)
	if len(configuration.Servers) != 1 || string(configuration.Servers[0].ID) != m.raftID {
		t.Fatalf("fsm_state.configuration = %+v, want the sole local server", configuration.Servers)
	}
}

func TestFollowerInstallSnapshotRecordsPosition(t *testing.T) {
	var (
		restores [3]atomic.Int32
		appliers []*testApplier
	)

	tc := SetupTestClusterWithAppliers(t, 3, func() Applier {
		i := len(appliers)

		a := newTestApplier(t)
		a.writeRows = true
		a.reopenHook = func() { restores[i].Add(1) }

		appliers = append(appliers, a)

		return a
	})

	leader := tc.WaitForLeader(5 * time.Second)
	follower := (tc.LeaderIndex() + 1) % 3

	tc.StopNode(follower)

	const writes = 300

	for i := 0; i < writes; i++ {
		cmd, err := NewCommand(CmdChangeset, map[string]int{"i": i})
		if err != nil {
			t.Fatalf("NewCommand: %v", err)
		}

		if _, err := leader.Propose(cmd, 5*time.Second); err != nil {
			t.Fatalf("propose %d: %v", i, err)
		}
	}

	if err := leader.Snapshot(); err != nil {
		t.Fatalf("leader snapshot: %v", err)
	}

	tc.RestartNode(follower)

	deadline := time.Now().Add(15 * time.Second)
	for rowCount(t, appliers[follower]) < writes {
		if time.Now().After(deadline) {
			t.Fatalf("follower never caught up: %d rows", rowCount(t, appliers[follower]))
		}

		time.Sleep(20 * time.Millisecond)
	}

	if err := tc.WaitForConvergence(leader.raft.AppliedIndex(), 10*time.Second); err != nil {
		t.Fatalf("converge: %v", err)
	}

	installs := restores[follower].Load()
	if installs == 0 {
		t.Fatal("follower must catch up through InstallSnapshot")
	}

	installed := appliedState(t, appliers[follower])
	if installed.Index < uint64(writes) || installed.Term == 0 || installed.Configuration == nil {
		t.Fatalf("installed snapshot position was not recorded: %+v", installed)
	}

	if servers := hraft.DecodeConfiguration(installed.Configuration).Servers; len(servers) != 3 {
		t.Fatalf("installed configuration has %d servers, want 3", len(servers))
	}

	tc.RestartNode(follower)
	time.Sleep(300 * time.Millisecond)

	if n := restores[follower].Load(); n != installs {
		t.Fatalf("restarting after an install restored again (%d restores, %d before restart)", n, installs)
	}

	if n := rowCount(t, appliers[follower]); n != writes {
		t.Fatalf("follower has %d rows after restart, want %d", n, writes)
	}
}

func TestPeersJSONRecoveryPersistsConfiguration(t *testing.T) {
	var (
		restores [3]atomic.Int32
		appliers []*testApplier
	)

	tc := SetupTestClusterWithAppliers(t, 3, func() Applier {
		i := len(appliers)

		a := newTestApplier(t)
		a.writeRows = true
		a.reopenHook = func() { restores[i].Add(1) }

		appliers = append(appliers, a)

		return a
	})

	leader := tc.WaitForLeader(5 * time.Second)

	cmd, err := NewCommand(CmdChangeset, map[string]string{"phase": "before-loss"})
	if err != nil {
		t.Fatalf("NewCommand: %v", err)
	}

	if _, err := leader.Propose(cmd, 5*time.Second); err != nil {
		t.Fatalf("propose: %v", err)
	}

	if err := tc.WaitForConvergence(leader.raft.AppliedIndex(), 5*time.Second); err != nil {
		t.Fatalf("converge: %v", err)
	}

	for i := range 3 {
		tc.StopNode(i)
	}

	survivor := tc.nodes[0]

	peers, err := json.Marshal([]map[string]string{{"id": survivor.nodeID, "address": survivor.addr}})
	if err != nil {
		t.Fatalf("marshal peers.json: %v", err)
	}

	if err := os.WriteFile(filepath.Join(survivor.dataDir, "raft", peersFileName), peers, 0o600); err != nil {
		t.Fatalf("write peers.json: %v", err)
	}

	for boot := 1; boot <= 2; boot++ {
		tc.startNode(survivor)
		tc.refresh()

		deadline := time.Now().Add(10 * time.Second)
		for survivor.mgr.raft.State() != hraft.Leader {
			if time.Now().After(deadline) {
				t.Fatalf("boot %d: recovered node never became leader", boot)
			}

			time.Sleep(20 * time.Millisecond)
		}

		if servers := soleServerConfig(t, survivor.mgr); len(servers) != 1 {
			t.Fatalf("boot %d: configuration has %d servers, want the recovered single server", boot, len(servers))
		}

		tc.StopNode(0)
	}

	if n := restores[0].Load(); n != 0 {
		t.Fatalf("recovery restored the database %d times", n)
	}

	if n := rowCount(t, appliers[0]); n != 1 {
		t.Fatalf("recovered node has %d rows, want 1", n)
	}
}

func userRestoreImage(t *testing.T, rows int) []byte {
	t.Helper()

	src := newTestApplier(t)

	for i := 1; i <= rows; i++ {
		if _, err := src.db.ExecContext(context.Background(), "INSERT INTO t(id, v) VALUES (?, 'backup')", i); err != nil {
			t.Fatalf("seed backup row: %v", err)
		}
	}

	path := filepath.Join(t.TempDir(), "backup.db")

	if _, err := src.db.ExecContext(context.Background(), "VACUUM INTO ?", path); err != nil {
		t.Fatalf("vacuum backup image: %v", err)
	}

	image, err := os.ReadFile(path) // #nosec: G304 — test temp file
	if err != nil {
		t.Fatalf("read backup image: %v", err)
	}

	return image
}

func lastSnapshotIndex(t *testing.T, m *Manager) uint64 {
	t.Helper()

	index, err := strconv.ParseUint(m.raft.Stats()["last_snapshot_index"], 10, 64)
	if err != nil {
		t.Fatalf("parse last_snapshot_index: %v", err)
	}

	return index
}

func TestUserRestoreStampsPositionAndSurvivesRestart(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dataDir := t.TempDir()
	a, restores := countingApplier(t)

	m, err := NewManager(ctx, FastTestConfig(), a, dataDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := m.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("barrier: %v", err)
	}

	proposeSome(t, m, 2)

	const backupRows = 5

	image := userRestoreImage(t, backupRows)

	if err := m.UserRestore(bytes.NewReader(image), int64(len(image)), 10*time.Second); err != nil {
		t.Fatalf("user restore: %v", err)
	}

	if n := restores.Load(); n != 1 {
		t.Fatalf("user restore restored %d times, want 1", n)
	}

	restoredIndex := lastSnapshotIndex(t, m)
	state := appliedState(t, a)

	if state.Index != restoredIndex || state.Term != m.raft.CurrentTerm() {
		t.Fatalf("fsm_state = %+v, want index %d and term %d stamped by the restore", state, restoredIndex, m.raft.CurrentTerm())
	}

	if servers := hraft.DecodeConfiguration(state.Configuration).Servers; len(servers) != 1 || string(servers[0].ID) != m.raftID {
		t.Fatalf("restored configuration = %+v, want the sole local server", servers)
	}

	if n := rowCount(t, a); n != backupRows {
		t.Fatalf("restored database has %d rows, want %d", n, backupRows)
	}

	entries, err := os.ReadDir(filepath.Join(dataDir, "raft", snapshotInstallsDir))
	if err != nil {
		t.Fatalf("read install dir: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("user restore left %d staged files behind", len(entries))
	}

	proposeSome(t, m, 2)

	if err := m.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	m, err = NewManager(ctx, FastTestConfig(), a, dataDir)
	if err != nil {
		t.Fatalf("restart after user restore: %v", err)
	}

	t.Cleanup(func() { _ = m.Shutdown() })

	if err := m.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("restart barrier: %v", err)
	}

	if n := restores.Load(); n != 1 {
		t.Fatalf("restart after a user restore restored again (%d restores)", n)
	}

	if n := rowCount(t, a); n != backupRows+2 {
		t.Fatalf("after restart: %d rows, want %d", n, backupRows+2)
	}

	proposeSome(t, m, 1)

	if n := rowCount(t, a); n != backupRows+3 {
		t.Fatalf("write after restart: %d rows, want %d", n, backupRows+3)
	}

	if got := appliedState(t, a).Index; got <= restoredIndex {
		t.Fatalf("fsm_state.lastApplied %d did not advance past the restore index %d", got, restoredIndex)
	}
}

func TestUserRestoreReplicatesToFollowers(t *testing.T) {
	var (
		restores [3]atomic.Int32
		appliers []*testApplier
	)

	tc := SetupTestClusterWithAppliers(t, 3, func() Applier {
		i := len(appliers)

		a := newTestApplier(t)
		a.writeRows = true
		a.reopenHook = func() { restores[i].Add(1) }

		appliers = append(appliers, a)

		return a
	})

	leader := tc.WaitForLeader(5 * time.Second)
	leaderIdx := tc.LeaderIndex()

	proposeSome(t, leader, 3)

	const backupRows = 4

	image := userRestoreImage(t, backupRows)

	if err := leader.UserRestore(bytes.NewReader(image), int64(len(image)), 10*time.Second); err != nil {
		t.Fatalf("user restore: %v", err)
	}

	restoredIndex := lastSnapshotIndex(t, leader)

	proposeSome(t, leader, 1)

	if err := tc.WaitForConvergence(leader.raft.AppliedIndex(), 15*time.Second); err != nil {
		t.Fatalf("converge after restore: %v", err)
	}

	waitRows := func(i, want int) {
		t.Helper()

		deadline := time.Now().Add(10 * time.Second)
		for rowCount(t, appliers[i]) != want {
			if time.Now().After(deadline) {
				t.Fatalf("node %d has %d rows, want %d", i, rowCount(t, appliers[i]), want)
			}

			time.Sleep(20 * time.Millisecond)
		}
	}

	for i := range 3 {
		waitRows(i, backupRows+1)

		if n := restores[i].Load(); n != 1 {
			t.Fatalf("node %d restored %d times, want 1", i, n)
		}

		state := appliedState(t, appliers[i])
		if state.Index <= restoredIndex || state.Term == 0 {
			t.Fatalf("node %d fsm_state = %+v, want a position past the restore index %d", i, state, restoredIndex)
		}

		if servers := hraft.DecodeConfiguration(state.Configuration).Servers; len(servers) != 3 {
			t.Fatalf("node %d configuration has %d servers, want 3", i, len(servers))
		}
	}

	follower := (leaderIdx + 1) % 3

	tc.RestartNode(follower)

	leader = tc.WaitForLeader(5 * time.Second)

	proposeSome(t, leader, 1)

	waitRows(follower, backupRows+2)

	if n := restores[follower].Load(); n != 1 {
		t.Fatalf("restarted follower restored again (%d restores)", n)
	}
}

func TestRecordPositionWritesEvenWhenUnchanged(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	store, err := newDatabaseSnapshotStore(fsm, t.TempDir())
	if err != nil {
		t.Fatalf("snapshot store: %v", err)
	}

	state := AppliedState{
		Index:              5,
		Term:               2,
		Configuration:      hraft.EncodeConfiguration(legacyConfiguration),
		ConfigurationIndex: 1,
	}

	if err := overwriteAppliedState(context.Background(), a.PlainDB(), state); err != nil {
		t.Fatalf("seed fsm_state: %v", err)
	}

	walSize := func() int64 {
		info, err := os.Stat(a.dbPath + "-wal")
		if err != nil {
			t.Fatalf("stat wal: %v", err)
		}

		return info.Size()
	}

	before := walSize()

	if err := store.recordPosition(state); err != nil {
		t.Fatalf("record position: %v", err)
	}

	if after := walSize(); after <= before {
		t.Fatalf("recording an unchanged position wrote no WAL frame (%d -> %d bytes), so nothing was fsynced before log compaction", before, after)
	}
}

func TestStartupRejectsDatabaseBehindRaftLog(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	if err := fsm.writeLastApplied(3); err != nil {
		t.Fatalf("seed lastApplied: %v", err)
	}

	logs := hraft.NewInmemStore()
	if err := logs.StoreLogs([]*hraft.Log{command(10, 2), command(11, 2)}); err != nil {
		t.Fatalf("store logs: %v", err)
	}

	err := assertFSMNotBehindRaftLog(fsm, logs, "raft")
	if err == nil || !strings.Contains(err.Error(), "database is behind the raft log") {
		t.Fatalf("want a database-behind error, got %v", err)
	}

	if err := fsm.writeLastApplied(9); err != nil {
		t.Fatalf("advance lastApplied: %v", err)
	}

	if err := assertFSMNotBehindRaftLog(fsm, logs, "raft"); err != nil {
		t.Fatalf("a database at the log's first index must be accepted: %v", err)
	}
}
