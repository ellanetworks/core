// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/ipam"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/google/uuid"
	hraft "github.com/hashicorp/raft"
	"go.uber.org/zap"
)

func applyIntent[P any](ctx context.Context, db *Database, cmd *ellaraft.Command, apply func(*Database, context.Context, *P) error) (any, error) {
	if err := db.assertAppliedSchema(ctx, intentMinSchemaForCmd(cmd.Type), cmd.Type.String()); err != nil {
		return nil, err
	}

	payload, err := unmarshalPayload[P](cmd.Payload)
	if err != nil {
		return nil, err
	}

	if err := apply(db, ctx, payload); err != nil {
		return nil, err
	}

	db.publishOpTopics(topicsForIntentCmd(cmd.Type))

	return nil, nil
}

func (db *Database) applyChangesetBatchCommand(ctx context.Context, cmd *ellaraft.Command, logIndex, logTerm uint64) error {
	if err := db.assertAppliedSchema(ctx, changesetBatchMinSchema, cmd.Type.String()); err != nil {
		return err
	}

	payload, err := unmarshalPayload[changesetBatchPayload](cmd.Payload)
	if err != nil {
		return err
	}

	for _, e := range payload.Entries {
		if err := db.assertAppliedSchema(ctx, e.RequiredSchema, fmt.Sprintf("changeset %q", e.Operation)); err != nil {
			return err
		}
	}

	if err := db.assertCapturedSchema(ctx, payload.CapturedSchema, cmd.Type.String()); err != nil {
		return err
	}

	if err := db.applyChangesetBatch(ctx, payload, logIndex, logTerm); err != nil {
		return err
	}

	topics := make(map[Topic]struct{})
	migrationCheck := false

	for _, e := range payload.Entries {
		if e.Operation == "UpsertClusterMember" {
			migrationCheck = true
		}

		for _, t := range topicsForChangesetOp(e.Operation) {
			topics[t] = struct{}{}
		}
	}

	if migrationCheck {
		db.signalMigrationCheck()
	}

	unique := make([]Topic, 0, len(topics))
	for t := range topics {
		unique = append(unique, t)
	}

	db.publishOpTopics(unique)

	return nil
}

// Compile-time check that *Database implements ellaraft.Applier.
var _ ellaraft.Applier = (*Database)(nil)

