// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
	"go.uber.org/zap"
)

const (
	verifyStateOp           = "VerifyState"
	verifyStateInterval     = 15 * time.Minute
	verifyStatePollInterval = 10 * time.Second
	stateHashWait           = time.Minute
	stateHashRetry          = time.Second
	stateHashReaderTimeout  = 5 * time.Second
)

func (db *Database) applyVerifyState(ctx context.Context, _ *struct{}) (any, error) {
	info, ok := applyInfoFrom(ctx)
	if !ok {
		return nil, invalidCommand(errors.New("VerifyState requires the command path"))
	}

	return info.index, nil
}

type stateHashRecord struct {
	mu     sync.Mutex
	latest ellaraft.StateHash
}

func (r *stateHashRecord) store(h ellaraft.StateHash) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if h.Index > r.latest.Index {
		r.latest = h
	}
}

func (r *stateHashRecord) load() ellaraft.StateHash {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.latest
}

func (db *Database) StateHash() ellaraft.StateHash {
	return db.stateHash.load()
}

func (db *Database) captureStateHash(index uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), stateHashReaderTimeout)
	defer cancel()

	conn, err := db.reader().Conn(ctx)
	if err != nil {
		logger.DBLog.Warn("State hash skipped: no reader connection", zap.Uint64("index", index), zap.Error(err))
		return
	}

	tx, err := conn.BeginTx(context.Background(), nil)
	if err != nil {
		_ = conn.Close()

		logger.DBLog.Warn("State hash skipped: begin read transaction", zap.Uint64("index", index), zap.Error(err))

		return
	}

	var lastApplied uint64
	if err := tx.QueryRowContext(ctx, "SELECT lastApplied FROM fsm_state WHERE id = 1").Scan(&lastApplied); err != nil || lastApplied != index {
		_ = tx.Rollback()
		_ = conn.Close()

		logger.DBLog.Warn("State hash skipped: read snapshot is not at the verified index",
			zap.Uint64("index", index), zap.Uint64("snapshot_index", lastApplied), zap.Error(err))

		return
	}

	go db.hashPinnedState(index, conn, tx)
}

func (db *Database) hashPinnedState(index uint64, conn *sql.Conn, tx *sql.Tx) {
	defer func() { _ = conn.Close() }()
	defer func() { _ = tx.Rollback() }()

	start := time.Now()

	sum, err := hashReplicatedState(context.Background(), tx)
	if err != nil {
		logger.DBLog.Warn("State hash failed", zap.Uint64("index", index), zap.Error(err))
		return
	}

	db.stateHash.store(ellaraft.StateHash{Index: index, Hash: sum})

	logger.DBLog.Debug("State hash computed", zap.Uint64("index", index), zap.String("hash", sum), zap.Duration("duration", time.Since(start)))
}

type rowQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

const (
	hashTagTable byte = iota + 1
	hashTagRow
	hashTagNull
	hashTagInteger
	hashTagReal
	hashTagText
	hashTagBlob
)

