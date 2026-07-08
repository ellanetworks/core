// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "testing"

func TestEMMTransition_AllowedTransitions(t *testing.T) {
	for from, targets := range validEMMTransitions {
		for _, to := range targets {
			ue := &UeContext{}

			ue.ForceStateForTest(from)
			ue.TransitionTo(to)

			if got := ue.EMMState(); got != to {
				t.Errorf("transition(%s→%s): expected %s, got %s", from, to, to, got)
			}
		}
	}
}

func TestEMMTransition_InvalidResetsToDeregistered(t *testing.T) {
	invalid := []struct{ from, to EMMState }{
		{EMMDeregistered, EMMRegistered},
		{EMMDeregistered, EMMDeregistrationInitiated},
		{EMMRegistrationInitiated, EMMDeregistrationInitiated},
		{EMMDeregistrationInitiated, EMMRegistered},
	}
	for _, tc := range invalid {
		ue := &UeContext{}

		ue.ForceStateForTest(tc.from)
		ue.TransitionTo(tc.to)

		if got := ue.EMMState(); got != EMMDeregistered {
			t.Errorf("transition(%s→%s): expected EMM-DEREGISTERED fallback, got %s", tc.from, tc.to, got)
		}
	}
}

func TestEMMTransition_Idempotent(t *testing.T) {
	for _, s := range []EMMState{EMMDeregistered, EMMRegistrationInitiated, EMMRegistered, EMMDeregistrationInitiated} {
		ue := &UeContext{}

		ue.ForceStateForTest(s)
		ue.TransitionTo(s)

		if got := ue.EMMState(); got != s {
			t.Errorf("transition(%s→%s): expected idempotent, got %s", s, s, got)
		}
	}
}

func TestEMMTransition_FullAttachDetachCycle(t *testing.T) {
	ue := &UeContext{}

	steps := []EMMState{EMMRegistrationInitiated, EMMRegistered, EMMDeregistrationInitiated, EMMDeregistered}
	for i, step := range steps {
		ue.TransitionTo(step)

		if got := ue.EMMState(); got != step {
			t.Fatalf("step %d: expected %s, got %s", i, step, got)
		}
	}
}