// ApplyCommand dispatches a Raft command to its applyX method on
// every node. assertAppliedSchema gates each dispatch on the
// per-command minSchema (RequiredSchema for CmdChangeset, intent op
// minSchema for the rest); a violation returns an error and the FSM
// panics, matching the contract for any other apply failure. The gate
// is defensive — Raft applies in log order and the leader proposes
// CmdMigrateShared(N) before any v=N op, so it only fires on leader
// misbehavior or skip-version upgrades (already a non-goal).
func (db *Database) ApplyCommand(ctx context.Context, cmd *ellaraft.Command, logIndex, logTerm uint64) (any, error) {
	switch cmd.Type {
	case ellaraft.CmdChangeset:
		payload, err := unmarshalPayload[bytesPayload](cmd.Payload)
		if err != nil {
			return nil, err
		}

		if err := db.assertAppliedSchema(ctx, payload.RequiredSchema, fmt.Sprintf("changeset %q", payload.Operation)); err != nil {
			return nil, err
		}

		if err := db.assertCapturedSchema(ctx, payload.CapturedSchema, fmt.Sprintf("changeset %q", payload.Operation)); err != nil {
			return nil, err
		}

		applyErr := db.applyChangeset(ctx, payload, logIndex, logTerm)
		if applyErr == nil {
			if payload.Operation == "UpsertClusterMember" {
				db.signalMigrationCheck()
			}

			db.publishOpTopics(topicsForChangesetOp(payload.Operation))
		}

		return nil, applyErr

	case ellaraft.CmdChangesetBatch:
		return nil, db.applyChangesetBatchCommand(ctx, cmd, logIndex, logTerm)

	case ellaraft.CmdDeleteOldAuditLogs:
		return nil, nil

	case ellaraft.CmdDeleteOldDailyUsage:
		if err := db.assertAppliedSchema(ctx, intentMinSchemaForCmd(cmd.Type), cmd.Type.String()); err != nil {
			return nil, err
		}

		payload, err := unmarshalPayload[int64Payload](cmd.Payload)
		if err != nil {
			return nil, err
		}

		applyErr := db.applyDeleteOldDailyUsage(ctx, payload)
		if applyErr == nil {
			db.publishOpTopics(topicsForIntentCmd(cmd.Type))
		}

		return nil, applyErr

	case ellaraft.CmdDeleteAllDynamicLeases:
		if err := db.assertAppliedSchema(ctx, intentMinSchemaForCmd(cmd.Type), cmd.Type.String()); err != nil {
			return nil, err
		}

		applyErr := db.applyDeleteAllDynamicLeases(ctx)
		if applyErr == nil {
			db.publishOpTopics(topicsForIntentCmd(cmd.Type))
		}

		return nil, applyErr

	case ellaraft.CmdDeleteExpiredSessions:
		if err := db.assertAppliedSchema(ctx, intentMinSchemaForCmd(cmd.Type), cmd.Type.String()); err != nil {
			return nil, err
		}

		payload, err := unmarshalPayload[int64Payload](cmd.Payload)
		if err != nil {
			return nil, err
		}

		result, applyErr := db.applyDeleteExpiredSessions(ctx, payload)
		if applyErr == nil {
			db.publishOpTopics(topicsForIntentCmd(cmd.Type))
		}

		return result, applyErr

	case ellaraft.CmdRotateJWTSecret:
		return applyIntent(ctx, db, cmd, (*Database).applyRotateJWTSecret)

	case ellaraft.CmdPutCellPosition:
		return applyIntent(ctx, db, cmd, (*Database).applyPutCellPosition)

	case ellaraft.CmdDeleteCellPosition:
		return applyIntent(ctx, db, cmd, (*Database).applyDeleteCellPosition)

	case ellaraft.CmdMigrateShared:
		payload, err := unmarshalPayload[migrateSharedPayload](cmd.Payload)
		if err != nil {
			return nil, err
		}

		result, applyErr := db.applyMigrateShared(ctx, payload)
		if applyErr == nil {
			db.signalMigrationCheck()

			if err := db.refreshAppliedSchema(ctx); err != nil {
				logger.DBLog.Warn("refresh applied-schema cache after migrate-shared apply",
					zap.Error(err))
			}
		}

		return result, applyErr

	default:
		return nil, fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// publishOpTopics fans out wakeup events for every topic an op
// declared via AffectsTopic. Safe to call with a nil/empty slice.
func (db *Database) publishOpTopics(topics []Topic) {
	if db.changefeed == nil || len(topics) == 0 {
		return
	}

	for _, t := range topics {
		db.changefeed.Publish(t)
	}
}

// PlainDB returns the raw *sql.DB for the application database.
func (db *Database) PlainDB() *sql.DB {
	return db.conn()
}

// Reopen migrates the database file FSM.Restore swapped in and retires the
// pools' connections so later checkouts open that file. Reopen is only
// invoked from FSM.Restore which holds the FSM write lock, so concurrent
// applies and snapshots are already serialised; the handle is never
// closed, so ad-hoc read sites (API handlers) never observe a closed
// database.
func (db *Database) Reopen(ctx context.Context) error {
	sqlConn, err := openSQLiteConnection(ctx, db.Path())
	if err != nil {
		return fmt.Errorf("reopen database: %w", err)
	}

	defer func() { _ = sqlConn.Close() }()

	maxVersion := db.migrationCeiling

	if err := runMigrations(ctx, sqlConn, maxVersion); err != nil {
		return fmt.Errorf("migrations after reopen: %w", err)
	}

	if err := ensureFsmStateTable(ctx, sqlConn); err != nil {
		return fmt.Errorf("ensure fsm_state after reopen: %w", err)
	}

	db.connector.retireConnections()
	db.readConnector.retireConnections()

	if err := db.refreshAppliedSchema(ctx); err != nil {
		return fmt.Errorf("refresh applied-schema cache after reopen: %w", err)
	}

	if db.changefeed != nil {
		db.changefeed.PublishAll()
	}

	db.signalMigrationCheck()

	return nil
}

func unmarshalPayload[T any](payload json.RawMessage) (*T, error) {
	var v T
	if err := json.Unmarshal(payload, &v); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	return &v, nil
}

// Payload types for simple values that don't warrant a dedicated struct.
type (
	stringPayload struct {
		Value string `json:"value"`
	}
	intPayload struct {
		Value int `json:"value"`
	}
	nodeIDPayload struct {
		Value pki.NodeID `json:"value"`
	}
	int64Payload struct {
		Value int64 `json:"value"`
	}
	boolPayload struct {
		Value bool `json:"value"`
	}
	bytesPayload struct {
		Value          []byte `json:"value"`
		Operation      string `json:"operation,omitempty"`
		RequiredSchema int    `json:"requiredSchema,omitempty"`
		CapturedSchema int    `json:"capturedSchema,omitempty"`
	}
	changesetBatchEntry struct {
		Value          []byte `json:"value"`
		Operation      string `json:"operation"`
		RequiredSchema int    `json:"requiredSchema,omitempty"`
	}
	changesetBatchPayload struct {
		Entries        []changesetBatchEntry `json:"entries"`
		CapturedSchema int                   `json:"capturedSchema,omitempty"`
	}
	auditLogPayload struct {
		ID        string `json:"id"`
		Timestamp string `json:"timestamp"`
		Level     string `json:"level"`
		Actor     string `json:"actor"`
		Action    string `json:"action"`
		IP        string `json:"ip"`
		Details   string `json:"details"`
	}
	migrateSharedPayload struct {
		TargetVersion int   `json:"targetVersion"`
		LocalSwitch   *bool `json:"localSwitch,omitempty"`
		NAT           *bool `json:"nat,omitempty"`
	}
)

// proposeChangeset and proposeIntent were removed in favour of the typed-op
// dispatch layer (internal/db/operations.go, operations_register.go). Every
// replicated write now goes through a registered ChangesetOp or intentOp,
// which handles leader capture + propose or follower forwarding.

// isTransientRaftErr reports whether a Raft apply error is transient —
// the caller should retry or surface a 503.
func isTransientRaftErr(err error) bool {
	return errors.Is(err, hraft.ErrEnqueueTimeout) ||
		errors.Is(err, hraft.ErrNotLeader) ||
		errors.Is(err, hraft.ErrLeadershipTransferInProgress) ||
		errors.Is(err, hraft.ErrRaftShutdown)
}

// --- Apply functions ---
// Each applyX function executes the actual SQL against the shared database.
// These are called both in standalone mode (directly) and in HA mode (via FSM).
// They contain no tracing or metrics — the propose layer handles those.

func (db *Database) checkMSISDNAvailable(ctx context.Context, s *Subscriber) error {
	if s.Msisdn == "" {
		return nil
	}

	identity, hosting, err := db.msisdnPSI(ctx, s.Msisdn)
	if err != nil {
		return err
	}

	if hosting == identity && hosting != "" {
		return fmt.Errorf("%w: %s is a public service identity", ErrMSISDNInUse, identity)
	}

	if hosting != "" {
		return fmt.Errorf("%w: %s is covered by the public service identity %s", ErrMSISDNInUse, identity, hosting)
	}

	holder, err := scanSubscriber(db.runner(ctx).QueryRowContext(ctx, getSubscriberByMSISDNSQL, s.Msisdn))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}

		return fmt.Errorf("query failed: %w", err)
	}

	if holder.Imsi != s.Imsi {
		return ErrMSISDNInUse
	}

	return nil
}