func hashReplicatedState(ctx context.Context, q rowQuerier) (string, error) {
	h := sha256.New()

	for _, table := range slices.Sorted(slices.Values(replicatedChangesetTables)) {
		if err := hashTable(ctx, q, h, table); err != nil {
			return "", fmt.Errorf("hash %s: %w", table, err)
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashTable(ctx context.Context, q rowQuerier, h hash.Hash, table string) error {
	columns, orderBy, err := tableHashColumns(ctx, q, table)
	if err != nil {
		return err
	}

	if len(columns) == 0 {
		return nil
	}

	h.Write([]byte{hashTagTable})
	writeHashBytes(h, []byte(table))

	selected := make([]string, len(columns))

	for i, c := range columns {
		writeHashBytes(h, []byte(c))
		selected[i] = "+" + quoteIdentifier(c)
	}

	query := "SELECT " + strings.Join(selected, ", ") + " FROM " + quoteIdentifier(table) + " ORDER BY " + strings.Join(orderBy, ", ")

	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return err
	}

	defer func() { _ = rows.Close() }()

	values := make([]any, len(columns))
	targets := make([]any, len(columns))

	for i := range values {
		targets[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return err
		}

		h.Write([]byte{hashTagRow})

		for _, v := range values {
			if err := writeHashValue(h, v); err != nil {
				return err
			}
		}
	}

	return rows.Err()
}

func tableHashColumns(ctx context.Context, q rowQuerier, table string) ([]string, []string, error) {
	rows, err := q.QueryContext(ctx, "SELECT name, pk FROM pragma_table_info(?) ORDER BY cid", table)
	if err != nil {
		return nil, nil, err
	}

	defer func() { _ = rows.Close() }()

	var (
		columns []string
		pk      = map[int]string{}
	)

	for rows.Next() {
		var (
			name     string
			position int
		)

		if err := rows.Scan(&name, &position); err != nil {
			return nil, nil, err
		}

		columns = append(columns, name)

		if position > 0 {
			pk[position] = name
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	orderBy := make([]string, 0, len(columns))

	if len(pk) == 0 {
		for _, c := range columns {
			orderBy = append(orderBy, quoteIdentifier(c))
		}
	} else {
		for i := 1; i <= len(pk); i++ {
			orderBy = append(orderBy, quoteIdentifier(pk[i]))
		}
	}

	return columns, orderBy, nil
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func writeHashBytes(h hash.Hash, b []byte) {
	var n [8]byte

	binary.BigEndian.PutUint64(n[:], uint64(len(b)))
	h.Write(n[:])
	h.Write(b)
}

func writeHashValue(h hash.Hash, v any) error {
	var n [8]byte

	switch x := v.(type) {
	case nil:
		h.Write([]byte{hashTagNull})
	case int64:
		binary.BigEndian.PutUint64(n[:], uint64(x))
		h.Write([]byte{hashTagInteger})
		h.Write(n[:])
	case float64:
		binary.BigEndian.PutUint64(n[:], math.Float64bits(x))
		h.Write([]byte{hashTagReal})
		h.Write(n[:])
	case string:
		h.Write([]byte{hashTagText})
		writeHashBytes(h, []byte(x))
	case []byte:
		h.Write([]byte{hashTagBlob})
		writeHashBytes(h, x)
	default:
		return fmt.Errorf("unexpected column value type %T", v)
	}

	return nil
}

type divergedNodes struct {
	mu    sync.Mutex
	nodes map[string]struct{}
}

func (d *divergedNodes) set(nodeID string, diverged bool) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, was := d.nodes[nodeID]

	if diverged {
		if d.nodes == nil {
			d.nodes = make(map[string]struct{})
		}

		d.nodes[nodeID] = struct{}{}
	} else {
		delete(d.nodes, nodeID)
	}

	return was
}

func (d *divergedNodes) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.nodes = nil
}

func (d *divergedNodes) has(nodeID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, ok := d.nodes[nodeID]

	return ok
}

func (d *divergedNodes) overlay(state *autopilot.State) *autopilot.State {
	if state == nil {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.nodes) == 0 {
		return state
	}

	out := *state
	out.Servers = make(map[raft.ServerID]*autopilot.ServerState, len(state.Servers))

	for id, srv := range state.Servers {
		s := *srv

		if _, diverged := d.nodes[string(id)]; diverged {
			s.Health.Healthy = false
			out.Healthy = false
		}

		out.Servers[id] = &s
	}

	healthyVoters := 0

	for _, id := range out.Voters {
		if srv, ok := out.Servers[id]; ok && srv.Health.Healthy {
			healthyVoters++
		}
	}

	out.FailureTolerance = max(healthyVoters-(len(out.Voters)/2+1), 0)

	return &out
}

func (db *Database) runStateVerifier(ctx context.Context) {
	defer db.diverged.reset()

	ticker := time.NewTicker(verifyStatePollInterval)
	defer ticker.Stop()

	var last time.Time

	for {
		if db.commandMode(ctx) && (last.IsZero() || time.Since(last) >= verifyStateInterval) {
			last = time.Now()

			db.verifyState(ctx)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (db *Database) verifyState(ctx context.Context) {
	index, err := opVerifyState.Invoke(ctx, db, &struct{}{})
	if err != nil {
		logger.From(ctx, logger.DBLog).Warn("State verification could not be proposed", zap.Error(err))
		return
	}

	if !db.IsLeader() {
		return
	}

	deadline := time.Now().Add(stateHashWait)
	selfID := db.RaftID()
	reports := make(map[string]string)
	voters := make(map[string]bool)

	for _, srv := range db.RaftServers() {
		voters[srv.NodeID] = srv.Suffrage == "voter"

		fetch := func(ctx context.Context) (ellaraft.StateHash, error) {
			return db.probeStateHash(ctx, srv.NodeID, srv.Address)
		}

		if srv.NodeID == selfID {
			fetch = func(context.Context) (ellaraft.StateHash, error) { return db.StateHash(), nil }
		}

		if sum, ok := stateHashAt(ctx, fetch, index, deadline); ok {
			reports[srv.NodeID] = sum
		} else {
			logger.From(ctx, logger.DBLog).Debug("State verification skipped a node that did not report the index",
				zap.String("node_id", srv.NodeID), zap.Uint64("index", index))
		}
	}

	if ctx.Err() != nil {
		return
	}

	if db.judgeStateHashes(ctx, index, selfID, reports, voters) {
		db.leaveDivergedLeadership(ctx, index)
	}
}

func stateHashAt(ctx context.Context, fetch func(context.Context) (ellaraft.StateHash, error), index uint64, deadline time.Time) (string, bool) {
	for {
		got, err := fetch(ctx)
		if err == nil {
			if got.Index == index {
				return got.Hash, true
			}

			if got.Index > index {
				return "", false
			}
		}

		if !time.Now().Before(deadline) {
			return "", false
		}

		select {
		case <-ctx.Done():
			return "", false
		case <-time.After(stateHashRetry):
		}
	}
}

func (db *Database) judgeStateHashes(ctx context.Context, index uint64, selfID string, reports map[string]string, voters map[string]bool) bool {
	log := logger.From(ctx, logger.DBLog)

	counts := make(map[string]int)
	voterReports := 0

	for nodeID, sum := range reports {
		if voters[nodeID] {
			counts[sum]++
			voterReports++
		}
	}

	reference := ""

	for sum, n := range counts {
		if n*2 > voterReports {
			reference = sum
		}
	}

	hasMajority := reference != ""

	if !hasMajority {
		reference = reports[selfID]
		if reference == "" {
			return false
		}

		if len(counts) > 1 {
			log.Error("Replicated state differs between voters and no hash has a majority; flagging nodes that differ from the leader",
				zap.Uint64("index", index), zap.Any("hashes", reports))
		}
	}

	selfDiverged := false

	for nodeID, sum := range reports {
		if sum == reference {
			if db.diverged.set(nodeID, false) {
				log.Info("Replicated state matches the cluster again", zap.String("node_id", nodeID), zap.Uint64("index", index))
			}

			continue
		}

		db.diverged.set(nodeID, true)

		log.Error("Replicated state diverged from the cluster",
			zap.String("node_id", nodeID),
			zap.Uint64("index", index),
			zap.String("hash", sum),
			zap.String("reference_hash", reference),
			zap.Bool("majority", hasMajority))

		if nodeID == selfID {
			selfDiverged = true
		}
	}

	return selfDiverged
}

func (db *Database) leaveDivergedLeadership(ctx context.Context, index uint64) {
	log := logger.From(ctx, logger.DBLog)

	if err := db.LeadershipTransfer(); err != nil {
		log.Error("Leader state diverged but leadership could not be transferred", zap.Uint64("index", index), zap.Error(err))
		return
	}

	log.Warn("Leadership transferred away from a node whose state diverged", zap.Uint64("index", index))
}
