// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf_test

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
)

func TestTimerIsActiveNewTimer(t *testing.T) {
	timer := amf.NewTimer(1*time.Hour, 1, func(_ int32) {}, func() {})
	defer timer.Stop()

	if !timer.IsActive() {
		t.Fatal("expected new timer to be active, got inactive")
	}
}

func TestTimerIsActiveAfterStop(t *testing.T) {
	timer := amf.NewTimer(1*time.Hour, 1, func(_ int32) {}, func() {})
	timer.Stop()

	if timer.IsActive() {
		t.Fatal("expected stopped timer to be inactive, got active")
	}
}

func TestTimerIsActiveAutoStop(t *testing.T) {
	done := make(chan struct{})
	timer := amf.NewTimer(10*time.Millisecond, 1, func(_ int32) {}, func() {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timer did not auto-stop within timeout")
	}

	// Give the goroutine time to set active=0 before we check
	time.Sleep(20 * time.Millisecond)

	if timer.IsActive() {
		t.Fatal("expected auto-stopped timer to be inactive, got active")
	}
}