func (db *Database) applyCreateSubscriber(ctx context.Context, s *Subscriber) (any, error) {
	if err := db.checkMSISDNAvailable(ctx, s); err != nil {
		return nil, err
	}

	if db.appliedSchemaAtLeast(ctx, roamingSchema) {
		if _, err := db.getVisitor(ctx, db.runner(ctx), s.Imsi); err == nil {
			return nil, fmt.Errorf("%w: IMSI %s is a visitor", ErrAlreadyExists, s.Imsi)
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	if err := db.checkSubscriberCapacity(ctx); err != nil {
		return nil, err
	}

	_, err := db.runner(ctx).ExecContext(ctx, createSubscriberSQL, s.ID, s.Imsi, s.SequenceNumber, s.PermanentKey, s.Opc, s.ProfileID, s.Description, s.Msisdn)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateSubscriberProfile(ctx context.Context, s *Subscriber) (any, error) {
	if err := db.checkMSISDNAvailable(ctx, s); err != nil {
		return nil, err
	}

	res, err := db.runner(ctx).ExecContext(ctx, editSubscriberProfileSQL, s.ProfileID, s.Description, s.Msisdn, s.Imsi)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyEditSubscriberSeqNum(ctx context.Context, s *Subscriber) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, editSubscriberSeqNumSQL, s.SequenceNumber, s.Imsi)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteSubscriber(ctx context.Context, p *stringPayload) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, deleteSubscriberSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	if err := db.deleteIMSIState(ctx, p.Value); err != nil {
		return nil, err
	}

	return nil, nil
}

func (db *Database) requireKnownIMSI(ctx context.Context, imsi string) error {
	known, err := db.imsiKnown(ctx, imsi)
	if err != nil {
		return err
	}

	if !known {
		return fmt.Errorf("%w: IMSI %s is neither a subscriber nor a visitor", ErrNotFound, imsi)
	}

	return nil
}

func (db *Database) incrementDailyUsageQuery(ctx context.Context, du *DailyUsage) (string, []any) {
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return incrementDailyUsagePreV27SQL, []any{du.EpochDay, du.IMSI, du.BytesUplink, du.BytesDownlink, du.BytesUplink, du.BytesDownlink}
	}

	return incrementDailyUsageSQL, []any{du.EpochDay, du.IMSI, du.Mcc, du.Mnc, du.BytesUplink, du.BytesDownlink}
}

func (db *Database) applyIncrementDailyUsage(ctx context.Context, du *DailyUsage) (any, error) {
	query, args := db.incrementDailyUsageQuery(ctx, du)

	_, err := db.runner(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyIncrementDailyUsageBatch(ctx context.Context, batch *DailyUsageBatch) (any, error) {
	var dropped droppedDailyUsage

	runner := db.runner(ctx)

	for i := range batch.Rows {
		query, args := db.incrementDailyUsageQuery(ctx, &batch.Rows[i])

		_, err := runner.ExecContext(ctx, query, args...)
		if err == nil {
			continue
		}

		if isForeignKeyError(err) {
			dropped.Rows++
			dropped.BytesUplink += batch.Rows[i].BytesUplink
			dropped.BytesDownlink += batch.Rows[i].BytesDownlink

			continue
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return dropped, nil
}

func (db *Database) applyClearDailyUsage(ctx context.Context) error {
	_, err := db.runner(ctx).ExecContext(ctx, deleteAllDailyUsageSQL)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyDeleteOldDailyUsage(ctx context.Context, p *int64Payload) error {
	query := deleteOldDailyUsageSQL
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		query = deleteOldDailyUsagePreV27SQL
	}

	_, err := db.runner(ctx).ExecContext(ctx, query, p.Value)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyCreateLease(ctx context.Context, lease *IPLease) (any, error) {
	if err := db.requireKnownIMSI(ctx, lease.IMSI); err != nil {
		return nil, err
	}

	if lease.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate lease id: %w", err)
		}

		lease.ID = id.String()
	}

	_, err := db.runner(ctx).ExecContext(ctx, createLeaseSQL, lease.ID, lease.PoolID, lease.PoolType, lease.AddressBin, lease.IMSI, lease.SessionID, lease.Type, lease.CreatedAt, lease.NodeID)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateLeaseSession(ctx context.Context, lease *IPLease) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, updateLeaseSessionSQL, lease.SessionID, lease.ID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteDynamicLease(ctx context.Context, p *stringPayload) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, deleteLeaseSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAllDynamicLeases(ctx context.Context) error {
	_, err := db.runner(ctx).ExecContext(ctx, deleteAllDynamicLeasesSQL)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyDeleteDynamicLeasesByNode(ctx context.Context, p *nodeIDPayload) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, deleteDynLeasesByNodeSQL, string(p.Value))
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateLeaseNode(ctx context.Context, lease *IPLease) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, updateLeaseNodeSQL, lease.NodeID, lease.SessionID, lease.ID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

// applyCreateStaticLease inserts an admin-pinned reservation. The
// SELECT-then-INSERT runs under proposeMu, so the one-per-(imsi, pool,
// family) check is race-free; a duplicate address is caught by
// UNIQUE(poolID, addressBin).
func (db *Database) applyCreateStaticLease(ctx context.Context, lease *IPLease) (any, error) {
	if _, err := db.getOwnSubscriber(ctx, db.runner(ctx), lease.IMSI); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	_, err := scanIPLease(db.runner(ctx).QueryRowContext(ctx, getStaticLeaseSQL, lease.PoolID, lease.PoolType, lease.IMSI))
	switch {
	case err == nil:
		return nil, ErrAlreadyExists
	case errors.Is(err, sql.ErrNoRows):
	default:
		return nil, fmt.Errorf("get static lease: %w", err)
	}

	lease.Type = "static"
	lease.SessionID = nil
	lease.NodeID = ""
	lease.CreatedAt = time.Now().Unix()

	return db.applyCreateLease(ctx, lease)
}

// applyUpdateStaticLeaseAddress repins a reserved static lease. Zero rows
// affected maps to ErrNotFound, a duplicate address to ErrAlreadyExists.
func (db *Database) applyUpdateStaticLeaseAddress(ctx context.Context, lease *IPLease) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, updateStaticLeaseAddressSQL, lease.AddressBin, lease.ID)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

// applyDeleteStaticLease removes a reserved static lease. Zero rows affected
// maps to ErrNotFound.
func (db *Database) applyDeleteStaticLease(ctx context.Context, p *stringPayload) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, deleteStaticLeaseSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

type releaseIPLeasePayload struct {
	PoolID    string     `json:"poolId"`
	PoolType  string     `json:"poolType"`
	IMSI      string     `json:"imsi"`
	SessionID int        `json:"sessionId"`
	NodeID    pki.NodeID `json:"nodeId"`
}

func (db *Database) applyReleaseIPLease(ctx context.Context, p *releaseIPLeasePayload) (any, error) {
	if p.IMSI == "" {
		return nil, fmt.Errorf("IMSI required")
	}

	sessionID := p.SessionID
	lease, err := scanIPLease(db.runner(ctx).QueryRowContext(ctx, getLeaseBySessionSQL, p.PoolID, p.PoolType, sessionID, p.IMSI))

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", nil
	case err != nil:
		return nil, fmt.Errorf("get lease: %w", err)
	}

	if lease.NodeID != string(p.NodeID) {
		return "", nil
	}

	addr := lease.Address().String()

	// A static reservation outlives the session; only its binding is cleared, so
	// listActiveLeases and BGP (sessionID IS NOT NULL) drop the address.
	if lease.Type == "static" {
		lease.SessionID = nil

		if _, applyErr := db.applyUpdateLeaseSession(ctx, &lease); applyErr != nil {
			return nil, fmt.Errorf("clear static lease session: %w", applyErr)
		}

		return addr, nil
	}

	if _, applyErr := db.applyDeleteDynamicLease(ctx, &stringPayload{Value: lease.ID}); applyErr != nil {
		return nil, fmt.Errorf("delete dynamic lease: %w", applyErr)
	}

	return addr, nil
}

// allocateIPLeasePayload is the wire payload for AllocateIPLease. The
// caller does not pre-resolve the address — that is the whole point of
// this op. The leader's apply function picks the address atomically
// inside leaderCaptureAndPropose's proposeMu.
type allocateIPLeasePayload struct {
	PoolID    string     `json:"poolId"`
	PoolType  string     `json:"poolType"`
	IMSI      string     `json:"imsi"`
	SessionID int        `json:"sessionId"`
	NodeID    pki.NodeID `json:"nodeId"`
}

// applyAllocateIPLease atomically resolves an IP for (poolID, IMSI,
// sessionID, nodeID) and inserts the lease. Runs under
// leaderCaptureAndPropose's proposeMu and inside captureChangeset's
// pinned connection, so the scan for a free address and the
// INSERT are serialised against every other replicated write — no
// other allocation can race in between.
//
// Every query and write goes through db.runner(ctx) so they target the
// pinned SQLite connection set up by applyWithPinnedConn. Calling the
// public DB methods (GetDataNetworkByID, GetLeaseBySession, etc.) here
// would dispatch to the reader pool, which cannot see the capture's
// uncommitted writes.
func (db *Database) applyAllocateIPLease(ctx context.Context, p *allocateIPLeasePayload) (any, error) {
	if p.IMSI == "" {
		return nil, fmt.Errorf("IMSI required")
	}

	runner := db.runner(ctx)

	// Resolve the pool's CIDR. Inlined query against the pinned runner;
	// see function comment.
	dn, err := scanDataNetwork(db.runner(ctx).QueryRowContext(ctx, getDataNetworkByIDSQL, p.PoolID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("data network %s not found", p.PoolID)
		}

		return nil, fmt.Errorf("get data network %s: %w", p.PoolID, err)
	}

	var pool ipam.Pool

	switch p.PoolType {
	case "ipv6":
		pool, err = ipam.NewPool6(dn.ID, dn.IPv6Pool, 64)
		if err != nil {
			return nil, fmt.Errorf("parse ipv6 pool %q: %w", dn.IPv6Pool, err)
		}
	default:
		pool, err = ipam.NewPool(dn.ID, dn.IPv4Pool)
		if err != nil {
			return nil, fmt.Errorf("parse pool %q: %w", dn.IPv4Pool, err)
		}
	}

	if err := db.requireKnownIMSI(ctx, p.IMSI); err != nil {
		return nil, err
	}

	sessionID := p.SessionID

	// Step 0: an admin-pinned static reservation takes precedence over
	// dynamic allocation. The free-address scan below skips every leased
	// address in the pool, so a reserved static address is never handed
	// out dynamically.
	static, err := scanIPLease(runner.QueryRowContext(ctx, getStaticLeaseSQL, p.PoolID, p.PoolType, p.IMSI))
	switch {
	case err == nil:
		// A pinned address binds exactly one session. Claim it when it is
		// unheld or already held by this same session (retry/failover); a
		// concurrent second session to the same DN falls through to a
		// dynamic address, since one address cannot serve two sessions.
		if static.SessionID == nil || *static.SessionID == sessionID {
			static.SessionID = &sessionID
			if static.NodeID != string(p.NodeID) {
				static.NodeID = string(p.NodeID)
				if _, applyErr := db.applyUpdateLeaseNode(ctx, &static); applyErr != nil {
					return nil, fmt.Errorf("bind static lease node: %w", applyErr)
				}
			} else {
				if _, applyErr := db.applyUpdateLeaseSession(ctx, &static); applyErr != nil {
					return nil, fmt.Errorf("bind static lease session: %w", applyErr)
				}
			}

			return static.Address().String(), nil
		}
	case errors.Is(err, sql.ErrNoRows):
		// no static reservation; fall through to the dynamic path
	default:
		return nil, fmt.Errorf("get static lease: %w", err)
	}

	// Step 1: re-registration of a dynamic lease already held by *this*
	// session (retry or failover). Keyed on sessionID so a distinct second
	// session on the same pool gets a fresh address — a subscriber may hold
	// concurrent sessions on one DN, each with its own IP (TS 23.501
	// §5.6.1, TS 23.401 §5.10.1).
	existing, err := scanIPLease(runner.QueryRowContext(ctx, getDynamicLeaseBySessionSQL, p.PoolID, p.PoolType, p.IMSI, sessionID))
	switch {
	case err == nil:
		if existing.NodeID != string(p.NodeID) {
			existing.NodeID = string(p.NodeID)
			if _, applyErr := db.applyUpdateLeaseNode(ctx, &existing); applyErr != nil {
				return nil, fmt.Errorf("update lease node: %w", applyErr)
			}
		}

		return existing.Address().String(), nil
	case errors.Is(err, sql.ErrNoRows):
		// fall through to fresh allocation
	default:
		return nil, fmt.Errorf("get dynamic lease: %w", err)
	}

	offset, err := db.findFreeLeaseOffset(ctx, runner, pool, p.PoolType)
	if err != nil {
		return nil, err
	}

	addr := pool.AddressAtOffset(offset)
	bin := addr.As16()
	lease := &IPLease{
		PoolID:     p.PoolID,
		PoolType:   p.PoolType,
		AddressBin: bin[:],
		IMSI:       p.IMSI,
		SessionID:  &sessionID,
		Type:       "dynamic",
		CreatedAt:  time.Now().Unix(),
		NodeID:     string(p.NodeID),
	}

	if _, applyErr := db.applyCreateLease(ctx, lease); applyErr != nil {
		return nil, fmt.Errorf("create lease: %w", applyErr)
	}

	return addr.String(), nil
}

func (db *Database) findFreeLeaseOffset(ctx context.Context, runner querier, pool ipam.Pool, poolType string) (int, error) {
	first := pool.FirstUsable()
	last := first + pool.Size() - 1
	start := first

	newest, err := scanLeaseAddress(runner.QueryRowContext(ctx, getNewestLeaseInPoolSQL, pool.ID, poolType))
	switch {
	case err == nil:
		if offset := pool.OffsetOf(newest.Address()); offset >= first && offset < last {
			start = offset + 1
		}
	case errors.Is(err, sql.ErrNoRows):
	default:
		return 0, fmt.Errorf("get newest lease: %w", err)
	}

	offset, found, err := db.firstFreeLeaseOffset(ctx, runner, pool, poolType, start, last)
	if err != nil || found {
		return offset, err
	}

	if start > first {
		offset, found, err = db.firstFreeLeaseOffset(ctx, runner, pool, poolType, first, start-1)
		if err != nil || found {
			return offset, err
		}
	}

	return 0, ipam.ErrPoolExhausted
}

func (db *Database) firstFreeLeaseOffset(ctx context.Context, runner querier, pool ipam.Pool, poolType string, from, to int) (int, bool, error) {
	low := pool.AddressAtOffset(from).As16()
	high := pool.AddressAtOffset(to).As16()

	rows, err := runner.QueryContext(ctx, listLeaseAddressRangeSQL, pool.ID, poolType, low[:], high[:])
	if err != nil {
		return 0, false, fmt.Errorf("list lease addresses: %w", err)
	}

	defer func() { _ = rows.Close() }()

	next := from

	for next <= to && rows.Next() {
		lease, err := scanLeaseAddress(rows)
		if err != nil {
			return 0, false, fmt.Errorf("scan lease address: %w", err)
		}

		offset := pool.OffsetOf(lease.Address())
		if offset > next {
			break
		}

		if offset == next {
			next++
		}
	}

	if err := rows.Err(); err != nil {
		return 0, false, fmt.Errorf("list lease addresses: %w", err)
	}

	return next, next <= to, nil
}

func (db *Database) applyInsertAuditLog(ctx context.Context, p *auditLogPayload) (any, error) {
	timestamp := int64(unparseableTimestampFallback)
	if ts, err := time.Parse(time.RFC3339, p.Timestamp); err == nil {
		timestamp = dbwriter.EpochMillis(ts)
	}

	log := &dbwriter.AuditLog{
		ID:        p.ID,
		Timestamp: timestamp,
		Level:     p.Level,
		Actor:     p.Actor,
		Action:    p.Action,
		IP:        p.IP,
		Details:   p.Details,
	}

	if _, err := db.runner(ctx).ExecContext(ctx, insertAuditLogSQL, log.ID, log.Timestamp, log.Level, log.Actor, log.Action, log.IP, log.Details); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateUser(ctx context.Context, u *User) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, createUserSQL, u.ID, u.Email, u.RoleID, u.HashedPassword)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return id, nil
}

func (db *Database) applyUpdateUser(ctx context.Context, u *User) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, editUserSQL, u.RoleID, u.Email)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyUpdateUserPassword(ctx context.Context, u *User) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, editUserPasswordSQL, u.HashedPassword, u.Email)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteUser(ctx context.Context, p *stringPayload) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, deleteUserSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyCreateProfile(ctx context.Context, p *Profile) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, createProfileSQL, p.ID, p.Name, p.UeAmbrUplink, p.UeAmbrDownlink, p.Allow4G, p.Allow5G)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateProfile(ctx context.Context, p *Profile) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, editProfileSQL, p.UeAmbrUplink, p.UeAmbrDownlink, p.Allow4G, p.Allow5G, p.Name)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteProfile(ctx context.Context, p *stringPayload) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, deleteProfileSQL, p.Value)
	if err != nil {
		if isForeignKeyRestrictError(err) {
			return nil, ErrInUse
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyCreateAPIToken(ctx context.Context, t *APIToken) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, createAPITokenSQL, t.ID, t.TokenID, t.Name, t.TokenHash, t.UserID, t.ExpiresAt)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAPIToken(ctx context.Context, p *stringPayload) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, deleteAPITokenSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateSession(ctx context.Context, s *Session) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, createSessionSQL, s.ID, s.UserID, s.TokenHash, s.CreatedAt, s.ExpiresAt); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteSessionByTokenHash(ctx context.Context, p *bytesPayload) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, deleteSessionByTokenHashSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteExpiredSessions(ctx context.Context, p *int64Payload) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, deleteExpiredSessionsSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	return int(count), nil
}

