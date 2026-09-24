// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"
)

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

func TestSessionReconcilerReconcilesOnWakeup(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	qos, err := ResolveQoS(context.Background(), m, ue.imsiOrEmpty())
	if err != nil {
		t.Fatal(err)
	}

	p := testPDN(ue)
	p.DnConfig = qos.DnFingerprint()

	defer ue.Conn().StopNASGuard(t.Context())

	wakeup := make(chan struct{})

	r := NewSessionReconciler(m, wakeup)
	r.backstop = time.Hour

	r.Start()
	defer r.Stop()

	wakeup <- struct{}{}

	ue.mu.Lock()
	p.DnConfig = "stale|config|0.0.0.0|0"
	ue.mu.Unlock()

	wakeup <- struct{}{}

	eventually(t, 2*time.Second, func() bool { return cc.count() == 1 })
}
