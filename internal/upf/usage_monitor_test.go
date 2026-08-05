// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"context"
	"testing"
	"time"
)

// At shutdown the counters since the last tick must be drained, and the poller
// must be finished before Close tears down the maps it reads.

// TestStopUsageMonitorWaitsForExit: Close depends on this ordering, since a
// select with both a ready tick and a ready stop picks uniformly.
func TestStopUsageMonitorWaitsForExit(t *testing.T) {
	u := &UPF{ctx: context.Background()}

	// A long interval, so the only thing that ends the poller is the stop —
	// which is what the wait has to observe.
	u.startUsageMonitor(context.Background(), time.Hour)

	done := u.usageDone

	select {
	case <-done:
		t.Fatal("usage monitor exited before it was stopped")
	default:
	}

	u.stopUsageMonitor()

	select {
	case <-done:
	default:
		t.Error("stopUsageMonitor returned while the usage monitor was still running")
	}
}

// TestStopUsageMonitorWithoutStart checks the nil case: Close runs even when
// Start bailed before the poller was launched.
func TestStopUsageMonitorWithoutStart(t *testing.T) {
	u := &UPF{}

	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		u.stopUsageMonitor()
	}()

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("stopUsageMonitor blocked when no monitor had been started")
	}
}

// TestMonitorUsageReturnsWithinTheFlushBudget pins that the shutdown flush
// cannot hang. It does not assert the counters drained: with no session engine
// there is nothing observable to drain.
func TestMonitorUsageReturnsWithinTheFlushBudget(t *testing.T) {
	u := &UPF{ctx: context.Background()}

	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		u.monitorUsage(time.Hour, stop)
	}()

	close(stop)

	select {
	case <-done:
	case <-time.After(usageFlushTimeout + 5*time.Second):
		t.Fatal("monitorUsage did not return after stop")
	}
}