func (db *Database) applyDeleteOldestSessions(ctx context.Context, args *DeleteOldestArgs) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, deleteOldestSessionsSQL, args.UserID, args.Limit)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAllSessionsForUser(ctx context.Context, p *stringPayload) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, deleteAllSessionsForUserSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAllSessions(ctx context.Context) error {
	_, err := db.runner(ctx).ExecContext(ctx, deleteAllSessionsSQL)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyCreateNetworkSlice(ctx context.Context, s *NetworkSlice) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, createNetworkSliceSQL, s.ID, s.Sst, s.Sd, s.Name)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateNetworkSlice(ctx context.Context, s *NetworkSlice) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, editNetworkSliceSQL, s.Sst, s.Sd, s.Name)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteNetworkSlice(ctx context.Context, p *stringPayload) (any, error) {
	if err := db.deleteSliceDataNetworksOfSlice(ctx, p.Value); err != nil {
		return nil, err
	}

	res, err := db.runner(ctx).ExecContext(ctx, deleteNetworkSliceSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyCreateDataNetwork(ctx context.Context, dn *DataNetwork) (any, error) {
	current, err := db.dataNetworkSchemaCurrent(ctx, dn)
	if err != nil {
		return nil, err
	}

	query, args := createDataNetworkQuery(dn, current)

	_, err = db.runner(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateDataNetwork(ctx context.Context, dn *DataNetwork) (any, error) {
	current, err := db.dataNetworkSchemaCurrent(ctx, dn)
	if err != nil {
		return nil, err
	}

	query, args := editDataNetworkQuery(dn, current)

	res, err := db.runner(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteDataNetwork(ctx context.Context, p *stringPayload) (any, error) {
	if err := db.deleteSliceDataNetworksOfDataNetwork(ctx, p.Value); err != nil {
		return nil, err
	}

	res, err := db.runner(ctx).ExecContext(ctx, deleteDataNetworkSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyCreatePolicy(ctx context.Context, p *Policy) (any, error) {
	p.defaultPreemption()

	current, err := db.policySchemaCurrent(ctx, p)
	if err != nil {
		return nil, err
	}

	query, args := createPolicyQuery(p, current)

	_, err = db.runner(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

// applySetDefaultPolicy makes p the sole default binding of its profile: it
// clears every default in the profile, then sets p, in one transaction.
func (db *Database) applySetDefaultPolicy(ctx context.Context, p *Policy) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, clearDefaultPoliciesSQL, p.ProfileID); err != nil {
		return nil, fmt.Errorf("clear defaults: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, setDefaultPolicySQL, p.Name); err != nil {
		return nil, fmt.Errorf("set default: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdatePolicy(ctx context.Context, p *Policy) (any, error) {
	p.defaultPreemption()

	current, err := db.policySchemaCurrent(ctx, p)
	if err != nil {
		return nil, err
	}

	query, args := editPolicyQuery(p, current)

	res, err := db.runner(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeletePolicy(ctx context.Context, p *stringPayload) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, deletePolicySQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyCreateNetworkRule(ctx context.Context, nr *NetworkRule) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, createNetworkRuleSQL, nr.ID, nr.PolicyID, nr.Description, nr.Direction, nr.RemotePrefix, nr.Protocol, nr.PortLow, nr.PortHigh, nr.Action, nr.Precedence, nr.CreatedAt, nr.UpdatedAt); err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateNetworkRule(ctx context.Context, nr *NetworkRule) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, updateNetworkRuleSQL, nr.Description, nr.Direction, nr.RemotePrefix, nr.Protocol, nr.PortLow, nr.PortHigh, nr.Action, nr.Precedence, nr.UpdatedAt, nr.ID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteNetworkRule(ctx context.Context, p *stringPayload) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, deleteNetworkRuleSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteNetworkRulesByPolicy(ctx context.Context, p *stringPayload) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, deleteNetworkRulesByPolicySQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateHomeNetworkKey(ctx context.Context, k *HomeNetworkKey) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, createHomeNetworkKeySQL, k.ID, k.KeyIdentifier, k.Scheme, k.PrivateKey)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteHomeNetworkKey(ctx context.Context, p *stringPayload) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, deleteHomeNetworkKeySQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyCreateBGPPeer(ctx context.Context, p *BGPPeer) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, createBGPPeerSQL, p.Address, p.RemoteAS, p.HoldTime, p.Password, p.Description)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return int(id), nil
}

func (db *Database) applyUpdateBGPPeer(ctx context.Context, p *BGPPeer) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, updateBGPPeerSQL, p.Address, p.RemoteAS, p.HoldTime, p.Password, p.Description, p.ID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return struct{}{}, nil
}

func (db *Database) applyDeleteBGPPeer(ctx context.Context, p *intPayload) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, deleteBGPPeerSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return struct{}{}, nil
}

func (db *Database) applyUpdateBGPSettings(ctx context.Context, s *BGPSettings) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, upsertBGPSettingsSQL, s.Enabled, s.LocalAS, s.RouterID, s.ListenAddress)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return struct{}{}, nil
}

func (db *Database) applyUpdateN3Settings(ctx context.Context, p *stringPayload) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, upsertN3SettingsSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return struct{}{}, nil
}

func (db *Database) applyUpdateFlowAccountingSettings(ctx context.Context, p *boolPayload) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, upsertFlowAccountingSettingsSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return struct{}{}, nil
}

func (db *Database) applySetRetentionPolicy(ctx context.Context, rp *RetentionPolicy) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, upsertRetentionPolicySQL, rp.ID, string(rp.Category), rp.Days)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyInitializeOperator(ctx context.Context, op *Operator) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, initializeOperatorSQL, op.Mcc, op.Mnc, op.OperatorCode, op.SupportedTACs, op.Ciphering, op.Integrity, op.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorTracking(ctx context.Context, op *Operator) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, updateOperatorTrackingSQL, op.SupportedTACs)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorID(ctx context.Context, op *Operator) (any, error) {
	seppIdentityDeleted, err := db.prepareHomePLMNChange(ctx, op)
	if err != nil {
		return nil, err
	}

	from, err := db.imsDomain(ctx)
	if err != nil {
		return nil, err
	}

	_, err = db.runner(ctx).ExecContext(ctx, updateOperatorIDSQL, op.Mcc, op.Mnc)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	to, err := db.imsDomain(ctx)
	if err != nil {
		return nil, err
	}

	if err := db.rehomeIMSApplicationServers(ctx, from, to); err != nil {
		return nil, err
	}

	return seppIdentityDeleted, nil
}

func (db *Database) applyUpdateOperatorCode(ctx context.Context, op *Operator) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, updateOperatorCodeSQL, op.OperatorCode)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorSecurityAlgorithms(ctx context.Context, op *Operator) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, updateOperatorSecurityAlgorithmsSQL, op.Ciphering, op.Integrity)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorSPN(ctx context.Context, op *Operator) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, updateOperatorSPNSQL, op.SpnFullName, op.SpnShortName)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorAMFIdentity(ctx context.Context, op *Operator) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, updateOperatorAMFIdentitySQL, op.AmfRegionID, op.AmfSetID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorClusterID(ctx context.Context, op *Operator) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, updateOperatorClusterIDSQL, op.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applySetJWTSecret(ctx context.Context, p *bytesPayload) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, upsertJWTSecretSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyRotateJWTSecret(ctx context.Context, p *bytesPayload) error {
	if _, err := db.runner(ctx).ExecContext(ctx, upsertJWTSecretSQL, p.Value); err != nil {
		return fmt.Errorf("upsert secret: %w", err)
	}

	if err := db.applyDeleteAllSessions(ctx); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}

	return nil
}

