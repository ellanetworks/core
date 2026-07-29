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

// detachUeConnLocked clears the connection's back-pointer before clearing
// ue.active, both under amf.mu, which the export path does not hold. It can
// therefore see a connection still in ue.active that no longer points back at its
// UeContext.
func TestExportUeContext_DetachedConnDoesNotPanic(t *testing.T) {
	ue := NewUeContext()

	conn := &UeConn{Log: zap.NewNop()}
	ue.active.Store(conn)
	conn.ue = nil

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

// The export runs under ue.mu while a release runs under amf.mu, so anything the
// export reads from the connection must not be written under amf.mu alone.
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
