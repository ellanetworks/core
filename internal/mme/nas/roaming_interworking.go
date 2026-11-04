// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func registerArrivingVisitor(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, accepted func(context.Context)) {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("tracking area update rejected: failed to resolve the serving PLMN", zap.Error(err))
		rejectArrivingVisitor(ctx, m, ue, ueConn, &mme.Rejection{Cause: eps.EMMCauseNetworkFailure, Reason: "no serving PLMN"})

		return
	}

	m.RequestInterSystemUpdateLocation(ctx, ue, ueConn, plmn, func(ctx context.Context, err error) {
		if err != nil {
			r := mme.HPLMNRejection(err)
			logger.From(ctx, logger.MmeLog).Info("tracking area update rejected: update location with the home HSS failed",
				logger.Cause(r.Reason), zap.Error(err))
			rejectArrivingVisitor(ctx, m, ue, ueConn, r)

			return
		}

		if err := m.RegisterUE(ctx, ue); err != nil {
			var r *mme.Rejection
			if !errors.As(err, &r) {
				logger.From(ctx, logger.MmeLog).Error("tracking area update rejected: failed to register the visitor", zap.Error(err))
				r = &mme.Rejection{Cause: eps.EMMCauseNetworkFailure, Reason: err.Error()}
			}

			rejectArrivingVisitor(ctx, m, ue, ueConn, r)

			return
		}

		access, err := mme.ResolveAccess(ctx, m, ue.IMSI())
		if err != nil {
			logger.From(ctx, logger.MmeLog).Error("tracking area update rejected: failed to resolve the visitor's access", zap.Error(err))
			rejectArrivingVisitor(ctx, m, ue, ueConn, &mme.Rejection{Cause: eps.EMMCauseNetworkFailure, Reason: err.Error()})

			return
		}

		ue.SetAccess(access)

		if subscribed, err := mme.SubscribedUEAMBR(ctx, m, ue.IMSI()); err == nil {
			ue.SetSubscribedUEAMBR(subscribed)
		}

		logger.From(ctx, logger.MmeLog).Info("registered the visitor that moved from 5GS at its home HSS")

		accepted(ctx)
	})
}

func rejectArrivingVisitor(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, r *mme.Rejection) {
	logger.LogRegistrationAttempt(ctx, logger.MmeLog, metrics.RAT4G, "Tracking Area Update", logger.RegistrationRejected,
		logger.Cause(r.Cause.String()))
	ueConn.StopNASGuard(ctx)

	m.ReleaseAllSessions(ctx, ue, models.ReleaseFor(models.ReleaseRegistrationFailure))
	ue.TransitionTo(ctx, mme.EMMDeregistered)

	reject := &eps.TrackingAreaUpdateReject{Cause: r.Cause, ExtendedEMMCause: r.Extended}

	if r.BackOff > 0 {
		if timer, err := nas.GPRSTimer2FromDuration(r.BackOff); err == nil {
			reject.T3346 = &timer
		}
	}

	ueConn.SendDownlink(ctx, reject)

	m.ReleaseUEContext(ctx, ue, mme.CauseNASUnspecified)
}
