// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

func TestSnapshotConnectedAgreesWithConnection(t *testing.T) {
	a := New(nil, nil, nil)
	radio := &Radio{amf: a, name: "gnb-1", Log: logger.AmfLog}
	ue := NewUeContext()

	ueConn, err := a.NewUeConn(radio, models.RanUeNgapID(1))
	if err != nil {
		t.Fatalf("NewUeConn: %v", err)
	}

	ueConn.ue.Store(ue)

	var (
		wg   sync.WaitGroup
		stop = make(chan struct{})
	)

	wg.Add(1)

	go func() {
		defer wg.Done()
		defer close(stop)

		for range 20000 {
			a.mu.Lock()
			a.attachUeConnLocked(ue, ueConn)
			a.mu.Unlock()

			ueConn.Release()
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		for id := models.RanUeNgapID(1); ; id++ {
			select {
			case <-stop:
				return
			default:
			}

			a.UpdateUERanNgapID(ueConn, id)
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			select {
			case <-stop:
				return
			default:
			}

			snap := ue.Snapshot()
			if snap.Connected != (snap.Connection != nil) {
				t.Errorf("Snapshot torn: Connected = %v, Connection = %v", snap.Connected, snap.Connection)

				return
			}
		}
	}()

	wg.Wait()
}
