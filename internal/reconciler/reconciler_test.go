// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"context"
	"testing"
	"time"
)

func TestReconcilerSweepsOnWakeup(t *testing.T) {
	first, second := make(chan struct{}, 4), make(chan struct{}, 4)
	wakeup := make(chan struct{})

	r := New(wakeup,
		func(context.Context) { first <- struct{}{} },
		func(context.Context) { second <- struct{}{} },
	)
	r.backstop = time.Hour

	r.Start()
	defer r.Stop()

	for _, sweep := range []chan struct{}{first, second} {
		select {
		case <-sweep:
		case <-time.After(2 * time.Second):
			t.Fatal("the initial reconcile did not run every sweep")
		}
	}

	wakeup <- struct{}{}

	for _, sweep := range []chan struct{}{first, second} {
		select {
		case <-sweep:
		case <-time.After(2 * time.Second):
			t.Fatal("a wakeup did not run every sweep")
		}
	}
}
