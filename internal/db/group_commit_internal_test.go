// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func pendingFor[P any, R any](t *testing.T, database *Database, op *ChangesetOp[P, R], payload *P) *pendingWrite {
	t.Helper()

	ctx := t.Context()
	_, span := tracer.Start(ctx, "test/wait")

	return &pendingWrite{
		ctx:       ctx,
		operation: op.name,
		minSchema: op.minSchema,
		applyFn: func(ctx context.Context) (any, error) {
			return op.apply(database, ctx, payload)
		},
		waitSpan: span,
		done:     make(chan writeOutcome, 1),
	}
}

func outcomeOf(t *testing.T, w *pendingWrite) writeOutcome {
	t.Helper()

	select {
	case out := <-w.done:
		return out
	default:
		t.Fatalf("write %s has no outcome", w.operation)

		return writeOutcome{}
	}
}

func newTestProfile(t *testing.T, name string) *Profile {
	t.Helper()

	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid: %v", err)
	}

	return &Profile{ID: id.String(), Name: name, UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps", Allow4G: true, Allow5G: true}
}

func createTestSubscriber(t *testing.T, database *Database, imsi string) {
	t.Helper()

	profile := newTestProfile(t, "profile-"+imsi)
	if err := database.CreateProfile(t.Context(), profile); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}

	if err := database.CreateSubscriber(t.Context(), &Subscriber{
		Imsi:           imsi,
		PermanentKey:   "465b5ce8b199b49faa5f0a2ee238a6bc",
		Opc:            "cd63cb71954a9f4e48a5994e37a02baf",
		SequenceNumber: "000000000000",
		ProfileID:      profile.ID,
	}); err != nil {
		t.Fatalf("CreateSubscriber: %v", err)
	}
}

func TestGroupCommitBatchAppliesWritesInOrderAndIsolatesFailures(t *testing.T) {
	database := newStandaloneDB(t)

	const imsi = "001010000000101"

	createTestSubscriber(t, database, imsi)

	before := database.raftManager.AppliedIndex()

	batch := []*pendingWrite{
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "gc-a")),
		pendingFor(t, database, opAdvanceSubscriberSQN, &AdvanceSQNPayload{IMSI: imsi}),
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "gc-a")),
		pendingFor(t, database, opAdvanceSubscriberSQN, &AdvanceSQNPayload{IMSI: imsi}),
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "gc-b")),
		pendingFor(t, database, opAdvanceSubscriberSQN, &AdvanceSQNPayload{IMSI: imsi}),
	}

	if carry := database.commitWrites(batch); len(carry) != 0 {
		t.Fatalf("carry = %d writes, want 0", len(carry))
	}

	outcomes := make([]writeOutcome, len(batch))
	for i, w := range batch {
		outcomes[i] = outcomeOf(t, w)
	}

	if !errors.Is(outcomes[2].err, ErrAlreadyExists) {
		t.Fatalf("duplicate profile: err = %v, want ErrAlreadyExists", outcomes[2].err)
	}

	var index uint64

	for i, out := range outcomes {
		if i == 2 {
			continue
		}

		if out.err != nil {
			t.Fatalf("write %d (%s): %v", i, batch[i].operation, out.err)
		}

		if index == 0 {
			index = out.result.Index
		}

		if out.result.Index != index {
			t.Fatalf("write %d committed at index %d, want the batch index %d", i, out.result.Index, index)
		}
	}

	if got := database.raftManager.AppliedIndex() - before; got != 1 {
		t.Fatalf("batch used %d raft entries, want 1", got)
	}

	for i, want := range map[int]string{1: "000000000020", 3: "000000000040", 5: "000000000060"} {
		creds, ok := outcomes[i].result.Value.(*AdvancedCredentials)
		if !ok {
			t.Fatalf("write %d result = %T, want *AdvancedCredentials", i, outcomes[i].result.Value)
		}

		if creds.SequenceNumber != want {
			t.Fatalf("write %d sequence number = %s, want %s", i, creds.SequenceNumber, want)
		}
	}

	sub, err := database.GetSubscriber(t.Context(), imsi)
	if err != nil {
		t.Fatalf("GetSubscriber: %v", err)
	}

	if sub.SequenceNumber != "000000000060" {
		t.Fatalf("stored sequence number = %s, want 000000000060", sub.SequenceNumber)
	}

	for _, name := range []string{"gc-a", "gc-b"} {
		if _, err := database.GetProfile(t.Context(), name); err != nil {
			t.Fatalf("GetProfile(%s): %v", name, err)
		}
	}
}

