// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

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
	// Subscribers
	case ellaraft.CmdCreateSubscriber:
		return applyJSON[Subscriber](ctx, cmd.Payload, db.applyCreateSubscriber)
	case ellaraft.CmdUpdateSubscriberProfile:
		return applyJSON[Subscriber](ctx, cmd.Payload, db.applyUpdateSubscriberProfile)
	case ellaraft.CmdEditSubscriberSeqNum:
		return applyJSON[Subscriber](ctx, cmd.Payload, db.applyEditSubscriberSeqNum)
	case ellaraft.CmdDeleteSubscriber:
		return applyJSON[stringPayload](ctx, cmd.Payload, db.applyDeleteSubscriber)

	// Daily Usage
	case ellaraft.CmdIncrementDailyUsage:
		return applyJSON[DailyUsage](ctx, cmd.Payload, db.applyIncrementDailyUsage)
	case ellaraft.CmdClearDailyUsage:
		return nil, db.applyClearDailyUsage(ctx)
	case ellaraft.CmdDeleteOldDailyUsage:
		return applyJSON[int64Payload](ctx, cmd.Payload, db.applyDeleteOldDailyUsage)

	// IP Leases
	case ellaraft.CmdCreateLease:
		return applyJSON[IPLease](ctx, cmd.Payload, db.applyCreateLease)
	case ellaraft.CmdUpdateLeaseSession:
		return applyJSON[IPLease](ctx, cmd.Payload, db.applyUpdateLeaseSession)
	case ellaraft.CmdDeleteDynamicLease:
		return applyJSON[intPayload](ctx, cmd.Payload, db.applyDeleteDynamicLease)
	case ellaraft.CmdDeleteAllDynamicLeases:
		return nil, db.applyDeleteAllDynamicLeases(ctx)
	case ellaraft.CmdDeleteDynamicLeasesByNode:
		return applyJSON[intPayload](ctx, cmd.Payload, db.applyDeleteDynamicLeasesByNode)
	case ellaraft.CmdUpdateLeaseNode:
		return applyJSON[IPLease](ctx, cmd.Payload, db.applyUpdateLeaseNode)

	// Audit Logs
	case ellaraft.CmdInsertAuditLog:
		return applyJSON[auditLogPayload](ctx, cmd.Payload, db.applyInsertAuditLog)
	case ellaraft.CmdDeleteOldAuditLogs:
		return applyJSON[stringPayload](ctx, cmd.Payload, db.applyDeleteOldAuditLogs)

	// Users
	case ellaraft.CmdCreateUser:
		return applyJSON[User](ctx, cmd.Payload, db.applyCreateUser)
	case ellaraft.CmdUpdateUser:
		return applyJSON[User](ctx, cmd.Payload, db.applyUpdateUser)
	case ellaraft.CmdUpdateUserPassword:
		return applyJSON[User](ctx, cmd.Payload, db.applyUpdateUserPassword)
	case ellaraft.CmdDeleteUser:
		return applyJSON[stringPayload](ctx, cmd.Payload, db.applyDeleteUser)

	// Profiles
	case ellaraft.CmdCreateProfile:
		return applyJSON[Profile](ctx, cmd.Payload, db.applyCreateProfile)
	case ellaraft.CmdUpdateProfile:
		return applyJSON[Profile](ctx, cmd.Payload, db.applyUpdateProfile)
	case ellaraft.CmdDeleteProfile:
		return applyJSON[stringPayload](ctx, cmd.Payload, db.applyDeleteProfile)

	// API Tokens
	case ellaraft.CmdCreateAPIToken:
		return applyJSON[APIToken](ctx, cmd.Payload, db.applyCreateAPIToken)
	case ellaraft.CmdDeleteAPIToken:
		return applyJSON[intPayload](ctx, cmd.Payload, db.applyDeleteAPIToken)

	// Sessions
	case ellaraft.CmdCreateSession:
		return applyJSON[Session](ctx, cmd.Payload, db.applyCreateSession)
	case ellaraft.CmdDeleteSessionByTokenHash:
		return applyJSON[bytesPayload](ctx, cmd.Payload, db.applyDeleteSessionByTokenHash)
	case ellaraft.CmdDeleteExpiredSessions:
		return applyJSON[int64Payload](ctx, cmd.Payload, db.applyDeleteExpiredSessions)
	case ellaraft.CmdDeleteOldestSessions:
		return applyJSON[DeleteOldestArgs](ctx, cmd.Payload, db.applyDeleteOldestSessions)
	case ellaraft.CmdDeleteAllSessionsForUser:
		return applyJSON[int64Payload](ctx, cmd.Payload, db.applyDeleteAllSessionsForUser)
	case ellaraft.CmdDeleteAllSessions:
		return nil, db.applyDeleteAllSessions(ctx)

	// Network Slices
	case ellaraft.CmdCreateNetworkSlice:
		return applyJSON[NetworkSlice](ctx, cmd.Payload, db.applyCreateNetworkSlice)
	case ellaraft.CmdUpdateNetworkSlice:
		return applyJSON[NetworkSlice](ctx, cmd.Payload, db.applyUpdateNetworkSlice)
	case ellaraft.CmdDeleteNetworkSlice:
		return applyJSON[stringPayload](ctx, cmd.Payload, db.applyDeleteNetworkSlice)

	// Data Networks
	case ellaraft.CmdCreateDataNetwork:
		return applyJSON[DataNetwork](ctx, cmd.Payload, db.applyCreateDataNetwork)
	case ellaraft.CmdUpdateDataNetwork:
		return applyJSON[DataNetwork](ctx, cmd.Payload, db.applyUpdateDataNetwork)
	case ellaraft.CmdDeleteDataNetwork:
		return applyJSON[stringPayload](ctx, cmd.Payload, db.applyDeleteDataNetwork)

	// Policies
	case ellaraft.CmdCreatePolicy:
		return applyJSON[Policy](ctx, cmd.Payload, db.applyCreatePolicy)
	case ellaraft.CmdUpdatePolicy:
		return applyJSON[Policy](ctx, cmd.Payload, db.applyUpdatePolicy)
	case ellaraft.CmdDeletePolicy:
		return applyJSON[stringPayload](ctx, cmd.Payload, db.applyDeletePolicy)

	// Network Rules
	case ellaraft.CmdCreateNetworkRule:
		return applyJSON[NetworkRule](ctx, cmd.Payload, db.applyCreateNetworkRule)
	case ellaraft.CmdUpdateNetworkRule:
		return applyJSON[NetworkRule](ctx, cmd.Payload, db.applyUpdateNetworkRule)
	case ellaraft.CmdDeleteNetworkRule:
		return applyJSON[int64Payload](ctx, cmd.Payload, db.applyDeleteNetworkRule)
	case ellaraft.CmdDeleteNetworkRulesByPolicy:
		return applyJSON[int64Payload](ctx, cmd.Payload, db.applyDeleteNetworkRulesByPolicy)

	// Home Network Keys
	case ellaraft.CmdCreateHomeNetworkKey:
		return applyJSON[HomeNetworkKey](ctx, cmd.Payload, db.applyCreateHomeNetworkKey)
	case ellaraft.CmdDeleteHomeNetworkKey:
		return applyJSON[intPayload](ctx, cmd.Payload, db.applyDeleteHomeNetworkKey)

	// BGP Peers
	case ellaraft.CmdCreateBGPPeer:
		return applyJSON[BGPPeer](ctx, cmd.Payload, db.applyCreateBGPPeer)
	case ellaraft.CmdUpdateBGPPeer:
		return applyJSON[BGPPeer](ctx, cmd.Payload, db.applyUpdateBGPPeer)
	case ellaraft.CmdDeleteBGPPeer:
		return applyJSON[intPayload](ctx, cmd.Payload, db.applyDeleteBGPPeer)

	// BGP Settings
	case ellaraft.CmdUpdateBGPSettings:
		return applyJSON[BGPSettings](ctx, cmd.Payload, db.applyUpdateBGPSettings)

	// BGP Import Prefixes
	case ellaraft.CmdSetImportPrefixesForPeer:
		return applyJSON[importPrefixesPayload](ctx, cmd.Payload, db.applySetImportPrefixesForPeer)

	// Settings (singletons)
	case ellaraft.CmdUpdateNATSettings:
		return applyJSON[boolPayload](ctx, cmd.Payload, db.applyUpdateNATSettings)
	case ellaraft.CmdUpdateN3Settings:
		return applyJSON[stringPayload](ctx, cmd.Payload, db.applyUpdateN3Settings)
	case ellaraft.CmdUpdateFlowAccountingSettings:
		return applyJSON[boolPayload](ctx, cmd.Payload, db.applyUpdateFlowAccountingSettings)
	case ellaraft.CmdSetRetentionPolicy:
		return applyJSON[RetentionPolicy](ctx, cmd.Payload, db.applySetRetentionPolicy)

	// Operator
	case ellaraft.CmdInitializeOperator:
		return applyJSON[Operator](ctx, cmd.Payload, db.applyInitializeOperator)
	case ellaraft.CmdUpdateOperatorTracking:
		return applyJSON[Operator](ctx, cmd.Payload, db.applyUpdateOperatorTracking)
	case ellaraft.CmdUpdateOperatorID:
		return applyJSON[Operator](ctx, cmd.Payload, db.applyUpdateOperatorID)
	case ellaraft.CmdUpdateOperatorCode:
		return applyJSON[Operator](ctx, cmd.Payload, db.applyUpdateOperatorCode)
	case ellaraft.CmdUpdateOperatorSecurityAlgorithms:
		return applyJSON[Operator](ctx, cmd.Payload, db.applyUpdateOperatorSecurityAlgorithms)
	case ellaraft.CmdUpdateOperatorSPN:
		return applyJSON[Operator](ctx, cmd.Payload, db.applyUpdateOperatorSPN)
	case ellaraft.CmdUpdateOperatorAMFIdentity:
		return applyJSON[Operator](ctx, cmd.Payload, db.applyUpdateOperatorAMFIdentity)
	case ellaraft.CmdUpdateOperatorClusterID:
		return applyJSON[Operator](ctx, cmd.Payload, db.applyUpdateOperatorClusterID)

	// JWT Secret
	case ellaraft.CmdSetJWTSecret:
		return applyJSON[bytesPayload](ctx, cmd.Payload, db.applySetJWTSecret)

	// Routes
	case ellaraft.CmdCreateRoute:
		return applyJSON[Route](ctx, cmd.Payload, db.applyCreateRoute)
	case ellaraft.CmdDeleteRoute:
		return applyJSON[int64Payload](ctx, cmd.Payload, db.applyDeleteRoute)

	// Cluster Members
	case ellaraft.CmdUpsertClusterMember:
		return applyJSON[ClusterMember](ctx, cmd.Payload, db.applyUpsertClusterMember)
	case ellaraft.CmdDeleteClusterMember:
		return applyJSON[intPayload](ctx, cmd.Payload, db.applyDeleteClusterMember)

	// Restore — replaces shared.db with the carried backup bytes.
	case ellaraft.CmdRestore:
		return applyJSON[bytesPayload](ctx, cmd.Payload, db.applyRestore)

	default:
		return nil, fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// SharedPlainDB returns the raw *sql.DB for the shared database.