func (db *Database) applyPutCellPosition(ctx context.Context, c *CellPosition) error {
	if _, err := db.runner(ctx).ExecContext(ctx, deleteCellPositionByCellSQL, c.ID, c.RAT, c.Mcc, c.Mnc, c.CellIdentity); err != nil {
		return fmt.Errorf("delete conflicting cell position: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, putCellPositionSQL, c.ID, c.RAT, c.Mcc, c.Mnc, c.CellIdentity, c.GNbID, c.Latitude, c.Longitude, c.Altitude, c.UncertaintySemiMajor, c.UncertaintySemiMinor, c.OrientationMajor, c.Confidence, c.Source, c.CreatedAt, c.UpdatedAt); err != nil {
		return fmt.Errorf("put cell position: %w", err)
	}

	return nil
}

func (db *Database) applyDeleteCellPosition(ctx context.Context, p *stringPayload) error {
	if _, err := db.runner(ctx).ExecContext(ctx, deleteCellPositionSQL, p.Value); err != nil {
		return fmt.Errorf("delete cell position: %w", err)
	}

	return nil
}

func (db *Database) requireCellPosition(ctx context.Context, id string) error {
	if _, err := scanCellPosition(db.runner(ctx).QueryRowContext(ctx, getCellPositionSQL, id)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}

		return fmt.Errorf("get cell position: %w", err)
	}

	return nil
}

