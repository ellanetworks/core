// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// sendGuardedMessage serializes a NAS message, sends it to the UE, and arms the
// NAS common-procedure guard timer so the message is retransmitted if the UE
// does not respond (TS 24.301: T3450/T3460/T3470).
func (m *MME) sendGuardedMessage(ctx context.Context, ue *UeContext, name string, msg nasMessage) {
	b, err := msg.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	m.sendGuardedDownlink(ctx, ue, name, b)
}

// sendGuardedDownlink sends already-serialized NAS bytes and arms the guard.
func (m *MME) sendGuardedDownlink(ctx context.Context, ue *UeContext, name string, nas []byte) {
	m.armNASGuard(ue, name, nas)
	m.sendDownlink(ctx, ue, nas)
}

// armNASGuard records the outstanding downlink message and starts its guard
// timer, cancelling any previous one. The retransmitted bytes are kept verbatim
// so the NAS sequence number is preserved across retransmissions (TS 24.301).
// Exhausting the retransmissions releases the UE (the UE stopped answering a
// procedure the network requires).
func (m *MME) armNASGuard(ue *UeContext, name string, nas []byte) {
	m.armNASGuardMode(ue, name, nas, nil)
}

// armNASGuardAbortOnly arms the guard for a non-critical procedure: exhausting
// the retransmissions runs onAbort and leaves the UE connected rather than
// releasing it (TS 24.301 §6.4.2.5, §6.4.4.5). onAbort finalizes the procedure
// locally (e.g. clearing a pending modification or releasing a single PDN).
func (m *MME) armNASGuardAbortOnly(ue *UeContext, name string, nas []byte, onAbort func()) {
	m.armNASGuardMode(ue, name, nas, onAbort)
}

func (m *MME) armNASGuardMode(ue *UeContext, name string, nas []byte, onAbort func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopNASGuardLocked(ue)
	gen := ue.nasGuardGen

	ue.nasGuardName = name
	ue.nasGuardPDU = nas
	ue.nasGuardTries = 0
	ue.nasGuardOnAbort = onAbort

	ue.nasGuardTimer = time.AfterFunc(m.nasGuardTimeout, func() {
		m.onNASGuardExpiry(ue, gen)
	})
}

// stopNASGuard cancels the guard once the UE's response arrives.
func (m *MME) stopNASGuard(ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopNASGuardLocked(ue)
}

// stopNASGuardLocked cancels the guard and invalidates any in-flight callback.
// The caller holds m.mu.
func (m *MME) stopNASGuardLocked(ue *UeContext) {
	ue.nasGuardGen++

	if ue.nasGuardTimer != nil {
		ue.nasGuardTimer.Stop()
		ue.nasGuardTimer = nil
	}

	ue.nasGuardPDU = nil
	ue.nasGuardOnAbort = nil
}

// onNASGuardExpiry retransmits the outstanding downlink message, or releases the
// UE once the retransmission limit is reached (TS 24.301): the UE has
// stopped answering, so the procedure is aborted and the UE released.
func (m *MME) onNASGuardExpiry(ue *UeContext, gen uint64) {
	m.mu.Lock()

	if ue.nasGuardGen != gen {
		m.mu.Unlock()
		return
	}

	ue.nasGuardTries++
	name := ue.nasGuardName

	if ue.nasGuardTries > m.nasGuardMaxRetransmit {
		ue.nasGuardTimer = nil
		ue.nasGuardPDU = nil
		ue.nasGuardGen++
		mmeUEID := ue.MMEUES1APID
		onAbort := ue.nasGuardOnAbort
		ue.nasGuardOnAbort = nil

		m.mu.Unlock()

		if onAbort != nil {
			logger.MmeLog.Info("NAS procedure timed out, aborting (UE stays connected)",
				zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("procedure", name))

			onAbort()

			return
		}

		logger.MmeLog.Info("NAS procedure timed out, releasing UE",
			zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("procedure", name))
		// The guard fires from a timer outside any request; start a fresh root.
		m.releaseUEContext(context.Background(), ue, causeNASUnspecified)

		return
	}

	pdu := ue.nasGuardPDU
	tries := ue.nasGuardTries

	ue.nasGuardTimer = time.AfterFunc(m.nasGuardTimeout, func() {
		m.onNASGuardExpiry(ue, gen)
	})

	m.mu.Unlock()

	logger.MmeLog.Info("retransmitting NAS message",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("procedure", name), zap.Int("attempt", tries))
	// Retransmission is timer-driven, outside the original request; start a fresh root.
	m.sendDownlink(context.Background(), ue, pdu)
}
