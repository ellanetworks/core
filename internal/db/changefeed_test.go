// Copyright 2026 Ella Networks

package db_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func receiveEvent(t *testing.T, sub *db.Subscription, want db.Topic, wantIndex uint64) {
	t.Helper()

	select {
	case ev := <-sub.Events:
		if ev.Topic != want {
			t.Fatalf("expected topic %q, got %q", want, ev.Topic)
		}

		if ev.Index != wantIndex {
			t.Fatalf("expected index %d, got %d", wantIndex, ev.Index)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for event on %q", want)
	}
}

func expectNoEvent(t *testing.T, sub *db.Subscription) {
	t.Helper()

	select {
	case ev := <-sub.Events:
		t.Fatalf("did not expect event, got %+v", ev)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestChangefeed_PublishDeliversToMatchingSubscriber(t *testing.T) {
	cf := db.NewChangefeed()

	sub := cf.Subscribe(db.TopicRoutes)
	defer sub.Close()

	cf.Publish(db.TopicRoutes, 42)

	receiveEvent(t, sub, db.TopicRoutes, 42)
}

func TestChangefeed_TopicIsolation(t *testing.T) {
	cf := db.NewChangefeed()

	subA := cf.Subscribe(db.TopicRoutes)
	defer subA.Close()

	subB := cf.Subscribe(db.TopicNATSettings)
	defer subB.Close()

	cf.Publish(db.TopicRoutes, 1)

	receiveEvent(t, subA, db.TopicRoutes, 1)
	expectNoEvent(t, subB)
}

func TestChangefeed_MultiTopicSubscriber(t *testing.T) {
	cf := db.NewChangefeed()

	sub := cf.Subscribe(db.TopicRoutes, db.TopicNATSettings)
	defer sub.Close()

	cf.Publish(db.TopicRoutes, 1)
	cf.Publish(db.TopicNATSettings, 2)

	receiveEvent(t, sub, db.TopicRoutes, 1)
	receiveEvent(t, sub, db.TopicNATSettings, 2)
}

func TestChangefeed_RingOverflowFiresDroppedAndDoesNotBlock(t *testing.T) {
	cf := db.NewChangefeed()

	sub := cf.Subscribe(db.TopicRoutes)
	defer sub.Close()

	// Publisher must remain non-blocking even if a subscriber never
	// drains its Events channel. We push enough events to overflow
	// the buffer (capacity 128) and assert Dropped fires.
	const overflow = 256

	done := make(chan struct{})

	go func() {
		for i := 0; i < overflow; i++ {
			cf.Publish(db.TopicRoutes, uint64(i))
		}

		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher blocked despite full subscriber buffer")
	}

	select {
	case <-sub.Dropped:
	case <-time.After(time.Second):
		t.Fatal("expected Dropped to fire after overflow")
	}
}

func TestChangefeed_DroppedDoesNotBlockSubsequentPublishes(t *testing.T) {
	cf := db.NewChangefeed()

	sub := cf.Subscribe(db.TopicRoutes)
	defer sub.Close()

	for i := 0; i < 256; i++ {
		cf.Publish(db.TopicRoutes, uint64(i))
	}

	// Even after Dropped is pending, further Publish calls must not
	// block; the publisher just keeps trying to enqueue and harmlessly
	// no-ops on the dropped signal.
	done := make(chan struct{})

	go func() {
		for i := 0; i < 100; i++ {
			cf.Publish(db.TopicRoutes, uint64(i+1000))
		}

		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publisher blocked on subsequent publish after overflow")
	}
}

func TestChangefeed_CloseStopsDelivery(t *testing.T) {
	cf := db.NewChangefeed()

	sub := cf.Subscribe(db.TopicRoutes)
	sub.Close()

	cf.Publish(db.TopicRoutes, 1)

	expectNoEvent(t, sub)
}

func TestChangefeed_CloseIsIdempotent(t *testing.T) {
	cf := db.NewChangefeed()

	sub := cf.Subscribe(db.TopicRoutes)
	sub.Close()
	sub.Close()
}

func TestChangefeed_PublishWithNoSubscribersIsNoop(t *testing.T) {
	cf := db.NewChangefeed()

	done := make(chan struct{})

	go func() {
		cf.Publish(db.TopicRoutes, 1)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked with no subscribers")
	}
}

func TestChangefeed_ConcurrentPublishSubscribeClose(t *testing.T) {
	cf := db.NewChangefeed()

	const workers = 8

	var (
		wg       sync.WaitGroup
		stop     atomic.Bool
		received atomic.Uint64
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for !stop.Load() {
				sub := cf.Subscribe(db.TopicRoutes)

				go func() {
					for {
						select {
						case <-sub.Events:
							received.Add(1)
						case <-sub.Dropped:
						case <-time.After(10 * time.Millisecond):
							return
						}
					}
				}()

				time.Sleep(time.Millisecond)
				sub.Close()
			}
		}()
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for !stop.Load() {
				cf.Publish(db.TopicRoutes, 1)
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
}

func TestChangefeed_WakeupCoalesces(t *testing.T) {
	cf := db.NewChangefeed()

	wakeup, stop := cf.Wakeup(db.TopicRoutes)
	defer stop()

	// Burst of publishes coalesces to a single wakeup, possibly two
	// if the bridge goroutine drained the first by the time we
	// publish more.
	for i := 0; i < 10; i++ {
		cf.Publish(db.TopicRoutes, uint64(i))
	}

	select {
	case <-wakeup:
	case <-time.After(time.Second):
		t.Fatal("expected at least one wakeup after publish burst")
	}
}

func TestChangefeed_WakeupStopReleasesResources(t *testing.T) {
	cf := db.NewChangefeed()

	wakeup, stop := cf.Wakeup(db.TopicRoutes)

	stop()

	cf.Publish(db.TopicRoutes, 1)

	select {
	case <-wakeup:
		// A wakeup raced through before stop; harmless.
	case <-time.After(50 * time.Millisecond):
	}
}

func TestChangefeed_EventOrderingPerSubscriber(t *testing.T) {
	cf := db.NewChangefeed()

	sub := cf.Subscribe(db.TopicRoutes)
	defer sub.Close()

	const n = 50

	for i := uint64(1); i <= n; i++ {
		cf.Publish(db.TopicRoutes, i)
	}

	for i := uint64(1); i <= n; i++ {
		receiveEvent(t, sub, db.TopicRoutes, i)
	}
}
