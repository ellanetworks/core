// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"context"
	"testing"
	"time"
)

func TestStopUsageMonitorWaitsForExit(t *testing.T) {
	u := &UPF{ctx: context.Background()}

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