func TestGroupCommitCarriesWritesBeyondByteLimit(t *testing.T) {
	database := newStandaloneDB(t)

	previous := groupCommitMaxEntryBytes
	groupCommitMaxEntryBytes = 1

	t.Cleanup(func() { groupCommitMaxEntryBytes = previous })

	batch := []*pendingWrite{
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "carry-a")),
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "carry-b")),
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "carry-c")),
	}

	carry := database.commitWrites(batch)
	if len(carry) != 2 || carry[0] != batch[1] || carry[1] != batch[2] {
		t.Fatalf("carry = %d writes, want the last 2", len(carry))
	}

	if out := outcomeOf(t, batch[0]); out.err != nil {
		t.Fatalf("first write: %v", out.err)
	}

	if len(batch[1].done) != 0 {
		t.Fatal("a carried write must not be answered before it commits")
	}

	if _, err := database.GetProfile(t.Context(), "carry-b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("carried write leaked into the database: %v", err)
	}

	for len(carry) > 0 {
		carry = database.commitWrites(carry)
	}

	for _, w := range batch[1:] {
		if out := outcomeOf(t, w); out.err != nil {
			t.Fatalf("%s: %v", w.operation, out.err)
		}
	}

	for _, name := range []string{"carry-a", "carry-b", "carry-c"} {
		if _, err := database.GetProfile(t.Context(), name); err != nil {
			t.Fatalf("GetProfile(%s): %v", name, err)
		}
	}
}

func TestGroupCommitBatchPublishesEveryOperationTopic(t *testing.T) {
	database := newStandaloneDB(t)

	profile := newTestProfile(t, "topic-profile")
	if err := database.CreateProfile(t.Context(), profile); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}

	wakeup, stop := database.Changefeed().Wakeup(TopicSessionReconcile)
	defer stop()

	updated := *profile
	updated.UeAmbrUplink = "300 Mbps"

	batch := []*pendingWrite{
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "topic-other")),
		pendingFor(t, database, opUpdateProfile, &updated),
	}

	database.commitWrites(batch)

	for _, w := range batch {
		if out := outcomeOf(t, w); out.err != nil {
			t.Fatalf("%s: %v", w.operation, out.err)
		}
	}

	select {
	case <-wakeup:
	case <-time.After(5 * time.Second):
		t.Fatal("UpdateProfile inside a batch did not wake TopicSessionReconcile subscribers")
	}
}

func TestGroupCommitConcurrentWritersShareRaftEntries(t *testing.T) {
	database := newStandaloneDB(t)

	const (
		subscribers = 8
		writers     = 128
	)

	imsis := make([]string, subscribers)
	for i := range imsis {
		imsis[i] = fmt.Sprintf("0010100000002%02d", i)
		createTestSubscriber(t, database, imsis[i])
	}

	before := database.raftManager.AppliedIndex()

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		seen = map[string]int{}
	)

	for i := range writers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			imsi := imsis[i%subscribers]

			creds, err := database.AdvanceSubscriberSQN(context.Background(), imsi, "", "")
			if err != nil {
				t.Errorf("advance %s: %v", imsi, err)
				return
			}

			mu.Lock()
			seen[imsi+"/"+creds.SequenceNumber]++
			mu.Unlock()
		}()
	}

	wg.Wait()

	if len(seen) != writers {
		t.Fatalf("got %d distinct (imsi, sequence number) pairs, want %d", len(seen), writers)
	}

	for _, imsi := range imsis {
		sub, err := database.GetSubscriber(t.Context(), imsi)
		if err != nil {
			t.Fatalf("GetSubscriber: %v", err)
		}

		want := fmt.Sprintf("%012x", (writers/subscribers)*0x20)
		if sub.SequenceNumber != want {
			t.Fatalf("%s stored sequence number = %s, want %s", imsi, sub.SequenceNumber, want)
		}
	}

	entries := database.raftManager.AppliedIndex() - before
	if entries > writers {
		t.Fatalf("%d writes used %d raft entries", writers, entries)
	}

	t.Logf("%d writes committed in %d raft entries", writers, entries)
}