func (db *Database) cellPositionTakenByOther(ctx context.Context, c *CellPosition) (bool, error) {
	existing, err := scanCellPosition(db.runner(ctx).QueryRowContext(ctx, getCellPositionByCellSQL, c.RAT, c.Mcc, c.Mnc, c.CellIdentity))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, fmt.Errorf("get cell position by cell: %w", err)
	}

	return existing.ID != c.ID, nil
}

func (db *Database) checkCreateCellPosition(ctx context.Context, c *CellPosition) error {
	if err := db.requireCellPosition(ctx, c.ID); !errors.Is(err, ErrNotFound) {
		if err == nil {
			return ErrAlreadyExists
		}

		return err
	}

	taken, err := db.cellPositionTakenByOther(ctx, c)
	if err != nil {
		return err
	}

	if taken {
		return ErrAlreadyExists
	}

	return nil
}

func (db *Database) checkUpdateCellPosition(ctx context.Context, c *CellPosition) error {
	if err := db.requireCellPosition(ctx, c.ID); err != nil {
		return err
	}

	taken, err := db.cellPositionTakenByOther(ctx, c)
	if err != nil {
		return err
	}

	if taken {
		return ErrAlreadyExists
	}

	return nil
}

func (db *Database) checkDeleteCellPosition(ctx context.Context, p *stringPayload) error {
	return db.requireCellPosition(ctx, p.Value)
}

