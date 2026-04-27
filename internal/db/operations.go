// Copyright 2026 Ella Networks

// Typed-operation dispatch for replicated writes.
//
// Every replicated SQL write is an explicit typed operation: a unique name
// plus a JSON-serialisable payload. On the leader, an operation dispatches
// to its apply function against the leader's own state, then captures the
// resulting changeset and proposes it through Raft. On a follower, the
// operation (operation name + payload JSON) is forwarded to the leader's
// /cluster/internal/propose endpoint; the follower never captures.
//
// This preserves the two invariants that make replication correct without
// "usually works" caveats:
//
//  1. Only the leader captures changesets, and it captures against state
//     that produced the captured values (auto-increment IDs, UPDATE
//     before-images, UPSERT-resolved values, default-expression results).
//  2. The forwarded wire (operation name + typed payload) is schema- and
//     version-stable, not an opaque byte blob with an implicit schema
//     contract.
//
// A registry maps operation name to (payload type, apply function) so the
// leader's HTTP handler can re-hydrate a payload arriving from a follower
// and invoke the same apply path a local caller would take.
//
// Each registration declares the minimum applied schema version it
// requires (default 1 — the baseline). Three enforcement points:
//
//   - call-time: ChangesetOp.Invoke / intentOp.Invoke return
//     ErrMigrationPending if applied < minSchema, surfaced as a
//     retryable 503 by the API layer.
//   - capture-time: leaderCaptureAndPropose stamps RequiredSchema on
//     the captured bytesPayload so apply-time can verify on every node.
//   - apply-time: ApplyCommand refuses to apply a changeset / intent
//     command whose minSchema exceeds local applied schema; the existing
//     FSM panic handler halts the node, matching the contract for any
//     other apply failure.

package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	hraft "github.com/hashicorp/raft"
	"go.uber.org/zap"
)

// OpOption configures an operation registration. RequireSchema is the
// only option today; the type stays open for future additions
// (ignorable-on-old-version, deprecation markers, etc.).
type OpOption func(*opMeta)

type opMeta struct {
	minSchema int
}

// RequireSchema declares the minimum applied schema version required
// for this operation to run. Operations registered without this option
// default to 1 (the baseline — works on every supported deployment).
// Operations whose apply function or any prepared statement it uses
// depends on a column or table introduced in migration N must declare
// RequireSchema(N).
//
// The value is enforced at three points: call-time (Invoke returns
// ErrMigrationPending), capture-time (stamped on the changeset
// envelope as RequiredSchema), and apply-time (every node verifies
// before running the FSM apply path).
func RequireSchema(n int) OpOption {
	return func(m *opMeta) {
		if n < 1 {
			n = 1
		}

		m.minSchema = n
	}
}

// changesetOpHandler erases the payload type P so all changeset ops can
// live in a single map keyed by operation name. unmarshal returns a typed
// payload (as any) so apply can then type-assert it back.
type changesetOpHandler struct {
	// minSchema is the value declared by RequireSchema at registration.
	// Defaults to 1.
	minSchema int

	// applyJSON deserialises raw payload bytes and runs the apply function
	// against db. Intended for the leader-side forwarded-op dispatch path.
	applyJSON func(db *Database, ctx context.Context, raw json.RawMessage) (any, error)
}

// intentOpHandler is the CmdXxx-typed counterpart for intent ops that the
// FSM dispatches directly (bulk retention deletes, migrations).
type intentOpHandler struct {
	minSchema int
	cmdType   ellaraft.CommandType
}

var (
	changesetOps = map[string]changesetOpHandler{}
	intentOps    = map[string]intentOpHandler{}
)

// ChangesetOp binds an operation name to a typed apply function. Registered
// once at package init via registerChangesetOp and referenced by call sites
// through Invoke, which hides the leader/follower branching.
type ChangesetOp[P any] struct {
	name      string
	minSchema int
	apply     func(db *Database, ctx context.Context, p *P) (any, error)
}

// Name returns the registered operation name. Used by the lock-file
// generator to enumerate the registry.
func (op *ChangesetOp[P]) Name() string { return op.name }

// MinSchema returns the minimum applied schema required for this op.
func (op *ChangesetOp[P]) MinSchema() int { return op.minSchema }

