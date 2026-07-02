// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

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
			ue := NewUeContext()
			ue.Log = zap.NewNop()
			ue.state = from

			ue.TransitionTo(to)

			if got := ue.State(); got != to {
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
		{Deregistered, DeregistrationInitiated},
		{RegistrationInitiated, DeregistrationInitiated},
		{DeregistrationInitiated, RegistrationInitiated},
		{DeregistrationInitiated, Registered},
	}
	for _, tc := range invalid {
		ue := NewUeContext()
		ue.Log = zap.NewNop()
		ue.state = tc.from

		ue.TransitionTo(tc.to)

		if got := ue.State(); got != Deregistered {
			t.Errorf("TransitionTo(%s→%s): expected Deregistered (fallback), got %s", tc.from, tc.to, got)
		}
	}
}

func TestTransitionTo_IdempotentSameState(t *testing.T) {
	for _, s := range []StateType{Deregistered, RegistrationInitiated, Registered, DeregistrationInitiated} {
		ue := NewUeContext()
		ue.Log = zap.NewNop()
		ue.state = s

		ue.TransitionTo(s)

		if got := ue.State(); got != s {
			t.Errorf("TransitionTo(%s→%s): expected idempotent %s, got %s", s, s, s, got)
		}
	}
}

func TestTransitionTo_FullRegistrationCycle(t *testing.T) {
	ue := NewUeContext()
	ue.Log = zap.NewNop()

	steps := []StateType{
		RegistrationInitiated,
		Registered,
		DeregistrationInitiated,
		Deregistered,
	}
	for i, step := range steps {
		ue.TransitionTo(step)

		if got := ue.State(); got != step {
			t.Fatalf("step %d: expected %s, got %s", i, step, got)
		}
	}
}

// TestRegStep_TracksRegistrationSubPhase checks that the registration sub-phase
// follows the mobility-management state: it starts at the authentication exchange
// on entering RegistrationInitiated, advances within it, and clears on leaving.
func TestRegStep_TracksRegistrationSubPhase(t *testing.T) {
	ue := NewUeContext()
	ue.Log = zap.NewNop()

	if got := ue.RegStep(); got != RegStepNone {
		t.Fatalf("a fresh UE must carry no registration sub-phase, got %d", got)
	}

	ue.TransitionTo(RegistrationInitiated)

	if got := ue.RegStep(); got != RegStepAuthenticating {
		t.Fatalf("entering RegistrationInitiated must start at the authentication exchange, got %d", got)
	}

	ue.AdvanceRegStep(RegStepSecurityMode)

	if got := ue.RegStep(); got != RegStepSecurityMode {
		t.Fatalf("AdvanceRegStep must move the sub-phase, got %d", got)
	}

	ue.TransitionTo(Registered)

	if got := ue.RegStep(); got != RegStepNone {
		t.Fatalf("leaving RegistrationInitiated must clear the sub-phase, got %d", got)
	}

	// AdvanceRegStep is a no-op outside RegistrationInitiated.
	ue.AdvanceRegStep(RegStepContextSetup)

	if got := ue.RegStep(); got != RegStepNone {
		t.Fatalf("AdvanceRegStep outside RegistrationInitiated must be a no-op, got %d", got)
	}
}

func TestTransitionTo_ConcurrentSafety(t *testing.T) {
	ue := NewUeContext()
	ue.Log = zap.NewNop()

	var wg sync.WaitGroup

	// Hammer state transitions from multiple goroutines.
	// This is a race detector test — it should not panic or race.
	for i := 0; i < 100; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()

			ue.TransitionTo(RegistrationInitiated)
		}()
		go func() {
			defer wg.Done()

			_ = ue.State()
		}()
	}

	wg.Wait()
}

func TestGetState_ReturnsCurrentState(t *testing.T) {
	ue := NewUeContext()
	if got := ue.State(); got != Deregistered {
		t.Fatalf("new UE should be Deregistered, got %s", got)
	}

	ue.state = Registered
	if got := ue.State(); got != Registered {
		t.Fatalf("expected Registered, got %s", got)
	}
}

func TestNewUeContext_DefaultState(t *testing.T) {
	ue := NewUeContext()
	if ue.state != Deregistered {
		t.Fatalf("expected initial state Deregistered, got %s", ue.state)
	}
}
