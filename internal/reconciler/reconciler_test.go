// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeFeed struct {
	wakeup chan struct{}
	target atomic.Uint64
	done   atomic.Uint64
}

func (f *fakeFeed) C() <-chan struct{} { return f.wakeup }

func (f *fakeFeed) Begin() uint64 { return f.target.Load() }

func (f *fakeFeed) Done(index uint64) { f.done.Store(index) }

func TestReconcilerSweepsOnWakeup(t *testing.T) {
	first, second := make(chan struct{}, 4), make(chan struct{}, 4)
	feed := &fakeFeed{wakeup: make(chan struct{})}
	wakeup := feed.wakeup

	r := New(feed,
		func(context.Context) error { first <- struct{}{}; return nil },
		func(context.Context) error { second <- struct{}{}; return nil },
	)
	r.backstop = time.Hour

	r.Start()
	defer r.Stop()

	for _, sweep := range []chan struct{}{first, second} {
		select {
		case <-sweep:
		case <-time.After(2 * time.Second):
			t.Fatal("the initial reconcile did not run every sweep")
		}
	}

	feed.target.Store(7)

	wakeup <- struct{}{}

	for _, sweep := range []chan struct{}{first, second} {
		select {
		case <-sweep:
		case <-time.After(2 * time.Second):
			t.Fatal("a wakeup did not run every sweep")
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for feed.done.Load() != 7 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if got := feed.done.Load(); got != 7 {
		t.Fatalf("Done(%d) after the sweep, want Done(7)", got)
	}
}

func TestReconcilerHoldsTheRevisionWhenASweepFails(t *testing.T) {
	feed := &fakeFeed{wakeup: make(chan struct{})}
	feed.target.Store(7)

	swept := make(chan struct{}, 4)

	r := New(feed, func(context.Context) error {
		swept <- struct{}{}
		return errors.New("policy unavailable")
	})
	r.backstop = time.Hour

	r.Start()
	defer r.Stop()

	select {
	case <-swept:
	case <-time.After(2 * time.Second):
		t.Fatal("the initial reconcile did not run")
	}

	r.Stop()

	if got := feed.done.Load(); got != 0 {
		t.Fatalf("Done(%d) after a failed sweep, want no Done", got)
	}
}
