// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/dbwriter"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	hraft "github.com/hashicorp/raft"
)

// Compile-time check that *Database implements ellaraft.Applier.
var _ ellaraft.Applier = (*Database)(nil)

// ApplyCommand dispatches a Raft command to the appropriate applyX method.
// Called by the FSM on every node (leader and followers) for each committed
// log entry. Each applyX uses sqlair to execute SQL against the shared database.
func (db *Database) ApplyCommand(ctx context.Context, cmd *ellaraft.Command) (any, error) {
	switch cmd.Type {
	case ellaraft.CmdChangeset:
		payload, err := unmarshalPayload[bytesPayload](cmd.Payload)
		if err != nil {
			return nil, err
		}

		return db.applyChangeset(ctx, payload)

	case ellaraft.CmdDeleteOldAuditLogs:
		payload, err := unmarshalPayload[stringPayload](cmd.Payload)
		if err != nil {
			return nil, err
		}

		return db.applyDeleteOldAuditLogs(ctx, payload)

	case ellaraft.CmdDeleteOldDailyUsage:
		payload, err := unmarshalPayload[int64Payload](cmd.Payload)
		if err != nil {
			return nil, err
		}

		return db.applyDeleteOldDailyUsage(ctx, payload)

	case ellaraft.CmdDeleteAllDynamicLeases:
		return nil, db.applyDeleteAllDynamicLeases(ctx)

	case ellaraft.CmdDeleteExpiredSessions:
		payload, err := unmarshalPayload[int64Payload](cmd.Payload)
		if err != nil {
			return nil, err
		}

		return db.applyDeleteExpiredSessions(ctx, payload)

	case ellaraft.CmdMigrateShared:
		payload, err := unmarshalPayload[migrateSharedPayload](cmd.Payload)
		if err != nil {
			return nil, err
		}

		return db.applyMigrateShared(ctx, payload)

	case ellaraft.CmdRestore:
		payload, err := unmarshalPayload[bytesPayload](cmd.Payload)
		if err != nil {
			return nil, err
		}

		return db.applyRestore(ctx, payload)

	default:
		return nil, fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// PlainDB returns the raw *sql.DB for the application database.
func (db *Database) PlainDB() *sql.DB {
	return db.conn.PlainDB()
}

// Reopen closes the current database connection, opens a fresh one, runs
// migrations, and re-prepares all sqlair statements.
func (db *Database) Reopen(ctx context.Context) error {
	if db.conn != nil {
		_ = db.conn.PlainDB().Close()
	}

	sqlConn, err := openSQLiteConnection(ctx, db.Path())
	if err != nil {
		return fmt.Errorf("reopen database: %w", err)
	}

	// In cluster mode, restore/reopen must track the snapshot baseline.
	// Post-baseline shared migrations are proposed by the leader via Raft.
	if db.raftManager != nil && db.raftManager.ClusterEnabled() {
		if err := runMigrationsUpTo(ctx, sqlConn, baselineVersion); err != nil {
			_ = sqlConn.Close()
			return fmt.Errorf("migrations after reopen: %w", err)
		}
	} else {
		if err := runMigrations(ctx, sqlConn); err != nil {
			_ = sqlConn.Close()
			return fmt.Errorf("migrations after reopen: %w", err)
		}
	}

	db.conn = sqlair.NewDB(sqlConn)

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
		Value []byte `json:"value"`
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

// proposeChangeset captures a local SQL mutation as a sqlite changeset and
// replicates it through Raft as CmdChangeset. The leader's SQL writes are
// rolled back locally and only become durable through committed apply.
func (db *Database) proposeChangeset(applyFn func(context.Context) (any, error), operation string) (any, error) {
	changeset, applyResult, err := db.captureChangeset(context.Background(), applyFn, operation)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			return nil, ErrAlreadyExists
		}

		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("capture changeset for %s: %w", operation, err)
	}

	changesetCmd, err := ellaraft.NewCommand(ellaraft.CmdChangeset, &bytesPayload{Value: changeset})
	if err != nil {
		return nil, err
	}

	_, err = db.raftManager.Propose(changesetCmd, db.proposeTimeout)
	if err != nil {
		if isTransientRaftErr(err) {
			return nil, fmt.Errorf("%w: %v", ErrProposeTimeout, err)
		}

		return nil, err
	}

	return applyResult, nil
}

// proposeIntent proposes non-changeset commands that intentionally remain
// explicit in the Raft log (bulk retention deletes, migrations, restore).
func (db *Database) proposeIntent(cmdType ellaraft.CommandType, payload any) (any, error) {
	cmd, err := ellaraft.NewCommand(cmdType, payload)
	if err != nil {
		return nil, err
	}

	result, err := db.raftManager.Propose(cmd, db.proposeTimeout)
	if err != nil {
		if isTransientRaftErr(err) {
			return nil, fmt.Errorf("%w: %v", ErrProposeTimeout, err)
		}

		return nil, err
	}

	return result, nil
}

// isTransientRaftErr reports whether a Raft apply error is transient —
// the caller should retry or surface a 503.
func isTransientRaftErr(err error) bool {
	return errors.Is(err, hraft.ErrEnqueueTimeout) ||
		errors.Is(err, hraft.ErrLeadershipLost) ||
		errors.Is(err, hraft.ErrRaftShutdown) ||
		errors.Is(err, hraft.ErrNotLeader)
}

// --- Apply functions ---
// Each applyX function executes the actual SQL against the shared database.
// These are called both in standalone mode (directly) and in HA mode (via FSM).
// They contain no tracing or metrics — the propose layer handles those.

func (db *Database) applyCreateSubscriber(ctx context.Context, s *Subscriber) (any, error) {
	err := db.conn.Query(ctx, db.createSubscriberStmt, s).Run()
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

	err := db.conn.Query(ctx, db.updateSubscriberProfileStmt, s).Get(&outcome)
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

	err := db.conn.Query(ctx, db.updateSubscriberSqnNumStmt, s).Get(&outcome)
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

	err := db.conn.Query(ctx, db.deleteSubscriberStmt, Subscriber{Imsi: p.Value}).Get(&outcome)
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
	err := db.conn.Query(ctx, db.incrementDailyUsageStmt, du).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyClearDailyUsage(ctx context.Context) error {
	err := db.conn.Query(ctx, db.deleteAllDailyUsageStmt).Run()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyDeleteOldDailyUsage(ctx context.Context, p *int64Payload) (any, error) {
	err := db.conn.Query(ctx, db.deleteOldDailyUsageStmt, cutoffDaysArgs{CutoffDays: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateLease(ctx context.Context, lease *IPLease) (any, error) {
	err := db.conn.Query(ctx, db.createLeaseStmt, lease).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateLeaseSession(ctx context.Context, lease *IPLease) (any, error) {
	err := db.conn.Query(ctx, db.updateLeaseSessionStmt, lease).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteDynamicLease(ctx context.Context, p *intPayload) (any, error) {
	err := db.conn.Query(ctx, db.deleteLeaseStmt, IPLease{ID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAllDynamicLeases(ctx context.Context) error {
	err := db.conn.Query(ctx, db.deleteAllDynamicLeasesStmt).Run()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyDeleteDynamicLeasesByNode(ctx context.Context, p *intPayload) (any, error) {
	err := db.conn.Query(ctx, db.deleteDynLeasesByNodeStmt, IPLease{NodeID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateLeaseNode(ctx context.Context, lease *IPLease) (any, error) {
	err := db.conn.Query(ctx, db.updateLeaseNodeStmt, lease).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
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

	err := db.conn.Query(ctx, db.insertAuditLogStmt, log).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteOldAuditLogs(ctx context.Context, p *stringPayload) (any, error) {
	err := db.conn.Query(ctx, db.deleteOldAuditLogsStmt, cutoffArgs{Cutoff: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateUser(ctx context.Context, u *User) (any, error) {
	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.createUserStmt, u).Get(&outcome)
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

	err := db.conn.Query(ctx, db.editUserStmt, u).Get(&outcome)
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

	err := db.conn.Query(ctx, db.editUserPasswordStmt, u).Get(&outcome)
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

	err := db.conn.Query(ctx, db.deleteUserStmt, User{Email: p.Value}).Get(&outcome)
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
	err := db.conn.Query(ctx, db.createProfileStmt, p).Run()
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

	err := db.conn.Query(ctx, db.editProfileStmt, p).Get(&outcome)
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

	err := db.conn.Query(ctx, db.deleteProfileStmt, Profile{Name: p.Value}).Get(&outcome)
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
	err := db.conn.Query(ctx, db.createAPITokenStmt, t).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAPIToken(ctx context.Context, p *intPayload) (any, error) {
	err := db.conn.Query(ctx, db.deleteAPITokenStmt, APIToken{ID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateSession(ctx context.Context, s *Session) (any, error) {
	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.createSessionStmt, s).Get(&outcome)
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
	err := db.conn.Query(ctx, db.deleteSessionByTokenHashStmt, Session{TokenHash: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteExpiredSessions(ctx context.Context, p *int64Payload) (any, error) {
	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.deleteExpiredSessionsStmt, SessionCutoff{NowUnix: p.Value}).Get(&outcome)
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
	err := db.conn.Query(ctx, db.deleteOldestSessionsStmt, args).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAllSessionsForUser(ctx context.Context, p *int64Payload) (any, error) {
	err := db.conn.Query(ctx, db.deleteAllSessionsForUserStmt, UserIDArgs{UserID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAllSessions(ctx context.Context) error {
	err := db.conn.Query(ctx, db.deleteAllSessionsStmt).Run()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyCreateNetworkSlice(ctx context.Context, s *NetworkSlice) (any, error) {
	err := db.conn.Query(ctx, db.createNetworkSliceStmt, s).Run()
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

	err := db.conn.Query(ctx, db.editNetworkSliceStmt, s).Get(&outcome)
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

	err := db.conn.Query(ctx, db.deleteNetworkSliceStmt, NetworkSlice{Name: p.Value}).Get(&outcome)
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
	err := db.conn.Query(ctx, db.createDataNetworkStmt, dn).Run()
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

	err := db.conn.Query(ctx, db.editDataNetworkStmt, dn).Get(&outcome)
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

	err := db.conn.Query(ctx, db.deleteDataNetworkStmt, DataNetwork{Name: p.Value}).Get(&outcome)
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
	err := db.conn.Query(ctx, db.createPolicyStmt, p).Run()
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

	err := db.conn.Query(ctx, db.editPolicyStmt, p).Get(&outcome)
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

	err := db.conn.Query(ctx, db.deletePolicyStmt, Policy{Name: p.Value}).Get(&outcome)
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

	err := db.conn.Query(ctx, db.createNetworkRuleStmt, nr).Get(&outcome)
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

	err := db.conn.Query(ctx, db.updateNetworkRuleStmt, nr).Get(&outcome)
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

	err := db.conn.Query(ctx, db.deleteNetworkRuleStmt, NetworkRule{ID: p.Value}).Get(&outcome)
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
	err := db.conn.Query(ctx, db.deleteNetworkRulesByPolicyStmt, NetworkRule{PolicyID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateHomeNetworkKey(ctx context.Context, k *HomeNetworkKey) (any, error) {
	err := db.conn.Query(ctx, db.createHomeNetworkKeyStmt, k).Run()
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

	err := db.conn.Query(ctx, db.deleteHomeNetworkKeyStmt, HomeNetworkKey{ID: p.Value}).Get(&outcome)
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

	err := db.conn.Query(ctx, db.createBGPPeerStmt, p).Get(&outcome)
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

	err := db.conn.Query(ctx, db.updateBGPPeerStmt, p).Get(&outcome)
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

	err := db.conn.Query(ctx, db.deleteBGPPeerStmt, BGPPeer{ID: p.Value}).Get(&outcome)
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
	err := db.conn.Query(ctx, db.upsertBGPSettingsStmt, s).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applySetImportPrefixesForPeer(ctx context.Context, p *importPrefixesPayload) (any, error) {
	tx, err := db.conn.Begin(ctx, nil)
	if err != nil {
		if strings.Contains(err.Error(), "cannot start a transaction within a transaction") {
			if err := db.conn.Query(ctx, db.deleteImportPrefixesByPeerStmt, BGPImportPrefix{PeerID: p.PeerID}).Run(); err != nil {
				return nil, fmt.Errorf("delete existing prefixes: %w", err)
			}

			for _, prefix := range p.Prefixes {
				prefix.PeerID = p.PeerID

				if err := db.conn.Query(ctx, db.createImportPrefixStmt, prefix).Run(); err != nil {
					return nil, fmt.Errorf("insert prefix: %w", err)
				}
			}

			return nil, nil
		}

		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	err = tx.Query(ctx, db.deleteImportPrefixesByPeerStmt, BGPImportPrefix{PeerID: p.PeerID}).Run()
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("delete existing prefixes: %w", err)
	}

	for _, prefix := range p.Prefixes {
		prefix.PeerID = p.PeerID

		err = tx.Query(ctx, db.createImportPrefixStmt, prefix).Run()
		if err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("insert prefix: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateNATSettings(ctx context.Context, p *boolPayload) (any, error) {
	err := db.conn.Query(ctx, db.upsertNATSettingsStmt, NATSettings{Enabled: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateN3Settings(ctx context.Context, p *stringPayload) (any, error) {
	err := db.conn.Query(ctx, db.updateN3SettingsStmt, N3Settings{ExternalAddress: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateFlowAccountingSettings(ctx context.Context, p *boolPayload) (any, error) {
	err := db.conn.Query(ctx, db.upsertFlowAccountingSettingsStmt, FlowAccountingSettings{Enabled: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applySetRetentionPolicy(ctx context.Context, rp *RetentionPolicy) (any, error) {
	err := db.conn.Query(ctx, db.upsertRetentionPolicyStmt, rp).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyInitializeOperator(ctx context.Context, op *Operator) (any, error) {
	err := db.conn.Query(ctx, db.initializeOperatorStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorTracking(ctx context.Context, op *Operator) (any, error) {
	err := db.conn.Query(ctx, db.updateOperatorTrackingStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorID(ctx context.Context, op *Operator) (any, error) {
	err := db.conn.Query(ctx, db.updateOperatorIDStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorCode(ctx context.Context, op *Operator) (any, error) {
	err := db.conn.Query(ctx, db.updateOperatorCodeStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorSecurityAlgorithms(ctx context.Context, op *Operator) (any, error) {
	err := db.conn.Query(ctx, db.updateOperatorSecurityAlgorithmsStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorSPN(ctx context.Context, op *Operator) (any, error) {
	err := db.conn.Query(ctx, db.updateOperatorSPNStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorAMFIdentity(ctx context.Context, op *Operator) (any, error) {
	err := db.conn.Query(ctx, db.updateOperatorAMFIdentityStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorClusterID(ctx context.Context, op *Operator) (any, error) {
	err := db.conn.Query(ctx, db.updateOperatorClusterIDStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applySetJWTSecret(ctx context.Context, p *bytesPayload) (any, error) {
	err := db.conn.Query(ctx, db.upsertJWTSecretStmt, JWTSecret{Secret: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateRoute(ctx context.Context, r *Route) (any, error) {
	err := db.conn.Query(ctx, db.createRouteStmt, r).Run()
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

	err := db.conn.Query(ctx, db.deleteRouteStmt, Route{ID: p.Value}).Get(&outcome)
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
	err := db.conn.Query(ctx, db.upsertClusterMemberStmt, m).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteClusterMember(ctx context.Context, p *intPayload) (any, error) {
	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.deleteClusterMemberStmt, ClusterMember{NodeID: p.Value}).Get(&outcome)
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
		return nil, fmt.Errorf("unknown shared migration version %d", p.TargetVersion)
	}

	m := migrations[idx]
	if m.version != p.TargetVersion {
		return nil, fmt.Errorf("migration registry mismatch: expected version %d at index %d, got %d", p.TargetVersion, idx, m.version)
	}

	sqlConn := db.conn.PlainDB()

	if _, err := sqlConn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return nil, fmt.Errorf("disable foreign keys for migration %d: %w", p.TargetVersion, err)
	}

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

	if _, err := sqlConn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("re-enable foreign keys after migration %d: %w", p.TargetVersion, err)
	}

	return nil, nil
}
