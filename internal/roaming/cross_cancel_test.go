// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/roaming"
)

func TestCrossCancelGuardPurgesWithoutACancellation(t *testing.T) {
	var g roaming.CrossCancelGuard

	g.SetWait(10 * time.Millisecond)

	fired := make(chan struct{})

	g.Arm("001010000000001", "p", func() { close(fired) })

	if partner, ok := g.Partner("001010000000001"); !ok || partner != "p" {
		t.Fatalf("Partner = %q, %v; want the armed partner", partner, ok)
	}

	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("the guard did not purge after its wait")
	}

	if _, ok := g.Partner("001010000000001"); ok {
		t.Fatal("the guard is still pending after it fired")
	}
}

func TestCrossCancelGuardDisarmAndRearm(t *testing.T) {
	var g roaming.CrossCancelGuard

	g.SetWait(20 * time.Millisecond)

	var first, second atomic.Int32

	g.Arm("001010000000001", "p", func() { first.Add(1) })

	if !g.Disarm("001010000000001") || g.Disarm("001010000000001") {
		t.Fatal("Disarm must succeed once for a pending guard")
	}

	g.Arm("001010000000002", "p", func() { first.Add(1) })
	g.Arm("001010000000002", "q", func() { second.Add(1) })

	time.Sleep(100 * time.Millisecond)

	if first.Load() != 0 || second.Load() != 1 {
		t.Fatalf("purges = %d stale, %d current; want only the re-armed purge", first.Load(), second.Load())
	}
}
