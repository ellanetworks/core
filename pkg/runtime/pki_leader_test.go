// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var errLeaderInitBoom = errors.New("boom")

type fakeLeaderDB struct {
	mu sync.Mutex

	restoreErr  error
	transferErr error

	restores  int
	transfers int
}

func (f *fakeLeaderDB) SetRestoreErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.restoreErr = err
}

func (f *fakeLeaderDB) SelfRestore(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.restores++

	return f.restoreErr
}

func (f *fakeLeaderDB) LeadershipTransfer() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.transfers++

	return f.transferErr
}

func (f *fakeLeaderDB) restoreCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.restores
}

func (f *fakeLeaderDB) transferCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.transfers
}

func shortBackoff(t *testing.T) {
	t.Helper()

	initial, maxBackoff := leaderInitInitialBackoff, leaderInitMaxBackoff
	leaderInitInitialBackoff = time.Millisecond
	leaderInitMaxBackoff = 5 * time.Millisecond

	t.Cleanup(func() {
		leaderInitInitialBackoff, leaderInitMaxBackoff = initial, maxBackoff
	})
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("condition not met before deadline")
}

func runInBackground(ctx context.Context, l *pkiLeader) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		l.run(ctx)
	}()

	return done
}

func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("leader hook did not return before deadline")
	}
}

func TestSelfRestoreFailureKeepsLeadershipAndRetries(t *testing.T) {
	shortBackoff(t)

	fdb := &fakeLeaderDB{restoreErr: errLeaderInitBoom}

	var inits atomic.Int32

	l := &pkiLeader{
		db: fdb,
		runInit: func(context.Context) error {
			inits.Add(1)

			return nil
		},
	}
	l.needsDRSnapshot.Store(true)

	done := runInBackground(t.Context(), l)

	waitFor(t, func() bool { return fdb.restoreCount() >= 1 })

	if inits.Load() != 0 {
		t.Fatal("leader init ran before the DR baseline was installed")
	}

	fdb.SetRestoreErr(nil)

	waitDone(t, done)

	if inits.Load() != 1 {
		t.Fatalf("expected leader init to run once after self-restore recovered, got %d", inits.Load())
	}

	if restores := fdb.restoreCount(); restores < 2 {
		t.Fatalf("expected the retry loop to re-run self-restore, got %d calls", restores)
	}

	if l.needsDRSnapshot.Load() {
		t.Fatal("needsDRSnapshot still set after a successful self-restore")
	}

	if transfers := fdb.transferCount(); transfers != 0 {
		t.Fatalf("self-restore failure yielded leadership: %d transfers", transfers)
	}
}

func TestLeaderInitFailureYieldsLeadershipWithoutRetrying(t *testing.T) {
	shortBackoff(t)

	fdb := &fakeLeaderDB{}

	var inits atomic.Int32

	l := &pkiLeader{
		db: fdb,
		runInit: func(context.Context) error {
			inits.Add(1)

			return errLeaderInitBoom
		},
	}

	waitDone(t, runInBackground(t.Context(), l))

	if transfers := fdb.transferCount(); transfers != 1 {
		t.Fatalf("expected one leadership transfer, got %d", transfers)
	}

	if inits.Load() != 1 {
		t.Fatalf("leader init ran %d times after yielding leadership", inits.Load())
	}
}

func TestLeaderInitRetriesWhenLeadershipTransferFails(t *testing.T) {
	shortBackoff(t)

	fdb := &fakeLeaderDB{transferErr: errLeaderInitBoom}

	var inits atomic.Int32

	l := &pkiLeader{
		db: fdb,
		runInit: func(context.Context) error {
			if inits.Add(1) < 3 {
				return errLeaderInitBoom
			}

			return nil
		},
	}

	waitDone(t, runInBackground(t.Context(), l))

	if inits.Load() != 3 {
		t.Fatalf("expected the retry loop to run init until it recovered, got %d calls", inits.Load())
	}

	if transfers := fdb.transferCount(); transfers != 1 {
		t.Fatalf("expected one leadership transfer attempt before retrying, got %d", transfers)
	}
}

func TestLeaderInitRetryStopsWhenLeadershipIsLost(t *testing.T) {
	shortBackoff(t)

	fdb := &fakeLeaderDB{transferErr: errLeaderInitBoom}

	var inits atomic.Int32

	l := &pkiLeader{
		db: fdb,
		runInit: func(context.Context) error {
			inits.Add(1)

			return errLeaderInitBoom
		},
	}

	ctx, cancel := context.WithCancel(t.Context())

	done := runInBackground(ctx, l)

	waitFor(t, func() bool { return inits.Load() > 1 })

	cancel()

	waitDone(t, done)
}

func TestSuccessiveLeadershipTermsSelfRestoreOnce(t *testing.T) {
	shortBackoff(t)

	fdb := &fakeLeaderDB{}

	var inits atomic.Int32

	l := &pkiLeader{
		db: fdb,
		runInit: func(context.Context) error {
			inits.Add(1)

			return nil
		},
	}
	l.needsDRSnapshot.Store(true)

	waitDone(t, runInBackground(t.Context(), l))
	waitDone(t, runInBackground(t.Context(), l))

	if restores := fdb.restoreCount(); restores != 1 {
		t.Fatalf("expected self-restore to run once across two terms, got %d", restores)
	}

	if inits.Load() != 2 {
		t.Fatalf("expected leader init to run once per term, got %d", inits.Load())
	}

	if l.needsDRSnapshot.Load() {
		t.Fatal("needsDRSnapshot set again after a successful self-restore")
	}
}

func TestSelfRestoreRetryStopsWhenLeadershipIsLost(t *testing.T) {
	shortBackoff(t)

	fdb := &fakeLeaderDB{restoreErr: errLeaderInitBoom}

	var inits atomic.Int32

	l := &pkiLeader{
		db: fdb,
		runInit: func(context.Context) error {
			inits.Add(1)

			return nil
		},
	}
	l.needsDRSnapshot.Store(true)

	ctx, cancel := context.WithCancel(t.Context())

	done := runInBackground(ctx, l)

	waitFor(t, func() bool { return fdb.restoreCount() > 1 })

	cancel()

	waitDone(t, done)

	if transfers := fdb.transferCount(); transfers != 0 {
		t.Fatalf("pending self-restore yielded leadership: %d transfers", transfers)
	}

	if inits.Load() != 0 {
		t.Fatalf("leader init ran %d times without a DR baseline", inits.Load())
	}

	if !l.needsDRSnapshot.Load() {
		t.Fatal("needsDRSnapshot cleared although self-restore never succeeded")
	}
}
