// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/guard"
)

func TestDisplacedConnectionNASGuardDoesNotAbortTheResumedUE(t *testing.T) {
	ue, _ := newDownlinkOrderUE(t)
	displaced := ue.Conn()

	var aborted atomic.Bool

	displaced.armNASGuardWith(t.Context(), guard.TimerValue{Enable: true, ExpireTime: 10 * time.Millisecond}, "T3560 (test)",
		func(context.Context, int32) {},
		func(context.Context) { aborted.Store(true) })

	replacement := &UeConn{amf: displaced.amf, AmfUeNgapID: 2}
	replacement.setConn(&downlinkOrderConn{wrote: make(chan struct{}, 1)})
	replacement.setRanUeNgapID(2)
	replacement.setRadio("", "test-gNB")

	displaced.amf.mu.Lock()
	displaced.amf.attachUeConnLocked(t.Context(), ue, replacement)
	displaced.amf.mu.Unlock()

	time.Sleep(50 * time.Millisecond)

	if aborted.Load() {
		t.Fatal("the NAS guard of the displaced connection aborted the procedure of a UE that resumed on a new connection")
	}
}