func (db *Database) SharedPlainDB() *sql.DB {
	return db.shared.PlainDB()
}

// ReopenShared closes the current shared database connection, opens a fresh
// one, runs migrations, and re-prepares all sqlair statements.
func (db *Database) ReopenShared(ctx context.Context) error {
	if db.shared != nil {
		_ = db.shared.PlainDB().Close()
	}

	sharedConn, err := openSQLiteConnection(ctx, db.SharedPath(), SyncFull)
	if err != nil {
		return fmt.Errorf("reopen shared database: %w", err)
	}

	if err := runSharedMigrations(ctx, sharedConn); err != nil {
		_ = sharedConn.Close()
		return fmt.Errorf("shared migrations after reopen: %w", err)
	}

	db.shared = sqlair.NewDB(sharedConn)

	if err := db.PrepareStatements(); err != nil {
		return fmt.Errorf("re-prepare statements: %w", err)
	}

	return nil
}

// applyJSON is a generic helper that unmarshals a JSON payload and calls the
// apply function. It reduces boilerplate in the ApplyCommand dispatch table.
func applyJSON[T any](ctx context.Context, payload json.RawMessage, fn func(context.Context, *T) (any, error)) (any, error) {
	var v T
	if err := json.Unmarshal(payload, &v); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	return fn(ctx, &v)
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
)

