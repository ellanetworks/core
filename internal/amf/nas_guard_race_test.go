// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/guard"
)

func TestNASGuardNameConcurrentAccessNoRace(t *testing.T) {
	conn := &UeConn{}
	cfg := guard.TimerValue{Enable: true, ExpireTime: time.Hour, MaxRetryTimes: 1}

	var wg sync.WaitGroup

	for _, op := range []func(){
		func() { conn.armNASGuardWith(cfg, "T3560 (Authentication Request)", func(int32) {}, func() {}) },
		func() { conn.StopNASGuard() },
		func() { _ = conn.nasGuardProcName() },
	} {
		wg.Add(1)

		go func(f func()) {
			defer wg.Done()

			for range 500 {
				f()
			}
		}(op)
	}

	wg.Wait()

	conn.StopNASGuard()
}