func TestGroupCommitProposesWritesSeparatelyBelowBatchSchema(t *testing.T) {
	database := newStandaloneDB(t)

	database.appliedSchemaCache.Store(changesetBatchMinSchema - 1)

	t.Cleanup(func() { database.appliedSchemaCache.Store(0) })

	before := database.raftManager.AppliedIndex()

	batch := []*pendingWrite{
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "legacy-a")),
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "legacy-b")),
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "legacy-c")),
	}

	database.commitWrites(batch)

	for _, w := range batch {
		if out := outcomeOf(t, w); out.err != nil {
			t.Fatalf("%s: %v", w.operation, out.err)
		}
	}

	if got := database.raftManager.AppliedIndex() - before; got != uint64(len(batch)) {
		t.Fatalf("%d writes used %d raft entries below schema %d, want one each", len(batch), got, changesetBatchMinSchema)
	}
}

func pendingFunc(t *testing.T, operation string, applyFn func(context.Context) (any, error)) *pendingWrite {
	t.Helper()

	_, span := tracer.Start(t.Context(), "test/wait")

	return &pendingWrite{
		ctx:       t.Context(),
		operation: operation,
		applyFn:   applyFn,
		waitSpan:  span,
		done:      make(chan writeOutcome, 1),
	}
}

func TestGroupCommitHoldsDependentOutcomesUntilTheBatchCommits(t *testing.T) {
	database := newStandaloneDB(t)

	if err := database.CreateProfile(t.Context(), newTestProfile(t, "held-existing")); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}

	batch := []*pendingWrite{
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "held-existing")),
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "held-new")),
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "held-new")),
	}

	captured, schema, carry, err := database.captureBatch(batch)
	if err != nil {
		t.Fatalf("captureBatch: %v", err)
	}

	if len(carry) != 0 || len(captured) != len(batch) {
		t.Fatalf("captured %d writes and carried %d, want %d and 0", len(captured), len(carry), len(batch))
	}

	if err := database.raftManager.Shutdown(); err != nil {
		t.Fatalf("shutdown raft: %v", err)
	}

	database.proposeBatch(captured, schema)

	if out := outcomeOf(t, batch[0]); !errors.Is(out.err, ErrAlreadyExists) {
		t.Fatalf("a duplicate of a committed row must keep its own error, got %v", out.err)
	}

	for _, w := range batch[1:] {
		out := outcomeOf(t, w)
		if out.err == nil || errors.Is(out.err, ErrAlreadyExists) {
			t.Fatalf("a write in a failed batch must get the batch error, got %v", out.err)
		}
	}
}

func TestGroupCommitIsolatesAPanickingWrite(t *testing.T) {
	database := newStandaloneDB(t)

	batch := []*pendingWrite{
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "panic-a")),
		pendingFunc(t, "Panics", func(context.Context) (any, error) { panic("boom") }),
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "panic-b")),
	}

	if carry := database.commitWritesRecovering(batch); len(carry) != 0 {
		t.Fatalf("carry = %d writes, want 0", len(carry))
	}

	if out := outcomeOf(t, batch[1]); out.err == nil {
		t.Fatal("a panicking write must fail")
	}

	for _, i := range []int{0, 2} {
		if out := outcomeOf(t, batch[i]); out.err != nil {
			t.Fatalf("write %d next to a panicking write: %v", i, out.err)
		}
	}

	for _, name := range []string{"panic-a", "panic-b"} {
		if _, err := database.GetProfile(t.Context(), name); err != nil {
			t.Fatalf("GetProfile(%s): %v", name, err)
		}
	}
}

