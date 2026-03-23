// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"sync"
	"testing"

	"go.uber.org/zap"
)

func TestTransitionTo_AllowedTransitions(t *testing.T) {
	// Every edge declared in validTransitions must succeed.
	for from, targets := range validTransitions {
		for _, to := range targets {
			ue := NewAmfUe()
			ue.Log = zap.NewNop()
			ue.state = from

			ue.TransitionTo(to)

			if got := ue.GetState(); got != to {
				t.Errorf("TransitionTo(%s→%s): expected %s, got %s", from, to, to, got)
			}
		}
	}
}

func TestTransitionTo_InvalidTransitionResetsToDeregistered(t *testing.T) {
	invalid := []struct {
		from, to StateType
	}{
		{Deregistered, Registered},
		{Deregistered, SecurityMode},
		{Deregistered, ContextSetup},
		{Authentication, Registered},
		{Authentication, ContextSetup},
		{SecurityMode, Registered},
		{SecurityMode, Authentication},
		{ContextSetup, Authentication},
		{ContextSetup, SecurityMode},
		{Registered, SecurityMode},
		{Registered, ContextSetup},
	}
	for _, tc := range invalid {
		ue := NewAmfUe()
		ue.Log = zap.NewNop()
		ue.state = tc.from

		ue.TransitionTo(tc.to)

		if got := ue.GetState(); got != Deregistered {
			t.Errorf("TransitionTo(%s→%s): expected Deregistered (fallback), got %s", tc.from, tc.to, got)
		}
	}
}

func TestTransitionTo_IdempotentSameState(t *testing.T) {
	for _, s := range []StateType{Deregistered, Authentication, SecurityMode, ContextSetup, Registered} {
		ue := NewAmfUe()
		ue.Log = zap.NewNop()
		ue.state = s

		ue.TransitionTo(s)

		if got := ue.GetState(); got != s {
			t.Errorf("TransitionTo(%s→%s): expected idempotent %s, got %s", s, s, s, got)
		}
	}
}

func TestTransitionTo_FullRegistrationCycle(t *testing.T) {
	ue := NewAmfUe()
	ue.Log = zap.NewNop()

	steps := []StateType{
		Authentication,
		SecurityMode,
		ContextSetup,
		Registered,
		Deregistered,
	}
	for i, step := range steps {
		ue.TransitionTo(step)

		if got := ue.GetState(); got != step {
			t.Fatalf("step %d: expected %s, got %s", i, step, got)
		}
	}
}

func TestTransitionTo_ConcurrentSafety(t *testing.T) {
	ue := NewAmfUe()
	ue.Log = zap.NewNop()

	var wg sync.WaitGroup

	// Hammer state transitions from multiple goroutines.
	// This is a race detector test — it should not panic or race.
	for i := 0; i < 100; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()

			ue.TransitionTo(Authentication)
		}()
		go func() {
			defer wg.Done()

			_ = ue.GetState()
		}()
	}

	wg.Wait()
}

func TestGetState_ReturnsCurrentState(t *testing.T) {
	ue := NewAmfUe()
	if got := ue.GetState(); got != Deregistered {
		t.Fatalf("new UE should be Deregistered, got %s", got)
	}

	ue.state = Registered
	if got := ue.GetState(); got != Registered {
		t.Fatalf("expected Registered, got %s", got)
	}
}

func TestNewAmfUe_DefaultState(t *testing.T) {
	ue := NewAmfUe()
	if ue.state != Deregistered {
		t.Fatalf("expected initial state Deregistered, got %s", ue.state)
	}
}
