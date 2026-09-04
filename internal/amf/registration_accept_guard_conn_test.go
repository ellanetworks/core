// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/guard"
	"go.uber.org/zap"
)

func registrationAcceptPlain() []byte {
	return []byte{0x7e, 0x00, 0x42, 0x01, 0x01}
}

func TestRegistrationAcceptGuardRetransmitsOnItsOwnConnection(t *testing.T) {
	ue, sender := newDownlinkOrderUE(t)

	amfInstance := ue.Conn().amf
	amfInstance.NASGuardCfg = guard.TimerValue{Enable: true, ExpireTime: time.Millisecond, MaxRetryTimes: 8}

	ArmRegistrationAcceptGuard(amfInstance, ue, registrationAcceptPlain())

	sender.awaitWrites(t, 1)

	ue.Conn().StopNASGuard()
}

func TestRegistrationAcceptGuardDoesNotRetransmitOnAReplacedConnection(t *testing.T) {
	ue, sender := newDownlinkOrderUE(t)

	original := ue.Conn()
	amfInstance := original.amf
	amfInstance.NASGuardCfg = guard.TimerValue{Enable: true, ExpireTime: 150 * time.Millisecond, MaxRetryTimes: 8}

	ArmRegistrationAcceptGuard(amfInstance, ue, registrationAcceptPlain())

	replacementSender := &downlinkOrderConn{wrote: make(chan struct{}, 1)}

	replacement := &UeConn{
		conn:        replacementSender,
		amf:         amfInstance,
		RanUeNgapID: 2,
		AmfUeNgapID: 2,
	}
	replacement.setRadio("", "test-gNB")
	replacement.setLog(zap.NewNop())

	amfInstance.AttachUeConn(ue, replacement)

	time.Sleep(600 * time.Millisecond)

	replacementSender.mu.Lock()
	onReplacement := len(replacementSender.seqs)
	replacementSender.mu.Unlock()

	if onReplacement != 0 {
		t.Errorf("registration accept retransmissions on the replacement connection = %d, want 0: the accept belongs to the connection it was sent on (TS 24.501 §5.5.1.2.8 a)", onReplacement)
	}

	sender.mu.Lock()
	onOriginal := len(sender.seqs)
	sender.mu.Unlock()

	if onOriginal != 0 {
		t.Errorf("registration accept retransmissions on the released connection = %d, want 0", onOriginal)
	}

	if original.NASGuardActive() {
		t.Error("T3550 is still armed on the replaced connection; it must be stopped so the exhausted callback cannot clear the new connection's registration data")
	}
}
