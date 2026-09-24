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

func TestChangefeed_PublishWakesMatchingSubscriber(t *testing.T) {
	cf := db.NewChangefeed()

	wakeup, stop := cf.Wakeup(db.TopicNATSettings)
	defer stop()

	cf.Publish(db.TopicNATSettings)

	expectWakeup(t, wakeup)
}

func TestChangefeed_TopicIsolation(t *testing.T) {
	cf := db.NewChangefeed()

	wakeupA, stopA := cf.Wakeup(db.TopicFlowAccountingSettings)
	defer stopA()

	wakeupB, stopB := cf.Wakeup(db.TopicNATSettings)
	defer stopB()

	cf.Publish(db.TopicFlowAccountingSettings)

	expectWakeup(t, wakeupA)
	expectNoWakeup(t, wakeupB)
}

func TestChangefeed_MultiTopicSubscriber(t *testing.T) {
	cf := db.NewChangefeed()

	wakeup, stop := cf.Wakeup(db.TopicNATSettings, db.TopicFlowAccountingSettings)
	defer stop()

	cf.Publish(db.TopicNATSettings)
	expectWakeup(t, wakeup)

	cf.Publish(db.TopicFlowAccountingSettings)
	expectWakeup(t, wakeup)
}

func TestChangefeed_BurstCoalescesWithoutBlocking(t *testing.T) {
	cf := db.NewChangefeed()

	wakeup, stop := cf.Wakeup(db.TopicNATSettings)
	defer stop()

	done := make(chan struct{})

	go func() {
		for range 256 {
			cf.Publish(db.TopicNATSettings)
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

	wakeup, stop := cf.Wakeup(db.TopicNATSettings)
	defer stop()

	cf.Publish(db.TopicNATSettings)
	expectWakeup(t, wakeup)

	cf.Publish(db.TopicNATSettings)
	expectWakeup(t, wakeup)
}

func TestChangefeed_StopStopsDelivery(t *testing.T) {
	cf := db.NewChangefeed()

	wakeup, stop := cf.Wakeup(db.TopicNATSettings)
	stop()

	cf.Publish(db.TopicNATSettings)

	expectNoWakeup(t, wakeup)
}

func TestChangefeed_PublishWithNoSubscribersIsNoop(t *testing.T) {
	cf := db.NewChangefeed()

	done := make(chan struct{})

	go func() {
		cf.Publish(db.TopicNATSettings)
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
				wakeup, cancel := cf.Wakeup(db.TopicNATSettings)

				select {
				case <-wakeup:
				case <-time.After(time.Millisecond):
				}

				cancel()
			}
		})

		wg.Go(func() {
			for !stop.Load() {
				cf.Publish(db.TopicNATSettings)
			}
		})
	}

	time.Sleep(100 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
}
