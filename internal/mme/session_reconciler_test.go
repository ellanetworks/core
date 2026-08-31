// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "testing"

func TestSessionReconciler_StartStop(t *testing.T) {
	m := newTestMME(t)
	r := NewSessionReconciler(m, nil)

	r.Start()
	r.Start()
	r.Stop()
	r.Stop()

	r.Start()
	r.Stop()
}