func TestGroupCommitAnswersEveryWriteWhenTheBatchPanics(t *testing.T) {
	database := newStandaloneDB(t)

	batch := []*pendingWrite{
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "batch-panic-a")),
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "batch-panic-b")),
	}

	batch[1].waitSpan = nil

	if carry := database.commitWritesRecovering(batch); len(carry) != 0 {
		t.Fatalf("carry = %d writes, want 0", len(carry))
	}

	for _, w := range batch {
		if out := outcomeOf(t, w); out.err == nil {
			t.Fatal("every write of a panicking batch must fail")
		}
	}

	if err := database.CreateProfile(t.Context(), newTestProfile(t, "after-panic")); err != nil {
		t.Fatalf("write after a panicking batch: %v", err)
	}
}

func TestGroupCommitRecoversWhenAWriteEndsTheTransaction(t *testing.T) {
	database := newStandaloneDB(t)

	batch := []*pendingWrite{
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "abort-a")),
		pendingFunc(t, "EndsTransaction", func(ctx context.Context) (any, error) {
			_, err := database.runner(ctx).PlainDB().ExecContext(ctx, "ROLLBACK")
			return nil, err
		}),
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "abort-b")),
	}

	carry := database.commitWrites(batch)
	if len(carry) != 1 || carry[0] != batch[2] {
		t.Fatalf("carry = %d writes, want the write after the one that ended the transaction", len(carry))
	}

	if out := outcomeOf(t, batch[0]); out.err != nil {
		t.Fatalf("write captured before the transaction ended: %v", out.err)
	}

	if out := outcomeOf(t, batch[1]); out.err == nil {
		t.Fatal("the write that ended the transaction must fail")
	}

	for len(carry) > 0 {
		carry = database.commitWrites(carry)
	}

	if out := outcomeOf(t, batch[2]); out.err != nil {
		t.Fatalf("carried write: %v", out.err)
	}

	for _, name := range []string{"abort-a", "abort-b"} {
		if _, err := database.GetProfile(t.Context(), name); err != nil {
			t.Fatalf("GetProfile(%s): %v", name, err)
		}
	}
}

func TestGroupCommitCarriesWritesBeyondTheCaptureBudget(t *testing.T) {
	database := newStandaloneDB(t)

	previous := groupCommitCaptureBudget
	groupCommitCaptureBudget = 0

	t.Cleanup(func() { groupCommitCaptureBudget = previous })

	batch := []*pendingWrite{
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "budget-a")),
		pendingFor(t, database, opCreateProfile, newTestProfile(t, "budget-b")),
	}

	carry := database.commitWrites(batch)
	if len(carry) != 1 || carry[0] != batch[1] {
		t.Fatalf("carry = %d writes, want the second", len(carry))
	}

	if len(batch[1].done) != 0 {
		t.Fatal("a write past the capture budget must not run")
	}

	if _, err := database.GetProfile(t.Context(), "budget-b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("write past the capture budget leaked: %v", err)
	}
}

func TestGroupCommitRejectsWritesWhenTheQueueIsFull(t *testing.T) {
	database := newStandaloneDB(t)

	previous := groupCommitMaxQueue
	groupCommitMaxQueue = 0

	t.Cleanup(func() { groupCommitMaxQueue = previous })

	err := database.CreateProfile(t.Context(), newTestProfile(t, "queue-full"))
	if !errors.Is(err, ErrProposeTimeout) {
		t.Fatalf("a full write queue must return a retryable error, got %v", err)
	}
}
