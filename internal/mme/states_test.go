// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "testing"

func TestEMMTransition_AllowedTransitions(t *testing.T) {
	for from, targets := range validEMMTransitions {
		for _, to := range targets {
			var a emmStateAtomic

			a.store(from)
			a.transition(to)

			if got := a.load(); got != to {
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
		var a emmStateAtomic

		a.store(tc.from)
		a.transition(tc.to)

		if got := a.load(); got != EMMDeregistered {
			t.Errorf("transition(%s→%s): expected EMM-DEREGISTERED fallback, got %s", tc.from, tc.to, got)
		}
	}
}

func TestEMMTransition_Idempotent(t *testing.T) {
	for _, s := range []EMMState{EMMDeregistered, EMMRegistrationInitiated, EMMRegistered, EMMDeregistrationInitiated} {
		var a emmStateAtomic

		a.store(s)
		a.transition(s)

		if got := a.load(); got != s {
			t.Errorf("transition(%s→%s): expected idempotent, got %s", s, s, got)
		}
	}
}

func TestEMMTransition_FullAttachDetachCycle(t *testing.T) {
	var a emmStateAtomic

	steps := []EMMState{EMMRegistrationInitiated, EMMRegistered, EMMDeregistrationInitiated, EMMDeregistered}
	for i, step := range steps {
		a.transition(step)

		if got := a.load(); got != step {
			t.Fatalf("step %d: expected %s, got %s", i, step, got)
		}
	}
}
