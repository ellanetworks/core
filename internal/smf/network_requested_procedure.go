// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/ellanetworks/core/internal/util/timer"
	"go.uber.org/zap"
)

// startRelease runs the network-requested PDU session release procedure
// (TS 24.501 §6.3.3): it frees the user-plane resources, sends the PDU Session
// Release Command with the given PTI and 5GSM cause, and either releases the
// session locally (UE in CM-IDLE, where N1 delivery is not possible) or arms
// T3592 to retransmit the command until the UE replies or the retransmission
// limit is reached. Caller must hold smContext.Mutex.
func (s *SMF) startRelease(ctx context.Context, smContext *SMContext, pti, cause uint8) error {
	s.releaseAllocatedAddresses(ctx, smContext)

	if err := s.releaseTunnel(ctx, smContext); err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Warn("release tunnel failed, continuing release",
			zap.Error(err), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
	}

	n1Msg, err := nas.BuildGSMPDUSessionReleaseCommand(smContext.PDUSessionID, pti, cause)
	if err != nil {
		return fmt.Errorf("build PDU Session Release Command (N1): %w", err)
	}

	n2Transfer, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
	if err != nil {
		return fmt.Errorf("build PDU Session Resource Release Command Transfer (N2): %w", err)
	}

	supi := smContext.Supi
	pduSessionID := smContext.PDUSessionID
	ref := smContext.CanonicalName()

	if err := s.amf.ReleaseSession(ctx, supi, pduSessionID, n1Msg, n2Transfer); err != nil {
		if errors.Is(err, ErrUENotReachable) {
			s.removeSessionUnlocked(ctx, ref)
			return nil
		}

		return fmt.Errorf("release session signaling: %w", err)
	}

	smContext.MarkPTIInUse(pti)
	s.armRetransmit(smContext, s.t3592,
		func() error { return s.amf.ReleaseSession(context.Background(), supi, pduSessionID, n1Msg, n2Transfer) },
		func(sc *SMContext) { s.removeSessionUnlocked(context.Background(), ref) })

	return nil
}

// armRetransmit starts a network-requested-procedure retransmission timer
// (TS 24.501 §6.3.2.5, §6.3.3): resend runs on each of the first
// maxSMProcedureRetransmissions expiries, and abort runs once that limit is
// exceeded. Both fire on the timer goroutine after the start path has released
// smContext.Mutex; each re-fetches the session and is a no-op if it is gone.
// Caller must hold smContext.Mutex.
func (s *SMF) armRetransmit(smContext *SMContext, d time.Duration, resend func() error, abort func(*SMContext)) {
	ref := smContext.CanonicalName()
	supi := smContext.Supi
	pduSessionID := smContext.PDUSessionID

	smContext.startProcedureTimer(timer.New(d, maxSMProcedureRetransmissions,
		func(expiry int32) {
			if s.GetSession(ref) == nil {
				return
			}

			if err := resend(); err != nil {
				logger.SmfLog.Warn("network-requested procedure retransmission failed",
					zap.Error(err), zap.Int32("attempt", expiry),
					logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))
			}
		},
		func() {
			sc := s.GetSession(ref)
			if sc == nil {
				return
			}

			sc.Mutex.Lock()
			defer sc.Mutex.Unlock()

			sc.stopProcedureTimer()
			abort(sc)
		}))
}