// registerChangesetOp creates a ChangesetOp, registers it in the global
// dispatch table, and returns a handle for call sites. The registry entry
// is needed so the leader's /cluster/internal/propose handler can invoke
// the op from (name, payload JSON) arriving on the wire.
//
// The registry is append-only: renaming a registered op, removing one,
// or reducing its minSchema breaks the wire contract for in-flight
// rolling upgrades. A unit test in operations_lock_test.go enforces
// this against a checked-in operations.lock.json.
func registerChangesetOp[P any](
	name string,
	apply func(db *Database, ctx context.Context, p *P) (any, error),
	opts ...OpOption,
) *ChangesetOp[P] {
	if _, exists := changesetOps[name]; exists {
		panic(fmt.Sprintf("duplicate changeset op registration: %s", name))
	}

	if _, exists := intentOps[name]; exists {
		panic(fmt.Sprintf("changeset op %s collides with intent op", name))
	}

	meta := opMeta{minSchema: 1}
	for _, opt := range opts {
		opt(&meta)
	}

	op := &ChangesetOp[P]{name: name, minSchema: meta.minSchema, apply: apply}

	changesetOps[name] = changesetOpHandler{
		minSchema: meta.minSchema,
		applyJSON: func(db *Database, ctx context.Context, raw json.RawMessage) (any, error) {
			var p P
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, fmt.Errorf("unmarshal %s payload: %w", name, err)
			}

			return apply(db, ctx, &p)
		},
	}

	return op
}

// registerIntentOp registers an intent command with an operation name used
// on the forwarded-op wire. CmdXxx-typed payload delivery across nodes stays
// opaque-json (the FSM decodes by command type), but the leader-receiver
// side reuses the same dispatch envelope.
//
// opts is variadic for symmetry with registerChangesetOp; today no
// shipped intent op needs a non-default RequireSchema (the bulk-delete
// and migration commands all operate on baseline tables). Kept in the
// signature so future intent ops can opt in without a backward-
// incompatible API change.
//
//nolint:unparam // opts intentionally retained for forward-compat; see comment above.
func registerIntentOp(name string, cmdType ellaraft.CommandType, opts ...OpOption) intentOp {
	if _, exists := intentOps[name]; exists {
		panic(fmt.Sprintf("duplicate intent op registration: %s", name))
	}

	if _, exists := changesetOps[name]; exists {
		panic(fmt.Sprintf("intent op %s collides with changeset op", name))
	}

	meta := opMeta{minSchema: 1}
	for _, opt := range opts {
		opt(&meta)
	}

	intentOps[name] = intentOpHandler{minSchema: meta.minSchema, cmdType: cmdType}

	return intentOp{name: name, minSchema: meta.minSchema, cmdType: cmdType}
}

// intentOp is the call-site handle for an intent command.
type intentOp struct {
	name      string
	minSchema int
	cmdType   ellaraft.CommandType
}

// Name returns the registered operation name.
func (op intentOp) Name() string { return op.name }

// MinSchema returns the minimum applied schema required for this op.
func (op intentOp) MinSchema() int { return op.minSchema }

// intentMinSchemaForCmd returns the minimum applied schema required for
// the given CommandType, or 1 if not a registered intent op. Used by
// the apply-time gate in ApplyCommand. CmdChangeset is gated by the
// per-changeset RequiredSchema field, not by command type — it always
// returns 1 here.
func intentMinSchemaForCmd(t ellaraft.CommandType) int {
	for _, h := range intentOps {
		if h.cmdType == t {
			return h.minSchema
		}
	}

	return 1
}

// allRegisteredOps returns every changeset and intent op registration in
// a stable shape for the lock-file generator and append-only test.
type registeredOp struct {
	Name      string
	Kind      string // "changeset" or "intent"
	MinSchema int
	CmdType   string // empty for changeset; CommandType.String() for intent
}

func allRegisteredOps() []registeredOp {
	out := make([]registeredOp, 0, len(changesetOps)+len(intentOps))

	for name, h := range changesetOps {
		out = append(out, registeredOp{
			Name:      name,
			Kind:      "changeset",
			MinSchema: h.minSchema,
		})
	}

	for name, h := range intentOps {
		out = append(out, registeredOp{
			Name:      name,
			Kind:      "intent",
			MinSchema: h.minSchema,
			CmdType:   h.cmdType.String(),
		})
	}

	return out
}

