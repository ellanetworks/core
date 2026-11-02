// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func (s *SMF) scheduleUnrecoverableReleaseLocked(ctx context.Context, sc *SMContext) {
	if sc.unrecoverableRelease || sc.releasing {
		return
	}

	sc.unrecoverableRelease = true

	logger.From(ctx, logger.SmfLog).Warn("UPF session state is unrecoverable; releasing the session",
		logger.SUPI(sc.Supi.String()), logger.SMContextRef(sc.Ref))

	go s.releaseUnrecoverable(context.WithoutCancel(ctx), sc)
}

func (s *SMF) releaseUnrecoverable(ctx context.Context, sc *SMContext) {
	const reason = models.ReleaseNetworkFailure

	sc.Mutex.Lock()

	if s.GetSession(sc.Ref) != sc || sc.releasing {
		sc.Mutex.Unlock()
		return
	}

	if sc.Access != Access4G {
		err := s.startRelease(ctx, sc, 0, fgs.GSMCauseReactivationRequested, models.ReleaseFor(reason))
		if err == nil {
			sc.Mutex.Unlock()
			return
		}

		logger.From(ctx, logger.SmfLog).Warn("could not signal the release of an unrecoverable session; removing it locally",
			logger.SUPI(sc.Supi.String()), logger.SMContextRef(sc.Ref), zap.Error(err))

		supi, pduSessionID, ref := sc.Supi, sc.PDUSessionID, sc.Ref
		s.teardownAndRemove(ctx, sc, models.ReleaseFor(reason))
		sc.Mutex.Unlock()

		if s.amf != nil {
			s.amf.SessionDropped(ctx, supi, pduSessionID, ref, nil)
		}

		return
	}

	imsi, ebi, ref := sc.Supi.IMSI(), sc.EBI, sc.Ref
	sc.Mutex.Unlock()

	if s.mme == nil {
		s.dropUnrecoverableEPS(ctx, sc, imsi, ebi, ref)
		return
	}

	err := s.mme.ReactivateEPSBearer(ctx, imsi, ebi, reason)
	if err == nil {
		return
	}

	if !errors.Is(err, ErrUENotReachable) {
		logger.From(ctx, logger.SmfLog).Warn("could not signal the release of an unrecoverable EPS session; removing it locally",
			logger.SUPIFromIMSI(imsi), logger.SMContextRef(ref), zap.Error(err))
	}

	s.dropUnrecoverableEPS(ctx, sc, imsi, ebi, ref)
}

func (s *SMF) dropUnrecoverableEPS(ctx context.Context, sc *SMContext, imsi string, ebi uint8, ref string) {
	sc.Mutex.Lock()

	if s.GetSession(ref) != sc {
		sc.Mutex.Unlock()
		return
	}

	s.teardownAndRemove(ctx, sc, models.ReleaseFor(models.ReleaseNetworkFailure))
	sc.Mutex.Unlock()

	if s.mme != nil {
		s.mme.SessionDropped(ctx, imsi, ebi, ref)
	}
}
