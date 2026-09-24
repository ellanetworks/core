// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/nas/fgs"
)

func displaceConn(t *testing.T, ue *UeContext, displaced *UeConn) {
	t.Helper()

	replacement := &UeConn{amf: displaced.amf, AmfUeNgapID: 2}
	replacement.setConn(&downlinkOrderConn{wrote: make(chan struct{}, 1)})
	replacement.setRanUeNgapID(2)
	replacement.setRadio("", "test-gNB")

	displaced.amf.mu.Lock()
	displaced.amf.attachUeConnLocked(t.Context(), ue, replacement)
	displaced.amf.mu.Unlock()
}

func TestDisplacedConnectionAuthenticationGuardDoesNotAbortTheResumedUE(t *testing.T) {
	ue, _ := newDownlinkOrderUE(t)
	displaced := ue.Conn()

	var aborted atomic.Bool

	armNASGuard(t.Context(), displaced, displaced, guard.TimerValue{Enable: true, ExpireTime: 5 * time.Millisecond},
		"T3560 (Authentication Request)", []byte{0x7e, 0x00, byte(fgs.MsgAuthenticationRequest)}, uint8(fgs.SHTPlain),
		func(context.Context) { aborted.Store(true) })

	displaceConn(t, ue, displaced)

	deadline := time.Now().Add(time.Second)
	for displaced.nasGuard.Active() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	time.Sleep(20 * time.Millisecond)

	if aborted.Load() {
		t.Fatal("the authentication guard of the displaced connection aborted the procedure of a UE that resumed on a new connection")
	}
}

func TestNetworkInitiatedProcedureGuardFollowsTheUEToItsNewConnection(t *testing.T) {
	ue, _ := newDownlinkOrderUE(t)
	displaced := ue.Conn()

	displaced.armNASGuardWith(t.Context(), guard.TimerValue{Enable: true, ExpireTime: time.Hour, MaxRetryTimes: 4}, "T3522 (Deregistration Request)",
		func(context.Context, int32) {},
		func(context.Context) {})

	displaceConn(t, ue, displaced)

	if !displaced.nasGuard.Active() {
		t.Fatal("T3522 was cancelled when the UE moved to a new connection; TS 24.501 §5.5.2.3.5 still requires its retransmission and expiry handling")
	}
}
