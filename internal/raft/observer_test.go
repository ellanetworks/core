// Copyright 2026 Ella Networks

package raft

import (
	"sync/atomic"
	"testing"
	"time"
)

type testCallback struct {
	becameLeader atomic.Int32
	lostLeader   atomic.Int32
}

func (c *testCallback) OnBecameLeader()   { c.becameLeader.Add(1) }
func (c *testCallback) OnLostLeadership() { c.lostLeader.Add(1) }

func TestLeaderObserver_SingleServer(t *testing.T) {
	applier := newTestApplier(t)

	mgr, cleanup := NewTestManager(t, applier)
	defer cleanup()

	cb := &testCallback{}
	mgr.LeaderObserver().Register(cb)

	// The observer Run goroutine was started inside NewTestManager.
	// In single-server the node is immediately leader, so the observer
	// should fire OnBecameLeader within a short window.
	deadline := time.After(2 * time.Second)

	for cb.becameLeader.Load() < 1 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for OnBecameLeader")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if got := cb.lostLeader.Load(); got != 0 {
		t.Fatalf("expected 0 OnLostLeadership calls, got %d", got)
	}

	if !mgr.LeaderObserver().IsLeader() {
		t.Fatal("expected IsLeader() == true")
	}
}