// Invoke runs the op: apply-locally on leader (or standalone), forward to
// the leader on a follower. The payload is marshalled once here and, on the
// leader, passed to the apply closure by value; on a follower, the marshalled
// bytes are what ship over the wire.
//
// Gates the call on `applied schema >= op.minSchema` before any work,
// returning ErrMigrationPending (mapped to 503 by the API layer) when
// the cluster is mid-rolling-upgrade and the migration this op depends
// on hasn't replicated yet.
func (op *ChangesetOp[P]) Invoke(db *Database, payload *P) (any, error) {
	if err := db.checkOpSchema(op.minSchema); err != nil {
		return nil, err
	}

	if db.raftManager == nil {
		return op.apply(db, context.Background(), payload)
	}

	if db.IsLeader() {
		result, err := db.leaderCaptureAndPropose(op.name, op.minSchema, func(ctx context.Context) (any, error) {
			return op.apply(db, ctx, payload)
		})
		if err == nil {
			return result, nil
		}

		if !errors.Is(err, hraft.ErrNotLeader) && !errors.Is(err, hraft.ErrLeadershipLost) {
			return nil, err
		}

		// Leadership lost between IsLeader() and Propose(); fall through
		// to the forward path. The payload is still valid; the leader
		// we forward to will capture against its own state.
	}

	return op.invokeFollower(db, payload)
}

func (op *ChangesetOp[P]) invokeFollower(db *Database, payload *P) (any, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s payload: %w", op.name, err)
	}

	result, err := db.forwardOperation(op.name, payloadJSON)
	if err != nil {
		return nil, err
	}

	return result.Value, nil
}

// Invoke runs an intent op. On the leader it goes straight to raft.Apply
// (intent commands are dispatched by the FSM via CommandType). On a
// follower it forwards (name, payload JSON) — the leader's handler wraps
// the payload into a Command envelope and applies.
func (op intentOp) Invoke(db *Database, payload any) (any, error) {
	if err := db.checkOpSchema(op.minSchema); err != nil {
		return nil, err
	}

	cmd, err := ellaraft.NewCommand(op.cmdType, payload)
	if err != nil {
		return nil, err
	}

	if db.raftManager == nil {
		return db.ApplyCommand(context.Background(), cmd)
	}

	if db.IsLeader() {
		data, err := cmd.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("marshal intent command: %w", err)
		}

		result, applyErr := db.raftManager.ApplyBytes(data, db.proposeTimeout)
		if applyErr == nil {
			return result.Value, nil
		}

		if !errors.Is(applyErr, hraft.ErrNotLeader) && !errors.Is(applyErr, hraft.ErrLeadershipLost) {
			if isTransientRaftErr(applyErr) {
				return nil, fmt.Errorf("%w: %v", ErrProposeTimeout, applyErr)
			}

			return nil, applyErr
		}
		// Lost leadership mid-apply — fall through to forward path.
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s payload: %w", op.name, err)
	}

	result, err := db.forwardOperation(op.name, payloadJSON)
	if err != nil {
		return nil, err
	}

	return result.Value, nil
}

// leaderCaptureAndPropose runs the capture→propose cycle on the leader.
// Serialised by proposeMu so concurrent writers never capture against the
// same pre-mutation state (which would produce conflicting changesets).
//
// minSchema is stamped on the captured bytesPayload as RequiredSchema so
// the FSM apply path on every node can verify before running.
func (db *Database) leaderCaptureAndPropose(operation string, minSchema int, applyFn func(context.Context) (any, error)) (any, error) {
	db.proposeMu.Lock()
	defer db.proposeMu.Unlock()

	changeset, applyResult, err := db.captureChangeset(context.Background(), applyFn, operation)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) ||
			errors.Is(err, ErrNotFound) ||
			errors.Is(err, ErrJoinTokenAlreadyConsumed) {
			return nil, err
		}

		return nil, fmt.Errorf("capture changeset for %s: %w", operation, err)
	}

	if len(changeset) == 0 {
		return applyResult, nil
	}

	changesetCmd, err := ellaraft.NewCommand(ellaraft.CmdChangeset, &bytesPayload{
		Value:          changeset,
		Operation:      operation,
		RequiredSchema: minSchema,
	})
	if err != nil {
		return nil, err
	}

	data, err := changesetCmd.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal changeset command: %w", err)
	}

	index, err := db.raftManager.ApplyBytes(data, db.proposeTimeout)
	if err != nil {
		if isTransientRaftErr(err) {
			return nil, fmt.Errorf("%w: %v", ErrProposeTimeout, err)
		}

		return nil, err
	}

	logger.DBLog.Debug("proposed changeset",
		zap.String("operation", operation),
		zap.Int("requiredSchema", minSchema),
		zap.Uint64("index", index.Index),
		zap.Int("bytes", len(changeset)))

	return applyResult, nil
}