func (db *Database) applyCreateRoute(ctx context.Context, r *Route) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, createRouteSQL, r.Destination, r.Gateway, r.Interface, r.Metric)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("retrieving insert ID failed: %w", err)
	}

	return id, nil
}

func (db *Database) applyDeleteRoute(ctx context.Context, p *int64Payload) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, deleteRouteSQL, p.Value)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return struct{}{}, nil
}

// applyUpsertClusterMember allocates the member's AMF Pointer and writes
// the row. The caller does not pre-resolve the pointer: like
// applyAllocateIPLease, the leader picks it here, inside
// leaderCaptureAndPropose's proposeMu, so concurrent joins see a stable
// view and the chosen value replicates in the changeset bytes.
//
// resolveAMFPointer returns any pointer the member already holds, so the
// write-back below is a no-op for an existing row. That ordering is what
// makes amfPointer=excluded.amfPointer safe in the conflict clause;
// reversing it would renumber live nodes.
func (db *Database) applyUpsertClusterMember(ctx context.Context, m *ClusterMember) (any, error) {
	if !db.appliedSchemaAtLeast(ctx, clusterMemberIdentitySchema) {
		if err := db.requireIdentitySchemaFor(ctx, m.NodeID); err != nil {
			return nil, err
		}

		if _, err := db.runner(ctx).ExecContext(ctx, upsertClusterMemberPreV20SQL, m.NodeID, m.APIAddress, m.BinaryVersion, m.APIAddress, m.BinaryVersion); err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}

		return nil, nil
	}

	pointer, err := db.resolveAMFPointer(ctx, m.NodeID)
	if err != nil {
		return nil, err
	}

	m.AMFPointer = pointer

	if _, err := db.runner(ctx).ExecContext(ctx, upsertClusterMemberSQL, m.NodeID, m.AMFPointer, m.DisplayName, m.APIAddress, m.BinaryVersion, m.APIAddress, m.BinaryVersion); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteClusterMember(ctx context.Context, p *nodeIDPayload) (any, error) {
	seppID := 0

	if db.appliedSchemaAtLeast(ctx, roamingSchema) {
		member, err := scanClusterMember(db.runner(ctx).QueryRowContext(ctx, getClusterMemberSQL, string(p.Value)))
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("query failed: %w", err)
		}

		seppID = member.AMFPointer
	}

	res, err := db.runner(ctx).ExecContext(ctx, deleteClusterMemberSQL, string(p.Value))
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	if err := db.purgeUERegistrationsByNode(ctx, string(p.Value)); err != nil {
		return nil, err
	}

	if db.appliedSchemaAtLeast(ctx, roamingSchema) {
		if err := db.deleteUnregisteredVisitors(ctx); err != nil {
			return nil, err
		}

		if ValidSEPPID(seppID) {
			if err := db.deleteGeneratedSEPPIdentity(ctx, seppID); err != nil {
				return nil, err
			}
		}
	}

	return nil, nil
}

