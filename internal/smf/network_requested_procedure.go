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
	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// startRelease runs the network-requested PDU session release procedure
// (TS 24.501 §6.3.3). User-plane resources are released up front so the UPF stops
// forwarding immediately (TS 23.502 §4.3.4.2 step 2); T3592 then retransmits the
// Release Command until the UE replies or the retransmission limit aborts, at which
// point the SM context is removed from the pool. Caller must hold smContext.Mutex.
func (s *SMF) startRelease(ctx context.Context, smContext *SMContext, pti uint8, cause fgs.GSMCause) error {
	if smContext.releasing {
		logger.WithTrace(ctx, logger.SmfLog).Info("a PDU session release is already outstanding, ignoring the colliding release trigger",
			zap.Uint8("pti", pti), logger.Cause(cause.String()),
			logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		return nil
	}

	s.releaseUserPlane(ctx, smContext)

	n1Msg, err := nas.BuildGSMPDUSessionReleaseCommand(fgs.PDUSessionID(smContext.PDUSessionID), naslib.ProcedureTransactionIdentity(pti), cause)
	if err != nil {
		return fmt.Errorf("build PDU Session Release Command (N1): %w", err)
	}

	smContext.recordN2Release(n2ReleaseSession)

	n2Transfer, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
	if err != nil {
		return fmt.Errorf("build PDU Session Resource Release Command Transfer (N2): %w", err)
	}

	supi := smContext.Supi
	pduSessionID := smContext.PDUSessionID

	if err := s.amf.ReleaseSession(ctx, supi, pduSessionID, n1Msg, n2Transfer); err != nil {
		if errors.Is(err, ErrUENotReachable) {
			// No UE to acknowledge and the user plane is already released, so remove
			// the SM context immediately.
			s.removeSessionUnlocked(ctx, smContext.Ref)
			return nil
		}

		return fmt.Errorf("release session signaling: %w", err)
	}

	smContext.releasing = true

	smContext.MarkPTIInUse(pti)
	s.armRetransmit(ctx, smContext, s.timerT3592(),
		func(ctx context.Context) error {
			return s.amf.ReleaseSession(ctx, supi, pduSessionID, n1Msg, n2Transfer)
		},
		func(ctx context.Context, sc *SMContext) { s.removeSessionUnlocked(ctx, sc.Ref) })

	return nil
}

// releaseUserPlane frees the session's user-plane resources — the UE IP address(es)
// and the UPF N4 tunnel — per TS 23.502 §4.3.4.2 step 2. Idempotent: safe to call up
// front on the release trigger and again on completion. Caller must hold
// smContext.Mutex.
func (s *SMF) releaseUserPlane(ctx context.Context, smContext *SMContext) {
	_ = s.releaseUserPlaneThenAddresses(ctx, smContext)
}

// teardownAndRemove releases the session's user-plane resources (idempotently) and
// removes it from the pool, for paths that reach the SM context without the
// network-requested procedure. Caller must hold smContext.Mutex.
func (s *SMF) teardownAndRemove(ctx context.Context, smContext *SMContext) {
	s.releaseUserPlane(ctx, smContext)
	s.removeSessionUnlocked(ctx, smContext.Ref)
}

// armRetransmit starts a network-requested-procedure retransmission timer
// (TS 24.501 §6.3.2.5, §6.3.3): resend runs on each of the first
// maxSMProcedureRetransmissions expiries, and abort runs once that limit is
// exceeded. Both fire on the timer goroutine and re-fetch the session, no-op if it
// is gone. Caller must hold smContext.Mutex.
func (s *SMF) armRetransmit(ctx context.Context, smContext *SMContext, timer smProcedureTimer, resend func(context.Context) error, abort func(context.Context, *SMContext)) {
	ref := smContext.Ref
	supi := smContext.Supi
	pduSessionID := smContext.PDUSessionID

	link := trace.SpanContextFromContext(ctx)

	smContext.procedureTimer.Arm(timer.d, maxSMProcedureRetransmissions,
		func(expiry int32) {
			if s.GetSession(ref) == nil {
				return
			}

			guardCtx, span := guardSpan(link, "smf/nas_guard_retransmit", timer.name, expiry)
			defer span.End()

			if err := resend(guardCtx); err != nil {
				logger.From(guardCtx, logger.SmfLog).Warn("network-requested procedure retransmission failed",
					zap.Error(err), zap.Int32("attempt", expiry),
					logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))
			}
		},
		func() {
			sc := s.GetSession(ref)
			if sc == nil {
				return
			}

			guardCtx, span := guardSpan(link, "smf/nas_guard_expire", timer.name, 0)
			defer span.End()

			sc.Mutex.Lock()
			defer sc.Mutex.Unlock()

			abort(guardCtx, sc)
		})
}

type smProcedureTimer struct {
	name string
	d    time.Duration
}

func (s *SMF) timerT3591() smProcedureTimer { return smProcedureTimer{name: "T3591", d: s.t3591} }

func (s *SMF) timerT3592() smProcedureTimer { return smProcedureTimer{name: "T3592", d: s.t3592} }
