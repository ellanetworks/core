// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"sync"
	"testing"
	"time"
)

func TestWaitTimeout(t *testing.T) {
	t.Run("empties in time", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			time.Sleep(50 * time.Millisecond)
			wg.Done()
		}()

		if !waitTimeout(&wg, 200*time.Millisecond) {
			t.Fatal("expected waitTimeout to return true when wg empties within d")
		}
	})

	t.Run("times out", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(1)
		// Never call wg.Done()

		if waitTimeout(&wg, 100*time.Millisecond) {
			t.Fatal("expected waitTimeout to return false when d elapses before wg empties")
		}

		wg.Done()
	})
}
