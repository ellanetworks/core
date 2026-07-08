// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/guard"
)

// TestNASGuardNameConcurrentAccessNoRace races the NAS-guard arm/stop writers (on
// the NAS-dispatch and network-initiated paths) against the status-export reader.
// Its value is under `-race`: on a plain string field this fails; through the
// atomic pointer it is clean.
func TestNASGuardNameConcurrentAccessNoRace(t *testing.T) {
	conn := &UeConn{}
	cfg := guard.TimerValue{Enable: true, ExpireTime: time.Hour, MaxRetryTimes: 1}

	var wg sync.WaitGroup

	for _, op := range []func(){
		func() { conn.armNASGuardWith(cfg, "T3560 (Authentication Request)", func(int32) {}, func() {}) },
		func() { conn.StopNASGuard() },
		func() { _ = conn.nasGuardProcName() }, // the status-export read
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
