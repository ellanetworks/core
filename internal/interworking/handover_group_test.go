// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/interworking"
)

func TestHandoverGroupAwaitReturnsOnAnIdleGroup(t *testing.T) {
	var g interworking.HandoverGroup

	if err := g.Await(t.Context()); err != nil {
		t.Fatalf("an idle group did not settle: %v", err)
	}
}

func TestHandoverGroupAwaitWaitsForTheInFlightHandover(t *testing.T) {
	var g interworking.HandoverGroup

	release := make(chan struct{})
	ran := make(chan struct{})

	g.Go(func() {
		<-release
		close(ran)
	})

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	if err := g.Await(ctx); err == nil {
		t.Fatal("the group settled while a handover was still running")
	}

	close(release)

	if err := g.Await(t.Context()); err != nil {
		t.Fatalf("the group did not settle after the handover finished: %v", err)
	}

	<-ran
}

func TestHandoverGroupAwaitsASecondRoundOfHandovers(t *testing.T) {
	var g interworking.HandoverGroup

	var ran atomic.Int32

	for range 2 {
		g.Go(func() { ran.Add(1) })

		if err := g.Await(t.Context()); err != nil {
			t.Fatalf("the group did not settle: %v", err)
		}
	}

	if got := ran.Load(); got != 2 {
		t.Errorf("%d handovers ran, want 2", got)
	}
}

func TestHandoverGroupToleratesDispatchRacingTheAwait(t *testing.T) {
	var g interworking.HandoverGroup

	var ran atomic.Int32

	var dispatching sync.WaitGroup

	for range 64 {
		dispatching.Go(func() {
			g.Go(func() { ran.Add(1) })
		})
	}

	if err := g.Await(t.Context()); err != nil {
		t.Fatalf("the await did not return: %v", err)
	}

	dispatching.Wait()

	if err := g.Await(t.Context()); err != nil {
		t.Fatalf("the group did not settle: %v", err)
	}

	if got := ran.Load(); got != 64 {
		t.Errorf("%d handovers ran, want 64", got)
	}
}
