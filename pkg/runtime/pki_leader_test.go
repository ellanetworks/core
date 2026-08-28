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

	leader      bool
	restoreErr  error
	transferErr error

	restores  int
	transfers int
}

func (f *fakeLeaderDB) IsLeader() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.leader
}

func (f *fakeLeaderDB) SetLeader(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.leader = v
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

	if f.transferErr == nil {
		f.leader = false
	}

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

func (c *pkiLeaderCallback) termDoneCh() chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.termDone
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

func TestSelfRestoreFailureKeepsLeadershipAndRetries(t *testing.T) {
	shortBackoff(t)

	fdb := &fakeLeaderDB{leader: true, restoreErr: errLeaderInitBoom}

	var inits atomic.Int32

	c := &pkiLeaderCallback{
		ctx:             context.Background(),
		db:              fdb,
		needsDRSnapshot: true,
		runInit: func(context.Context) error {
			inits.Add(1)

			return nil
		},
	}

	c.OnBecameLeader()

	if transfers := fdb.transferCount(); transfers != 0 {
		t.Fatalf("self-restore failure yielded leadership: %d transfers", transfers)
	}

	if inits.Load() != 0 {
		t.Fatal("leader init ran before the DR baseline was installed")
	}

	fdb.SetRestoreErr(nil)

	waitFor(t, func() bool { return inits.Load() == 1 })

	if restores := fdb.restoreCount(); restores < 2 {
		t.Fatalf("expected the retry loop to re-run self-restore, got %d calls", restores)
	}

	<-c.termDoneCh()

	c.mu.Lock()
	pending := c.needsDRSnapshot
	c.mu.Unlock()

	if pending {
		t.Fatal("needsDRSnapshot still set after a successful self-restore")
	}

	if transfers := fdb.transferCount(); transfers != 0 {
		t.Fatalf("self-restore retry yielded leadership: %d transfers", transfers)
	}
}

func TestLeaderInitFailureYieldsLeadershipWithoutRetrying(t *testing.T) {
	shortBackoff(t)

	fdb := &fakeLeaderDB{leader: true}

	var inits atomic.Int32

	c := &pkiLeaderCallback{
		ctx: context.Background(),
		db:  fdb,
		runInit: func(context.Context) error {
			inits.Add(1)

			return errLeaderInitBoom
		},
	}

	c.OnBecameLeader()

	if transfers := fdb.transferCount(); transfers != 1 {
		t.Fatalf("expected one leadership transfer, got %d", transfers)
	}

	if done := c.termDoneCh(); done != nil {
		t.Fatal("retry goroutine started after a successful leadership transfer")
	}

	time.Sleep(20 * time.Millisecond)

	if inits.Load() != 1 {
		t.Fatalf("leader init ran %d times after yielding leadership", inits.Load())
	}
}

func TestLeaderInitRetriesWhenLeadershipTransferFails(t *testing.T) {
	shortBackoff(t)

	fdb := &fakeLeaderDB{leader: true, transferErr: errLeaderInitBoom}

	var inits atomic.Int32

	c := &pkiLeaderCallback{
		ctx: context.Background(),
		db:  fdb,
		runInit: func(context.Context) error {
			if inits.Add(1) < 3 {
				return errLeaderInitBoom
			}

			return nil
		},
	}

	c.OnBecameLeader()

	<-c.termDoneCh()

	if inits.Load() != 3 {
		t.Fatalf("expected the retry loop to run init until it recovered, got %d calls", inits.Load())
	}
}

func TestLeaderInitRetryStopsWhenLeadershipIsLost(t *testing.T) {
	shortBackoff(t)

	fdb := &fakeLeaderDB{leader: true, transferErr: errLeaderInitBoom}

	var inits atomic.Int32

	c := &pkiLeaderCallback{
		ctx: context.Background(),
		db:  fdb,
		runInit: func(context.Context) error {
			inits.Add(1)

			return errLeaderInitBoom
		},
	}

	c.OnBecameLeader()

	waitFor(t, func() bool { return inits.Load() > 1 })

	fdb.SetLeader(false)

	<-c.termDoneCh()

	settled := inits.Load()

	time.Sleep(20 * time.Millisecond)

	if inits.Load() != settled {
		t.Fatal("retry loop kept running after leadership was lost")
	}
}

func TestLeaderTermsDoNotOverlap(t *testing.T) {
	shortBackoff(t)

	fdb := &fakeLeaderDB{leader: true, transferErr: errLeaderInitBoom}

	var (
		inFlight   atomic.Int32
		overlapped atomic.Bool
	)

	c := &pkiLeaderCallback{
		ctx: context.Background(),
		db:  fdb,
		runInit: func(context.Context) error {
			if inFlight.Add(1) > 1 {
				overlapped.Store(true)
			}

			time.Sleep(2 * time.Millisecond)
			inFlight.Add(-1)

			return errLeaderInitBoom
		},
	}

	for range 20 {
		c.OnBecameLeader()
		time.Sleep(time.Millisecond)
	}

	c.OnLostLeadership()
	<-c.termDoneCh()

	if overlapped.Load() {
		t.Fatal("two leader sequences ran concurrently")
	}
}
