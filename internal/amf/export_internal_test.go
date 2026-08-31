// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/sctp"
	"go.uber.org/zap"
)

type nopNGAPSender struct{}

func (nopNGAPSender) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) { return len(b), nil }

func TestExportUeContext_DetachedConnDoesNotPanic(t *testing.T) {
	ue := NewUeContext()

	conn := &UeConn{}
	conn.setLog(zap.NewNop())
	ue.active.Store(conn)
	conn.ue.Store(nil)

	amf := New(nil, nil, nil)

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("export panicked on a detaching connection: %v", r)
			}
		}()

		_ = amf.exportUeContext(nil, ue)
	}()

	if !ue.mu.TryLock() {
		t.Fatal("ue.mu was left held by the export")
	}

	ue.mu.Unlock()
}

func TestExportUeContext_ConcurrentWithReleaseNasConnection(t *testing.T) {
	for i := 0; i < 200; i++ {
		amf := New(nil, nil, nil)
		ue := NewUeContext()

		conn := NewUeConnForTest(&Radio{Conn: nopNGAPSender{}, Log: zap.NewNop()}, 1, 10, zap.NewNop())
		amf.AttachUeConn(ue, conn)

		var wg sync.WaitGroup

		wg.Add(2)

		go func() {
			defer wg.Done()

			amf.ReleaseNasConnection(ue, nil)
		}()

		go func() {
			defer wg.Done()

			_ = amf.exportUeContext(nil, ue)
		}()

		wg.Wait()
	}
}
