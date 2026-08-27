// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

func TestUeConnLogConcurrentAccessNoRace(t *testing.T) {
	a := New(nil, nil, nil)
	target := &Radio{amf: a, name: "target", Log: logger.AmfLog}

	ueConn := &UeConn{AmfUeNgapID: 1, amf: a}
	ueConn.setLog(logger.AmfLog)

	ue := NewUeContext()
	ueConn.ue.Store(ue)

	a.mu.Lock()
	a.conns[int64(ueConn.AmfUeNgapID)] = ueConn
	a.mu.Unlock()

	var (
		wg      sync.WaitGroup
		commits atomic.Int64
	)

	for _, op := range []func(){
		func() {
			if a.CommitPathSwitch(ue, ueConn, target, models.RanUeNgapID(7), [32]uint8{}, 0) {
				commits.Add(1)
			}
		},
		func() { _ = ueConn.Log() },
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

	if commits.Load() == 0 {
		t.Fatal("CommitPathSwitch never committed; the logger write was not exercised")
	}
}
