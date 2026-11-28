// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func expectWakeup(t *testing.T, wakeup <-chan struct{}) {
	t.Helper()

	select {
	case <-wakeup:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for wakeup")
	}
}

func expectNoWakeup(t *testing.T, wakeup <-chan struct{}) {
	t.Helper()

	select {
	case <-wakeup:
		t.Fatal("did not expect a wakeup")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestChangefeed_TopicIsolation(t *testing.T) {
	cf := db.NewChangefeed()

	wakeupA, stopA := cf.Wakeup(db.TopicFlowAccountingSettings)
	defer stopA()

	wakeupB, stopB := cf.Wakeup(db.TopicN3Settings)
	defer stopB()

	cf.Publish(db.TopicFlowAccountingSettings, 0)

	expectWakeup(t, wakeupA)
	expectNoWakeup(t, wakeupB)
}

func TestChangefeed_MultiTopicSubscriber(t *testing.T) {
	cf := db.NewChangefeed()

	wakeup, stop := cf.Wakeup(db.TopicN3Settings, db.TopicFlowAccountingSettings)
	defer stop()

	cf.Publish(db.TopicN3Settings, 0)
	expectWakeup(t, wakeup)

	cf.Publish(db.TopicFlowAccountingSettings, 0)
	expectWakeup(t, wakeup)
}

func TestChangefeed_BurstCoalescesWithoutBlocking(t *testing.T) {
	cf := db.NewChangefeed()

	wakeup, stop := cf.Wakeup(db.TopicN3Settings)
	defer stop()

	done := make(chan struct{})

	go func() {
		for range 256 {
			cf.Publish(db.TopicN3Settings, 0)
		}

		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher blocked on an undrained subscriber")
	}

	expectWakeup(t, wakeup)
	expectNoWakeup(t, wakeup)
}

func TestChangefeed_PublishDuringReconcileWakesAgain(t *testing.T) {
	cf := db.NewChangefeed()

	wakeup, stop := cf.Wakeup(db.TopicN3Settings)
	defer stop()

	cf.Publish(db.TopicN3Settings, 0)
	expectWakeup(t, wakeup)

	cf.Publish(db.TopicN3Settings, 0)
	expectWakeup(t, wakeup)
}

func TestChangefeed_StopStopsDelivery(t *testing.T) {
	cf := db.NewChangefeed()

	wakeup, stop := cf.Wakeup(db.TopicN3Settings)
	stop()

	cf.Publish(db.TopicN3Settings, 0)

	expectNoWakeup(t, wakeup)
}

func TestChangefeed_PublishWithNoSubscribersIsNoop(t *testing.T) {
	cf := db.NewChangefeed()

	done := make(chan struct{})

	go func() {
		cf.Publish(db.TopicN3Settings, 0)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked with no subscribers")
	}
}

func TestChangefeed_ConcurrentPublishSubscribeStop(t *testing.T) {
	cf := db.NewChangefeed()

	const workers = 8

	var (
		wg   sync.WaitGroup
		stop atomic.Bool
	)

	for range workers {
		wg.Go(func() {
			for !stop.Load() {
				wakeup, cancel := cf.Wakeup(db.TopicN3Settings)

				select {
				case <-wakeup:
				case <-time.After(time.Millisecond):
				}

				cancel()
			}
		})

		wg.Go(func() {
			for !stop.Load() {
				cf.Publish(db.TopicN3Settings, 0)
			}
		})
	}

	time.Sleep(100 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
}

func TestChangefeed_RealizedFollowsTrackers(t *testing.T) {
	cf := db.NewChangefeed()

	tracker := cf.Track(db.TopicDataNetworks)
	defer tracker.Stop()

	if got := cf.Realized(5); got != 0 {
		t.Fatalf("Realized before Arm = %d, want 0", got)
	}

	cf.Arm()

	if got := cf.Realized(5); got != 0 {
		t.Fatalf("Realized before the first pass = %d, want 0", got)
	}

	tracker.Done(tracker.Begin())

	if got := cf.Realized(5); got != 5 {
		t.Fatalf("Realized after the first pass = %d, want 5", got)
	}

	cf.Publish(db.TopicDataNetworks, 7)
	expectWakeup(t, tracker.C())

	if got := cf.Realized(7); got != 5 {
		t.Fatalf("Realized with a pending write = %d, want 5, the last index realized", got)
	}

	target := tracker.Begin()
	if target != 7 {
		t.Fatalf("Begin() = %d, want 7", target)
	}

	cf.Publish(db.TopicN3Settings, 9)

	if got := cf.Realized(9); got != 5 {
		t.Fatalf("Realized = %d, want 5 while the pass runs", got)
	}

	tracker.Done(target)

	if got := cf.Realized(9); got != 9 {
		t.Fatalf("Realized after the pass = %d, want 9: the write at 9 does not concern the tracker", got)
	}

	cf.Publish(db.TopicDataNetworks, 12)

	if got := cf.Realized(12); got != 9 {
		t.Fatalf("Realized = %d, want it to stay at 9 rather than drop to the last pass at 7", got)
	}

	tracker.Done(tracker.Begin())
	cf.PublishAll(14)

	if got := cf.Realized(14); got != 12 {
		t.Fatalf("Realized after a restore = %d, want 12, the index the last pass started at", got)
	}

	tracker.Stop()

	if got := cf.Realized(14); got != 0 {
		t.Fatalf("Realized after a tracker stopped = %d, want 0", got)
	}
}