// propose creates a Raft command and proposes it through the leader. Returns
// the result from FSM.Apply. Transient Raft errors (queue full, leadership
// lost, shutdown) are mapped to ErrProposeTimeout so the API layer can
// return 503.
func (db *Database) propose(cmdType ellaraft.CommandType, payload any) (any, error) {
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
// the caller should retry or surface a 503. ErrNotLeader is included for
// future HA mode; in standalone the local node is always the leader.
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
	err := db.shared.Query(ctx, db.createSubscriberStmt, s).Run()
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

	err := db.shared.Query(ctx, db.updateSubscriberProfileStmt, s).Get(&outcome)
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

	err := db.shared.Query(ctx, db.updateSubscriberSqnNumStmt, s).Get(&outcome)
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

	err := db.shared.Query(ctx, db.deleteSubscriberStmt, Subscriber{Imsi: p.Value}).Get(&outcome)
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
	err := db.shared.Query(ctx, db.incrementDailyUsageStmt, du).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyClearDailyUsage(ctx context.Context) error {
	err := db.shared.Query(ctx, db.deleteAllDailyUsageStmt).Run()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyDeleteOldDailyUsage(ctx context.Context, p *int64Payload) (any, error) {
	err := db.shared.Query(ctx, db.deleteOldDailyUsageStmt, cutoffDaysArgs{CutoffDays: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateLease(ctx context.Context, lease *IPLease) (any, error) {
	err := db.shared.Query(ctx, db.createLeaseStmt, lease).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateLeaseSession(ctx context.Context, lease *IPLease) (any, error) {
	err := db.shared.Query(ctx, db.updateLeaseSessionStmt, lease).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteDynamicLease(ctx context.Context, p *intPayload) (any, error) {
	err := db.shared.Query(ctx, db.deleteLeaseStmt, IPLease{ID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAllDynamicLeases(ctx context.Context) error {
	err := db.shared.Query(ctx, db.deleteAllDynamicLeasesStmt).Run()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyDeleteDynamicLeasesByNode(ctx context.Context, p *intPayload) (any, error) {
	err := db.shared.Query(ctx, db.deleteDynLeasesByNodeStmt, IPLease{NodeID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateLeaseNode(ctx context.Context, lease *IPLease) (any, error) {
	err := db.shared.Query(ctx, db.updateLeaseNodeStmt, lease).Run()
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

	err := db.shared.Query(ctx, db.insertAuditLogStmt, log).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteOldAuditLogs(ctx context.Context, p *stringPayload) (any, error) {
	err := db.shared.Query(ctx, db.deleteOldAuditLogsStmt, cutoffArgs{Cutoff: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateUser(ctx context.Context, u *User) (any, error) {
	var outcome sqlair.Outcome

	err := db.shared.Query(ctx, db.createUserStmt, u).Get(&outcome)
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

	err := db.shared.Query(ctx, db.editUserStmt, u).Get(&outcome)
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

	err := db.shared.Query(ctx, db.editUserPasswordStmt, u).Get(&outcome)
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

	err := db.shared.Query(ctx, db.deleteUserStmt, User{Email: p.Value}).Get(&outcome)
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
	err := db.shared.Query(ctx, db.createProfileStmt, p).Run()
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

	err := db.shared.Query(ctx, db.editProfileStmt, p).Get(&outcome)
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

	err := db.shared.Query(ctx, db.deleteProfileStmt, Profile{Name: p.Value}).Get(&outcome)
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
	err := db.shared.Query(ctx, db.createAPITokenStmt, t).Run()
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAPIToken(ctx context.Context, p *intPayload) (any, error) {
	err := db.shared.Query(ctx, db.deleteAPITokenStmt, APIToken{ID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateSession(ctx context.Context, s *Session) (any, error) {
	var outcome sqlair.Outcome

	err := db.shared.Query(ctx, db.createSessionStmt, s).Get(&outcome)
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
	err := db.shared.Query(ctx, db.deleteSessionByTokenHashStmt, Session{TokenHash: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteExpiredSessions(ctx context.Context, p *int64Payload) (any, error) {
	var outcome sqlair.Outcome

	err := db.shared.Query(ctx, db.deleteExpiredSessionsStmt, SessionCutoff{NowUnix: p.Value}).Get(&outcome)
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
	err := db.shared.Query(ctx, db.deleteOldestSessionsStmt, args).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAllSessionsForUser(ctx context.Context, p *int64Payload) (any, error) {
	err := db.shared.Query(ctx, db.deleteAllSessionsForUserStmt, UserIDArgs{UserID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteAllSessions(ctx context.Context) error {
	err := db.shared.Query(ctx, db.deleteAllSessionsStmt).Run()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) applyCreateNetworkSlice(ctx context.Context, s *NetworkSlice) (any, error) {
	err := db.shared.Query(ctx, db.createNetworkSliceStmt, s).Run()
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

	err := db.shared.Query(ctx, db.editNetworkSliceStmt, s).Get(&outcome)
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

	err := db.shared.Query(ctx, db.deleteNetworkSliceStmt, NetworkSlice{Name: p.Value}).Get(&outcome)
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
	err := db.shared.Query(ctx, db.createDataNetworkStmt, dn).Run()
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

	err := db.shared.Query(ctx, db.editDataNetworkStmt, dn).Get(&outcome)
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

	err := db.shared.Query(ctx, db.deleteDataNetworkStmt, DataNetwork{Name: p.Value}).Get(&outcome)
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
	err := db.shared.Query(ctx, db.createPolicyStmt, p).Run()
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

	err := db.shared.Query(ctx, db.editPolicyStmt, p).Get(&outcome)
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

	err := db.shared.Query(ctx, db.deletePolicyStmt, Policy{Name: p.Value}).Get(&outcome)
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

	err := db.shared.Query(ctx, db.createNetworkRuleStmt, nr).Get(&outcome)
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

	err := db.shared.Query(ctx, db.updateNetworkRuleStmt, nr).Get(&outcome)
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

	err := db.shared.Query(ctx, db.deleteNetworkRuleStmt, NetworkRule{ID: p.Value}).Get(&outcome)
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
	err := db.shared.Query(ctx, db.deleteNetworkRulesByPolicyStmt, NetworkRule{PolicyID: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateHomeNetworkKey(ctx context.Context, k *HomeNetworkKey) (any, error) {
	err := db.shared.Query(ctx, db.createHomeNetworkKeyStmt, k).Run()
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

	err := db.shared.Query(ctx, db.deleteHomeNetworkKeyStmt, HomeNetworkKey{ID: p.Value}).Get(&outcome)
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

	err := db.shared.Query(ctx, db.createBGPPeerStmt, p).Get(&outcome)
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

	err := db.shared.Query(ctx, db.updateBGPPeerStmt, p).Get(&outcome)
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

	err := db.shared.Query(ctx, db.deleteBGPPeerStmt, BGPPeer{ID: p.Value}).Get(&outcome)
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
	err := db.shared.Query(ctx, db.upsertBGPSettingsStmt, s).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applySetImportPrefixesForPeer(ctx context.Context, p *importPrefixesPayload) (any, error) {
	tx, err := db.shared.Begin(ctx, nil)
	if err != nil {
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
	err := db.shared.Query(ctx, db.upsertNATSettingsStmt, NATSettings{Enabled: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateN3Settings(ctx context.Context, p *stringPayload) (any, error) {
	err := db.shared.Query(ctx, db.updateN3SettingsStmt, N3Settings{ExternalAddress: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateFlowAccountingSettings(ctx context.Context, p *boolPayload) (any, error) {
	err := db.shared.Query(ctx, db.upsertFlowAccountingSettingsStmt, FlowAccountingSettings{Enabled: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applySetRetentionPolicy(ctx context.Context, rp *RetentionPolicy) (any, error) {
	err := db.shared.Query(ctx, db.upsertRetentionPolicyStmt, rp).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyInitializeOperator(ctx context.Context, op *Operator) (any, error) {
	err := db.shared.Query(ctx, db.initializeOperatorStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorTracking(ctx context.Context, op *Operator) (any, error) {
	err := db.shared.Query(ctx, db.updateOperatorTrackingStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorID(ctx context.Context, op *Operator) (any, error) {
	err := db.shared.Query(ctx, db.updateOperatorIDStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorCode(ctx context.Context, op *Operator) (any, error) {
	err := db.shared.Query(ctx, db.updateOperatorCodeStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorSecurityAlgorithms(ctx context.Context, op *Operator) (any, error) {
	err := db.shared.Query(ctx, db.updateOperatorSecurityAlgorithmsStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorSPN(ctx context.Context, op *Operator) (any, error) {
	err := db.shared.Query(ctx, db.updateOperatorSPNStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorAMFIdentity(ctx context.Context, op *Operator) (any, error) {
	err := db.shared.Query(ctx, db.updateOperatorAMFIdentityStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateOperatorClusterID(ctx context.Context, op *Operator) (any, error) {
	err := db.shared.Query(ctx, db.updateOperatorClusterIDStmt, op).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applySetJWTSecret(ctx context.Context, p *bytesPayload) (any, error) {
	err := db.shared.Query(ctx, db.upsertJWTSecretStmt, JWTSecret{Secret: p.Value}).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyCreateRoute(ctx context.Context, r *Route) (any, error) {
	err := db.shared.Query(ctx, db.createRouteStmt, r).Run()
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

	err := db.shared.Query(ctx, db.deleteRouteStmt, Route{ID: p.Value}).Get(&outcome)
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
	err := db.shared.Query(ctx, db.upsertClusterMemberStmt, m).Run()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteClusterMember(ctx context.Context, p *intPayload) (any, error) {
	var outcome sqlair.Outcome

	err := db.shared.Query(ctx, db.deleteClusterMemberStmt, ClusterMember{NodeID: p.Value}).Get(&outcome)
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