func (db *Database) applySetDisplayName(ctx context.Context, m *ClusterMember) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, setDisplayNameSQL, m.DisplayName, m.NodeID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applySetDrainState(ctx context.Context, m *ClusterMember) (any, error) {
	var current ClusterMember

	err := db.runner(ctx).QueryRowContext(ctx, getDrainStateSQL, m.NodeID).Scan(&current.DrainState)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("read drain state: %w", err)
	}

	if !drainTransitionAllowed(current.DrainState, m.DrainState) {
		return normalizeDrainState(current.DrainState), nil
	}

	res, err := db.runner(ctx).ExecContext(ctx, setDrainStateSQL, m.DrainState, m.DrainUpdatedAt, m.NodeID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return m.DrainState, nil
}

func (db *Database) applyMigrateShared(ctx context.Context, p *migrateSharedPayload) (any, error) {
	idx := p.TargetVersion - 1
	if idx < 0 || idx >= len(migrations) {
		// The leader's proposal gate (CheckPendingMigrations) normally
		// prevents this path. Reaching it means a laggard voter is being
		// asked to apply a migration beyond its binary — the fail-stop
		// FSM will panic. Keep the message informative: it's likely to
		// be the last thing the operator sees before a node restart.
		return nil, fmt.Errorf(
			"refusing to apply migration v%d: this binary supports up to v%d (rolling upgrade skew)",
			p.TargetVersion, SchemaVersion())
	}

	m := migrations[idx]
	if m.version != p.TargetVersion {
		return nil, fmt.Errorf("migration registry mismatch: expected version %d at index %d, got %d", p.TargetVersion, idx, m.version)
	}

	sqlConn := db.writer()

	var current int
	if err := sqlConn.QueryRowContext(ctx, "SELECT version FROM schema_version WHERE id = 1").Scan(&current); err != nil {
		return nil, fmt.Errorf("read schema_version before migration %d: %w", p.TargetVersion, err)
	}

	// Replay-safe: if this migration was already applied (e.g. via a log entry
	// replayed after a stale snapshot), skip re-running the non-idempotent
	// migration body.
	if current >= p.TargetVersion {
		return nil, nil
	}

	if current != p.TargetVersion-1 {
		return nil, fmt.Errorf("out-of-order migration: current=%d target=%d", current, p.TargetVersion)
	}

	if _, err := sqlConn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return nil, fmt.Errorf("disable foreign keys for migration %d: %w", p.TargetVersion, err)
	}

	defer func() {
		_, _ = sqlConn.ExecContext(context.Background(), "PRAGMA foreign_keys = ON")
	}()

	tx, err := sqlConn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin migration %d tx: %w", p.TargetVersion, err)
	}

	if err := m.fn(withMigrationToggles(ctx, p.LocalSwitch, p.NAT), tx); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("shared migration %d (%s) failed: %w", m.version, m.description, err)
	}

	if _, err := tx.ExecContext(ctx, "UPDATE schema_version SET version = ? WHERE id = 1", p.TargetVersion); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("update schema_version to %d: %w", p.TargetVersion, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit migration %d: %w", p.TargetVersion, err)
	}

	return nil, nil
}
