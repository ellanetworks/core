// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// releaseGuardTimeout bounds the wait for a UE Context Release Complete; on expiry the
// EMMState-keyed local cleanup runs, so a lost Complete cannot leak the UeConn + M-TMSI
// (S1AP §8.3 has no MME-side supervision timer, so this is a robustness guard).
const releaseGuardTimeout = 5 * time.Second

// causeSupersededConnection is the release cause for the old S1 connection when a UE
// re-establishes on a new one: a normal release of the superseded NAS signalling
// connection (TS 36.413 §8.3.3.1).
var causeSupersededConnection = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASNormalRelease}

// releaseSupersededConn releases the detached old connection toward the eNB and guards
// the Release Complete (TS 36.413 §8.3.3.1).
func (m *MME) releaseSupersededConn(ctx context.Context, c *UeConn) {
	SendUEContextRelease(ctx, m, c.conn, c.MMEUES1APID, c.ENBUES1APID, true, causeSupersededConnection)
	m.guardDetachedRelease(c)
}

// guardDetachedRelease supervises the Release Complete for a detached connection, so a
// lost Complete cannot leak its reserved MME-UE-S1AP-ID.
func (m *MME) guardDetachedRelease(c *UeConn) {
	c.releaseGuard.Arm(releaseGuardTimeout, 0, nil, func() {
		if m.ReleaseDetachedConn(c.conn, c.MMEUES1APID, c.ENBUES1APID) {
			logger.MmeLog.Info("reaped detached S1 connection after release timeout",
				zap.Uint32("mme-ue-id", uint32(c.MMEUES1APID)))
		}
	})
}

// AnswerDetachedRelease answers an eNB UE Context Release Request that names a detached
// connection the MME still holds — superseded, handover source, or bare — with a UE
// Context Release Command and guards the Complete, so the reserved MME-UE-S1AP-ID cannot
// leak (TS 36.413 §8.3.2.2). A release already in flight for the connection keeps its own
// command and guard. Reports whether conn held a matching detached connection.
func (m *MME) AnswerDetachedRelease(ctx context.Context, conn S1APWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID, cause s1ap.Cause) bool {
	m.mu.RLock()
	c, ok := m.conns[uint32(mmeUEID)]
	matched := ok && c.ue == nil && c.conn == conn && c.ENBUES1APID == enbUEID

	m.mu.RUnlock()

	if !matched {
		return false
	}

	if !c.releaseGuard.Active() {
		SendUEContextRelease(ctx, m, conn, mmeUEID, enbUEID, true, cause)
		m.guardDetachedRelease(c)
	}

	return true
}

// SendUEContextReleaseCommand builds a UE Context Release Command for this
// connection's S1AP identities and sends it to the eNB (TS 36.413 §8.3.1).
func (c *UeConn) SendUEContextReleaseCommand(ctx context.Context, cause s1ap.Cause) {
	if c == nil {
		return
	}

	cmd := &s1ap.UEContextReleaseCommand{
		UES1APIDs: s1ap.UES1APIDs{MMEUES1APID: c.MMEUES1APID, ENBUES1APID: c.ENBUES1APID, Pair: true},
		Cause:     s1ap.Ptr(cause),
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal UE Context Release Command", zap.Error(err))
		return
	}

	logger.From(ctx, c.Log).Info("UE Context Release Command")
	c.SendS1AP(ctx, S1APProcedureUEContextReleaseCommand, b)
}

func (m *MME) ReleaseUEContext(ctx context.Context, ue *UeContext, cause s1ap.Cause) {
	// The idempotency claim is atomic: a NAS guard timeout and an eNB-initiated
	// release request can race to release the same UE from different goroutines. A
	// Release Complete in the gap may already have freed the connection, which is
	// itself a completed release.
	if !m.claimRelease(ue) {
		return
	}

	conn := ue.Conn()
	if conn == nil {
		// No S1 connection to command; release the context locally.
		m.ReleaseUEContextLocally(ue, "release-no-connection")
		return
	}

	// Deactivate before the S1 UE Context Release Command so a concurrent downlink is
	// buffered for paging (TS 23.401 §5.3.5: Release Access Bearers precedes the release
	// command). Only a registered UE transitions to ECM-IDLE.
	if ue.EMMState() == EMMRegistered {
		m.DeactivateAllSessions(ctx, ue)
	}

	conn.SendUEContextReleaseCommand(ctx, cause)

	// Supervise the Release Complete: a lost Complete (or a command that could not be
	// marshalled/sent) fires the guard, which runs the EMMState-keyed local cleanup.
	conn.releaseGuard.Arm(releaseGuardTimeout, 0, nil, func() {
		m.ReleaseUEContextLocally(ue, "release-command-timeout")
	})
}

// ReleaseUEContextLocally releases a UE without sending a UE Context Release Command,
// for cases where the eNB has already released its side (e.g. INITIAL CONTEXT SETUP
// FAILURE, or an eNB/association loss). An incomplete registration is aborted; a
// registered UE drops to ECM-IDLE.
func (m *MME) ReleaseUEContextLocally(ue *UeContext, trigger string) {
	registered, imsi, mmeUEID := m.releaseContextLockedPart(ue)

	if !registered {
		m.ReleaseAllSessions(context.Background(), ue)
		logger.MmeLog.Info("aborted incomplete UE registration",
			zap.String("trigger", trigger), zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("imsi", imsi))

		return
	}

	m.DeactivateAllSessions(context.Background(), ue)
	m.StartMobileReachable(ue)
	logger.MmeLog.Info("UE moved to ECM-IDLE",
		zap.String("trigger", trigger), zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("imsi", imsi))
}
