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

// OpOption configures an operation registration.
type OpOption func(*opMeta)

type opMeta struct {
	minSchema int
	topics    []Topic
}

// RequireSchema declares the minimum applied schema version required
// for this operation. Default 1 (works on the baseline). Set to N when
// the apply function or any statement it uses depends on a column or
// table introduced in migration N.
func RequireSchema(n int) OpOption {
	return func(m *opMeta) {
		if n < 1 {
			n = 1
		}

		m.minSchema = n
	}
}

// AffectsTopic declares the changefeed topics this op writes to.
// Subscribers to these topics are signalled after the op applies on
// every node. Ops that don't affect any reconciler-watched table omit
// this option.
func AffectsTopic(topics ...Topic) OpOption {
	return func(m *opMeta) {
		m.topics = append(m.topics, topics...)
	}
}

// changesetOpHandler erases the payload type P so changeset ops live
// in a single map keyed by operation name.
type changesetOpHandler struct {
	minSchema int
	topics    []Topic
	applyJSON func(db *Database, ctx context.Context, raw json.RawMessage) (any, error)
}

type intentOpHandler struct {
	minSchema int
	topics    []Topic
	cmdType   ellaraft.CommandType
}

var (
	changesetOps = map[string]changesetOpHandler{}
	intentOps    = map[string]intentOpHandler{}
)

// ChangesetOp binds an operation name to a typed apply function.
// Call-site handle returned by registerChangesetOp.
type ChangesetOp[P any] struct {
	name      string
	minSchema int
	apply     func(db *Database, ctx context.Context, p *P) (any, error)
}

func (op *ChangesetOp[P]) Name() string   { return op.name }
func (op *ChangesetOp[P]) MinSchema() int { return op.minSchema }

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
		topics:    meta.topics,
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

// opts retained for symmetry with registerChangesetOp; no shipped
// intent op currently needs a non-default RequireSchema.
//
//nolint:unparam
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

	intentOps[name] = intentOpHandler{minSchema: meta.minSchema, topics: meta.topics, cmdType: cmdType}

	return intentOp{name: name, minSchema: meta.minSchema, cmdType: cmdType}
}

// topicsForChangesetOp returns the topics declared by a registered
// changeset op. Empty when the op was registered without
// AffectsTopic — used by ApplyCommand to publish wakeups.
func topicsForChangesetOp(name string) []Topic {
	if h, ok := changesetOps[name]; ok {
		return h.topics
	}

	return nil
}

// topicsForIntentCmd returns the topics declared by the intent op
// matching the given CommandType.
func topicsForIntentCmd(t ellaraft.CommandType) []Topic {
	for _, h := range intentOps {
		if h.cmdType == t {
			return h.topics
		}
	}

	return nil
}

type intentOp struct {
	name      string
	minSchema int
	cmdType   ellaraft.CommandType
}

func (op intentOp) Name() string   { return op.name }
func (op intentOp) MinSchema() int { return op.minSchema }

// intentMinSchemaForCmd returns the minSchema for an intent CommandType,
// or 1 if not registered. CmdChangeset is gated by bytesPayload.RequiredSchema
// instead and always returns 1 here.
func intentMinSchemaForCmd(t ellaraft.CommandType) int {
	for _, h := range intentOps {
		if h.cmdType == t {
			return h.minSchema
		}
	}

	return 1
}

// registeredOp is the shape consumed by the lock-file test.
type registeredOp struct {
	Name      string
	Kind      string // "changeset" or "intent"
	MinSchema int
	CmdType   string // empty for changeset
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

// Invoke runs the op locally on leader / standalone, or forwards
// (operation, payload JSON) to the leader on a follower.
func (op *ChangesetOp[P]) Invoke(db *Database, payload *P) (any, error) {
	if err := db.checkOpSchema(op.minSchema); err != nil {
		return nil, err
	}

	if db.raftManager == nil {
		result, err := op.apply(db, context.Background(), payload)
		if err == nil {
			db.publishOpTopics(topicsForChangesetOp(op.name), 0)
		}

		return result, err
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

// Invoke runs an intent op via raft.Apply on the leader, or forwards
// to the leader on a follower.
func (op intentOp) Invoke(db *Database, payload any) (any, error) {
	if err := db.checkOpSchema(op.minSchema); err != nil {
		return nil, err
	}

	cmd, err := ellaraft.NewCommand(op.cmdType, payload)
	if err != nil {
		return nil, err
	}

	if db.raftManager == nil {
		return db.ApplyCommand(context.Background(), cmd, 0)
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
// proposeMu serialises captures so concurrent writers don't observe
// the same pre-mutation state. minSchema is stamped on bytesPayload as
// RequiredSchema for the apply-time gate on every node.
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

// forwardOperation POSTs to the leader's /cluster/internal/propose
// endpoint. Transient errors (no leader, leadership changed) become
// ErrProposeTimeout so the API maps them to 503.
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

// ApplyForwardedOperation is the leader-side handler for the
// /cluster/internal/propose endpoint. Changeset ops capture+propose;
// intent ops go straight to raft.Apply.
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
