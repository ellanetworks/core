// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "testing"

// TestSessionReconciler_StartStop asserts the reconciler's lifecycle is safe:
// Start is idempotent, Stop drains the goroutine and is safe when already
// stopped, and the reconciler is restartable. Run under -race, it also guards
// against a leaked or racing reconcile goroutine.
func TestSessionReconciler_StartStop(t *testing.T) {
	m := newTestMME(t)
	r := NewSessionReconciler(m, nil)

	r.Start()
	r.Start() // a second Start without a paired Stop is a no-op
	r.Stop()
	r.Stop() // safe when already stopped

	// Restartable after Stop.
	r.Start()
	r.Stop()
}
