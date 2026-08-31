// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
)

func TestClaimICSExactlyOneWinner(t *testing.T) {
	conn := &amf.UeConn{}

	const n = 128

	var (
		wins int64
		wg   sync.WaitGroup
		gate = make(chan struct{})
	)

	for range n {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-gate

			if conn.ClaimICS() {
				atomic.AddInt64(&wins, 1)
			}
		}()
	}

	close(gate)
	wg.Wait()

	if wins != 1 {
		t.Fatalf("ClaimICS from ICSNotStarted: expected exactly 1 winner, got %d", wins)
	}

	if got := conn.ICS(); got != amf.ICSPending {
		t.Fatalf("ICS after claim = %v, want ICSPending", got)
	}
}

func TestICSConcurrentAccessNoRace(t *testing.T) {
	conn := &amf.UeConn{}

	var wg sync.WaitGroup

	for _, op := range []func(){
		func() { conn.ClaimICS() },
		func() { conn.MarkICSCompleted() },
		func() { conn.MarkICSPending() },
		func() { _ = conn.ICS() },
		func() { conn.ResetICS() },
	} {
		wg.Add(1)

		go func(f func()) {
			defer wg.Done()

			for range 1000 {
				f()
			}
		}(op)
	}

	wg.Wait()
}
