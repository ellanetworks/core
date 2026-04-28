// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/ipam"
	"github.com/ellanetworks/core/internal/logger"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	hraft "github.com/hashicorp/raft"
	"go.uber.org/zap"
)

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
func (db *Database) ApplyCommand(ctx context.Context, cmd *ellaraft.Command, logIndex uint64) (any, error) {
	switch cmd.Type {
	case ellaraft.CmdChangeset:
		payload, err := unmarshalPayload[bytesPayload](cmd.Payload)
		if err != nil {
			return nil, err
		}

		if err := db.assertAppliedSchema(ctx, payload.RequiredSchema, fmt.Sprintf("changeset %q", payload.Operation)); err != nil {
			return nil, err
		}

		result, applyErr := db.applyChangeset(ctx, payload)
		if applyErr == nil {
			if payload.Operation == "UpsertClusterMember" {
				db.signalMigrationCheck()
			}

			db.publishOpTopics(topicsForChangesetOp(payload.Operation), logIndex)
		}

		return result, applyErr

	case ellaraft.CmdDeleteOldAuditLogs:
		if err := db.assertAppliedSchema(ctx, intentMinSchemaForCmd(cmd.Type), cmd.Type.String()); err != nil {
			return nil, err
		}

		payload, err := unmarshalPayload[stringPayload](cmd.Payload)
		if err != nil {
			return nil, err
		}

		applyErr := db.applyDeleteOldAuditLogs(ctx, payload)
		if applyErr == nil {
			db.publishOpTopics(topicsForIntentCmd(cmd.Type), logIndex)
		}

		return nil, applyErr

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
			db.publishOpTopics(topicsForIntentCmd(cmd.Type), logIndex)
		}

		return nil, applyErr

	case ellaraft.CmdDeleteAllDynamicLeases:
		if err := db.assertAppliedSchema(ctx, intentMinSchemaForCmd(cmd.Type), cmd.Type.String()); err != nil {
			return nil, err
		}

		applyErr := db.applyDeleteAllDynamicLeases(ctx)
		if applyErr == nil {
			db.publishOpTopics(topicsForIntentCmd(cmd.Type), logIndex)
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
			db.publishOpTopics(topicsForIntentCmd(cmd.Type), logIndex)
		}

		return result, applyErr

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
func (db *Database) publishOpTopics(topics []Topic, index uint64) {
	if db.changefeed == nil || len(topics) == 0 {
		return
	}

	for _, t := range topics {
		db.changefeed.Publish(t, index)
	}
}

// PlainDB returns the raw *sql.DB for the application database.
func (db *Database) PlainDB() *sql.DB {
	return db.conn().PlainDB()
}

// Reopen closes the current database connection, opens a fresh one, runs
// migrations, and re-prepares all sqlair statements. Reopen is only invoked
// from FSM.Restore which holds the FSM write lock, so concurrent applies and
// snapshots are already serialised; the atomic swap of db.connPtr protects
// ad-hoc read sites (API handlers) from observing a torn pointer.
func (db *Database) Reopen(ctx context.Context) error {
	old := db.conn()

	sqlConn, err := openSQLiteConnection(ctx, db.Path())
	if err != nil {
		return fmt.Errorf("reopen database: %w", err)
	}

	maxVersion := 0
	if db.raftManager != nil && db.raftManager.ClusterEnabled() {
		// In cluster mode, restore/reopen must track the snapshot baseline.
		// Post-baseline shared migrations are proposed by the leader via Raft.
		maxVersion = baselineVersion
	}

	if err := runMigrations(ctx, sqlConn, maxVersion); err != nil {
		_ = sqlConn.Close()
		return fmt.Errorf("migrations after reopen: %w", err)
	}

	if err := ensureFsmStateTable(ctx, sqlConn); err != nil {
		_ = sqlConn.Close()
		return fmt.Errorf("ensure fsm_state after reopen: %w", err)
	}

	db.connPtr.Store(sqlair.NewDB(sqlConn))

	if old != nil {
		_ = old.PlainDB().Close()
	}

	if err := db.refreshAppliedSchema(ctx); err != nil {
		return fmt.Errorf("refresh applied-schema cache after reopen: %w", err)
	}

	if err := db.PrepareStatements(); err != nil {
		return fmt.Errorf("re-prepare statements: %w", err)
	}

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
	}
	auditLogPayload struct {
		Timestamp string `json:"timestamp"`
		Level     string `json:"level"`
		Actor     string `json:"actor"`
		Action    string `json:"action"`
		IP        string `json:"ip"`
		Details   string `json:"details"`
	}
	importPrefixesPayload struct {
		PeerID   int               `json:"peer_id"`
		Prefixes []BGPImportPrefix `json:"prefixes"`
	}
	migrateSharedPayload struct {
		TargetVersion int `json:"targetVersion"`
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
		errors.Is(err, hraft.ErrLeadershipLost) ||
		errors.Is(err, hraft.ErrRaftShutdown)
}

// --- Apply functions ---
// Each applyX function executes the actual SQL against the shared database.
// These are called both in standalone mode (directly) and in HA mode (via FSM).
// They contain no tracing or metrics — the propose layer handles those.

func (db *Database) applyCreateSubscriber(ctx context.Context, s *Subscriber) (any, error) {
	err := db.runner(ctx).Query(ctx, db.createSubscriberStmt, s).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateSubscriberProfile(ctx context.Context, s *Subscriber) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.updateSubscriberProfileStmt, s).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyEditSubscriberSeqNum(ctx context.Context, s *Subscriber) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.updateSubscriberSqnNumStmt, s).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteSubscriber(ctx context.Context, p *stringPayload) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.deleteSubscriberStmt, Subscriber{Imsi: p.Value}).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyIncrementDailyUsage(ctx context.Context, du *DailyUsage) (any, error) {
	err := db.runner(ctx).Query(ctx, db.incrementDailyUsageStmt, du).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyClearDailyUsage(ctx context.Context) error {
	err := db.runner(ctx).Query(ctx, db.deleteAllDailyUsageStmt).Run()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyDeleteOldDailyUsage(ctx context.Context, p *int64Payload) error {
	err := db.runner(ctx).Query(ctx, db.deleteOldDailyUsageStmt, cutoffDaysArgs{CutoffDays: p.Value}).Run()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyCreateLease(ctx context.Context, lease *IPLease) (any, error) {
	err := db.runner(ctx).Query(ctx, db.createLeaseStmt, lease).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateLeaseSession(ctx context.Context, lease *IPLease) (any, error) {
	err := db.runner(ctx).Query(ctx, db.updateLeaseSessionStmt, lease).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteDynamicLease(ctx context.Context, p *intPayload) (any, error) {
	err := db.runner(ctx).Query(ctx, db.deleteLeaseStmt, IPLease{ID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAllDynamicLeases(ctx context.Context) error {
	err := db.runner(ctx).Query(ctx, db.deleteAllDynamicLeasesStmt).Run()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyDeleteDynamicLeasesByNode(ctx context.Context, p *intPayload) (any, error) {
	err := db.runner(ctx).Query(ctx, db.deleteDynLeasesByNodeStmt, IPLease{NodeID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateLeaseNode(ctx context.Context, lease *IPLease) (any, error) {
	err := db.runner(ctx).Query(ctx, db.updateLeaseNodeStmt, lease).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

// allocateIPLeasePayload is the wire payload for AllocateIPLease. The
// caller does not pre-resolve the address — that is the whole point of
// this op. The leader's apply function picks the address atomically
// inside leaderCaptureAndPropose's proposeMu.
type allocateIPLeasePayload struct {
	PoolID    int    `json:"poolId"`
	IMSI      string `json:"imsi"`
	SessionID int    `json:"sessionId"`
	NodeID    int    `json:"nodeId"`
}

// applyAllocateIPLease atomically resolves an IP for (poolID, IMSI,
// sessionID, nodeID) and inserts the lease. Runs under
// leaderCaptureAndPropose's proposeMu and inside captureChangeset's
// pinned connection, so the merge-scan for a free address and the
// INSERT are serialised against every other replicated write — no
// other allocation can race in between.
//
// Every query and write goes through db.runner(ctx) so they target the
// pinned SQLite connection set up by applyWithPinnedConn. Calling the
// public DB methods (GetDataNetworkByID, GetDynamicLease, etc.) here
// would dispatch to db.conn() — the shared pool with MaxOpenConns=1
// whose only connection is already held by the active capture, and
// every such SELECT would deadlock until the proposeTimeout context
// fires.
//
// Replaces the SequentialAllocator path that had each follower locally
// pick a free IP and forward only the INSERT: under concurrency, two
// followers could pick the same offset from a stale local view; the
// second INSERT then collided at the leader's unique constraint and
// surfaced as "capture sqlite changeset: already exists" with no retry
// because the ipam.ErrAlreadyExists sentinel did not survive the
// proxy boundary.
func (db *Database) applyAllocateIPLease(ctx context.Context, p *allocateIPLeasePayload) (any, error) {
	if p.IMSI == "" {
		return nil, fmt.Errorf("IMSI required")
	}

	runner := db.runner(ctx)

	// Resolve the pool's CIDR. Inlined query against the pinned runner;
	// see function comment.
	dn := DataNetwork{ID: p.PoolID}

	if err := runner.Query(ctx, db.getDataNetworkByIDStmt, dn).Get(&dn); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("data network %d not found", p.PoolID)
		}

		return nil, fmt.Errorf("get data network %d: %w", p.PoolID, err)
	}

	pool, err := ipam.NewPool(dn.ID, dn.IPPool)
	if err != nil {
		return nil, fmt.Errorf("parse pool %q: %w", dn.IPPool, err)
	}

	sessionID := p.SessionID

	// Step 1: existing dynamic lease (re-registration). Update the
	// owning node and session and return its address.
	existing := IPLease{PoolID: p.PoolID, IMSI: p.IMSI}

	err = runner.Query(ctx, db.getDynamicLeaseStmt, existing).Get(&existing)
	switch {
	case err == nil:
		existing.SessionID = &sessionID
		if existing.NodeID != p.NodeID {
			existing.NodeID = p.NodeID
			if _, applyErr := db.applyUpdateLeaseNode(ctx, &existing); applyErr != nil {
				return nil, fmt.Errorf("update lease node: %w", applyErr)
			}
		} else {
			if _, applyErr := db.applyUpdateLeaseSession(ctx, &existing); applyErr != nil {
				return nil, fmt.Errorf("update lease session: %w", applyErr)
			}
		}

		return existing.Address().String(), nil
	case errors.Is(err, sql.ErrNoRows):
		// fall through to fresh allocation
	default:
		return nil, fmt.Errorf("get dynamic lease: %w", err)
	}

	// Step 2: list every allocated address in this pool and convert
	// to sorted offsets so the merge-scan can skip them in O(N).
	var leases []IPLease

	err = runner.Query(ctx, db.listLeaseAddressesByPoolStmt, IPLease{PoolID: p.PoolID}).GetAll(&leases)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("list lease addresses: %w", err)
	}

	allocated := make([]int, 0, len(leases))

	for i := range leases {
		offset := pool.OffsetOf(leases[i].Address())
		if offset >= 0 {
			allocated = append(allocated, offset)
		}
	}

	sort.Ints(allocated)

	// Step 3: merge-scan. Under proposeMu the SELECT view is stable, so
	// the first free offset we find is also free at INSERT time. The
	// inner ErrAlreadyExists branch is defensive only — it should be
	// unreachable while proposeMu is held.
	poolSize := pool.Size()
	firstUsable := pool.FirstUsable()
	allocIdx := 0
	now := time.Now().Unix()

	for offset := firstUsable; offset < firstUsable+poolSize; offset++ {
		for allocIdx < len(allocated) && allocated[allocIdx] < offset {
			allocIdx++
		}

		if allocIdx < len(allocated) && allocated[allocIdx] == offset {
			continue
		}

		addr := pool.AddressAtOffset(offset)
		bin := addr.As16()
		lease := &IPLease{
			PoolID:     p.PoolID,
			AddressBin: bin[:],
			IMSI:       p.IMSI,
			SessionID:  &sessionID,
			Type:       "dynamic",
			CreatedAt:  now,
			NodeID:     p.NodeID,
		}

		if _, applyErr := db.applyCreateLease(ctx, lease); applyErr != nil {
			if errors.Is(applyErr, ErrAlreadyExists) {
				continue
			}

			return nil, fmt.Errorf("create lease: %w", applyErr)
		}

		return addr.String(), nil
	}

	return nil, ipam.ErrPoolExhausted
}

func (db *Database) applyInsertAuditLog(ctx context.Context, p *auditLogPayload) (any, error) {
	log := &dbwriter.AuditLog{
		Timestamp: p.Timestamp,
		Level:     p.Level,
		Actor:     p.Actor,
		Action:    p.Action,
		IP:        p.IP,
		Details:   p.Details,
	}

	err := db.runner(ctx).Query(ctx, db.insertAuditLogStmt, log).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteOldAuditLogs(ctx context.Context, p *stringPayload) error {
	err := db.runner(ctx).Query(ctx, db.deleteOldAuditLogsStmt, cutoffArgs{Cutoff: p.Value}).Run()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyCreateUser(ctx context.Context, u *User) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.createUserStmt, u).Get(&outcome)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	id, err := outcome.Result().LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return id, nil
}

func (db *Database) applyUpdateUser(ctx context.Context, u *User) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.editUserStmt, u).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyUpdateUserPassword(ctx context.Context, u *User) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.editUserPasswordStmt, u).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteUser(ctx context.Context, p *stringPayload) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.deleteUserStmt, User{Email: p.Value}).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyCreateProfile(ctx context.Context, p *Profile) (any, error) {
	err := db.runner(ctx).Query(ctx, db.createProfileStmt, p).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateProfile(ctx context.Context, p *Profile) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.editProfileStmt, p).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteProfile(ctx context.Context, p *stringPayload) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.deleteProfileStmt, Profile{Name: p.Value}).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyCreateAPIToken(ctx context.Context, t *APIToken) (any, error) {
	err := db.runner(ctx).Query(ctx, db.createAPITokenStmt, t).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAPIToken(ctx context.Context, p *intPayload) (any, error) {
	err := db.runner(ctx).Query(ctx, db.deleteAPITokenStmt, APIToken{ID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateSession(ctx context.Context, s *Session) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.createSessionStmt, s).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	id, err := outcome.Result().LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return id, nil
}

func (db *Database) applyDeleteSessionByTokenHash(ctx context.Context, p *bytesPayload) (any, error) {
	err := db.runner(ctx).Query(ctx, db.deleteSessionByTokenHashStmt, Session{TokenHash: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteExpiredSessions(ctx context.Context, p *int64Payload) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.deleteExpiredSessionsStmt, SessionCutoff{NowUnix: p.Value}).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	count, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	return int(count), nil
}

func (db *Database) applyDeleteOldestSessions(ctx context.Context, args *DeleteOldestArgs) (any, error) {
	err := db.runner(ctx).Query(ctx, db.deleteOldestSessionsStmt, args).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAllSessionsForUser(ctx context.Context, p *int64Payload) (any, error) {
	err := db.runner(ctx).Query(ctx, db.deleteAllSessionsForUserStmt, UserIDArgs{UserID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAllSessions(ctx context.Context) error {
	err := db.runner(ctx).Query(ctx, db.deleteAllSessionsStmt).Run()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyCreateNetworkSlice(ctx context.Context, s *NetworkSlice) (any, error) {
	err := db.runner(ctx).Query(ctx, db.createNetworkSliceStmt, s).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateNetworkSlice(ctx context.Context, s *NetworkSlice) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.editNetworkSliceStmt, s).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteNetworkSlice(ctx context.Context, p *stringPayload) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.deleteNetworkSliceStmt, NetworkSlice{Name: p.Value}).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyCreateDataNetwork(ctx context.Context, dn *DataNetwork) (any, error) {
	err := db.runner(ctx).Query(ctx, db.createDataNetworkStmt, dn).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateDataNetwork(ctx context.Context, dn *DataNetwork) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.editDataNetworkStmt, dn).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteDataNetwork(ctx context.Context, p *stringPayload) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.deleteDataNetworkStmt, DataNetwork{Name: p.Value}).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyCreatePolicy(ctx context.Context, p *Policy) (any, error) {
	err := db.runner(ctx).Query(ctx, db.createPolicyStmt, p).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdatePolicy(ctx context.Context, p *Policy) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.editPolicyStmt, p).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeletePolicy(ctx context.Context, p *stringPayload) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.deletePolicyStmt, Policy{Name: p.Value}).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyCreateNetworkRule(ctx context.Context, nr *NetworkRule) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.createNetworkRuleStmt, nr).Get(&outcome)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	id, err := outcome.Result().LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return id, nil
}

func (db *Database) applyUpdateNetworkRule(ctx context.Context, nr *NetworkRule) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.updateNetworkRuleStmt, nr).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteNetworkRule(ctx context.Context, p *int64Payload) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.deleteNetworkRuleStmt, NetworkRule{ID: p.Value}).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteNetworkRulesByPolicy(ctx context.Context, p *int64Payload) (any, error) {
	err := db.runner(ctx).Query(ctx, db.deleteNetworkRulesByPolicyStmt, NetworkRule{PolicyID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateHomeNetworkKey(ctx context.Context, k *HomeNetworkKey) (any, error) {
	err := db.runner(ctx).Query(ctx, db.createHomeNetworkKeyStmt, k).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteHomeNetworkKey(ctx context.Context, p *intPayload) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.deleteHomeNetworkKeyStmt, HomeNetworkKey{ID: p.Value}).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyCreateBGPPeer(ctx context.Context, p *BGPPeer) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.createBGPPeerStmt, p).Get(&outcome)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	id, err := outcome.Result().LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return int(id), nil
}

func (db *Database) applyUpdateBGPPeer(ctx context.Context, p *BGPPeer) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.updateBGPPeerStmt, p).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteBGPPeer(ctx context.Context, p *intPayload) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.deleteBGPPeerStmt, BGPPeer{ID: p.Value}).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyUpdateBGPSettings(ctx context.Context, s *BGPSettings) (any, error) {
	err := db.runner(ctx).Query(ctx, db.upsertBGPSettingsStmt, s).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applySetImportPrefixesForPeer(ctx context.Context, p *importPrefixesPayload) (any, error) {
	// Runs inside the capture transaction opened by captureChangeset, so no
	// nested Begin is needed here.
	runner := db.runner(ctx)

	if err := runner.Query(ctx, db.deleteImportPrefixesByPeerStmt, BGPImportPrefix{PeerID: p.PeerID}).Run(); err != nil {
		return nil, fmt.Errorf("delete existing prefixes: %w", err)
	}

	for _, prefix := range p.Prefixes {
		prefix.PeerID = p.PeerID

		if err := runner.Query(ctx, db.createImportPrefixStmt, prefix).Run(); err != nil {
			return nil, fmt.Errorf("insert prefix: %w", err)
		}
	}

	return nil, nil
}

func (db *Database) applyUpdateNATSettings(ctx context.Context, p *boolPayload) (any, error) {
	err := db.runner(ctx).Query(ctx, db.upsertNATSettingsStmt, NATSettings{Enabled: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateN3Settings(ctx context.Context, p *stringPayload) (any, error) {
	err := db.runner(ctx).Query(ctx, db.updateN3SettingsStmt, N3Settings{ExternalAddress: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateFlowAccountingSettings(ctx context.Context, p *boolPayload) (any, error) {
	err := db.runner(ctx).Query(ctx, db.upsertFlowAccountingSettingsStmt, FlowAccountingSettings{Enabled: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applySetRetentionPolicy(ctx context.Context, rp *RetentionPolicy) (any, error) {
	err := db.runner(ctx).Query(ctx, db.upsertRetentionPolicyStmt, rp).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyInitializeOperator(ctx context.Context, op *Operator) (any, error) {
	err := db.runner(ctx).Query(ctx, db.initializeOperatorStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorTracking(ctx context.Context, op *Operator) (any, error) {
	err := db.runner(ctx).Query(ctx, db.updateOperatorTrackingStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorID(ctx context.Context, op *Operator) (any, error) {
	err := db.runner(ctx).Query(ctx, db.updateOperatorIDStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorCode(ctx context.Context, op *Operator) (any, error) {
	err := db.runner(ctx).Query(ctx, db.updateOperatorCodeStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorSecurityAlgorithms(ctx context.Context, op *Operator) (any, error) {
	err := db.runner(ctx).Query(ctx, db.updateOperatorSecurityAlgorithmsStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorSPN(ctx context.Context, op *Operator) (any, error) {
	err := db.runner(ctx).Query(ctx, db.updateOperatorSPNStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorAMFIdentity(ctx context.Context, op *Operator) (any, error) {
	err := db.runner(ctx).Query(ctx, db.updateOperatorAMFIdentityStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorClusterID(ctx context.Context, op *Operator) (any, error) {
	err := db.runner(ctx).Query(ctx, db.updateOperatorClusterIDStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applySetJWTSecret(ctx context.Context, p *bytesPayload) (any, error) {
	err := db.runner(ctx).Query(ctx, db.upsertJWTSecretStmt, JWTSecret{Secret: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateRoute(ctx context.Context, r *Route) (any, error) {
	err := db.runner(ctx).Query(ctx, db.createRouteStmt, r).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteRoute(ctx context.Context, p *int64Payload) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.deleteRouteStmt, Route{ID: p.Value}).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyUpsertClusterMember(ctx context.Context, m *ClusterMember) (any, error) {
	err := db.runner(ctx).Query(ctx, db.upsertClusterMemberStmt, m).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteClusterMember(ctx context.Context, p *intPayload) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.deleteClusterMemberStmt, ClusterMember{NodeID: p.Value}).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applySetDrainState(ctx context.Context, m *ClusterMember) (any, error) {
	var outcome sqlair.Outcome

	err := db.runner(ctx).Query(ctx, db.setDrainStateStmt, m).Get(&outcome)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
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

	sqlConn := db.conn().PlainDB()

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

	if err := m.fn(ctx, tx); err != nil {
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