// forwardOperation POSTs (operation, payload JSON) to the current leader's
// /cluster/internal/propose endpoint and returns the ProposeResult the
// leader produced. Classifies transient errors (no leader / leadership
// changed mid-forward) as ErrProposeTimeout so the API layer maps them to
// 503.
func (db *Database) forwardOperation(opName string, payload json.RawMessage) (*ellaraft.ProposeResult, error) {
	if db.raftManager == nil {
		return nil, hraft.ErrNotLeader
	}

	ctx, cancel := context.WithTimeout(context.Background(), db.proposeTimeout)
	defer cancel()

	result, err := db.raftManager.ForwardOperation(ctx, opName, payload, db.proposeTimeout)
	if err != nil {
		if isTransientRaftErr(err) {
			return nil, fmt.Errorf("%w: %v", ErrProposeTimeout, err)
		}

		return nil, err
	}

	return result, nil
}

// ApplyForwardedOperation is the leader-side entry point for a forwarded
// op. It dispatches (opName, payloadJSON) to the registered apply function,
// captures the resulting changeset, and proposes it through Raft.
// Intent ops skip capture and go straight to raft.Apply — the FSM itself
// dispatches them by CommandType.
func (db *Database) ApplyForwardedOperation(opName string, payload json.RawMessage) (*ellaraft.ProposeResult, error) {
	if db.raftManager == nil {
		return nil, fmt.Errorf("cluster not enabled")
	}

	if h, ok := changesetOps[opName]; ok {
		return db.applyForwardedChangesetOp(opName, h, payload)
	}

	if h, ok := intentOps[opName]; ok {
		return db.applyForwardedIntentOp(h, payload)
	}

	return nil, fmt.Errorf("%w %q", ErrUnknownOperation, opName)
}

func (db *Database) applyForwardedChangesetOp(opName string, h changesetOpHandler, payload json.RawMessage) (*ellaraft.ProposeResult, error) {
	if err := db.checkOpSchema(h.minSchema); err != nil {
		return nil, err
	}

	db.proposeMu.Lock()
	defer db.proposeMu.Unlock()

	changeset, applyResult, err := db.captureChangeset(context.Background(), func(ctx context.Context) (any, error) {
		return h.applyJSON(db, ctx, payload)
	}, opName)
	if err != nil {
		return nil, err
	}

	if len(changeset) == 0 {
		return &ellaraft.ProposeResult{Value: applyResult}, nil
	}

	changesetCmd, err := ellaraft.NewCommand(ellaraft.CmdChangeset, &bytesPayload{
		Value:          changeset,
		Operation:      opName,
		RequiredSchema: h.minSchema,
	})
	if err != nil {
		return nil, err
	}

	data, err := changesetCmd.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal changeset command: %w", err)
	}

	res, err := db.raftManager.ApplyBytes(data, db.proposeTimeout)
	if err != nil {
		return nil, err
	}

	return &ellaraft.ProposeResult{Index: res.Index, Value: applyResult}, nil
}

func (db *Database) applyForwardedIntentOp(h intentOpHandler, payload json.RawMessage) (*ellaraft.ProposeResult, error) {
	if err := db.checkOpSchema(h.minSchema); err != nil {
		return nil, err
	}

	cmd := &ellaraft.Command{Type: h.cmdType, Payload: payload}

	data, err := cmd.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal intent command: %w", err)
	}

	return db.raftManager.ApplyBytes(data, db.proposeTimeout)
}
