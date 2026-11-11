// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

func (m *MME) DetachUEAfterPathSwitchFailure(ctx context.Context, ue *UeContext) {
	if ue == nil {
		return
	}

	logger.From(ctx, logger.MmeLog).Warn("detaching UE: no EPS bearer could be switched during path switch",
		logger.SUPI(ue.Supi().String()))

	ue.TransitionTo(ctx, EMMDeregistered)
	m.ReleaseUEContext(ctx, ue, ue.Conn(), s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASDetach})
}

func (m *MME) DetachSubscriber(ctx context.Context, imsi string) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return
	}

	m.detachUE(ctx, ue, eps.DetachTypeReattachNotRequired, nil, models.ReleaseFor(models.ReleaseSubscriptionWithdrawn), "subscriber deletion")
}

func (m *MME) detachUE(ctx context.Context, ue *UeContext, detachType eps.DetachTypeNetwork, cause *eps.EMMCause, release models.SessionReleaseRequest, reason string) {
	ueConn := ue.Conn()
	if ueConn == nil || !m.UeConnected(ue) {
		ue.TransitionTo(ctx, EMMDeregistered)
		logger.From(ctx, logger.MmeLog).Info("releasing idle UE on "+reason, logger.SUPI(ue.Supi().String()))
		m.ReleaseAllSessions(ctx, ue, release)
		m.RemoveUe(ctx, ue)

		return
	}

	ctx = logger.Into(ctx, ueConn.LogFields()...)

	if !ue.Secured() {
		logger.From(ctx, logger.MmeLog).Info("local detach of connected-but-unsecured UE on " + reason)
		m.ReleaseUEContextLocally(ctx, ue, reason, release)

		return
	}

	m.sendNetworkDetach(ctx, ue, ueConn, detachType, cause, release)
}

func (m *MME) sendNetworkDetach(ctx context.Context, ue *UeContext, ueConn *UeConn, detachType eps.DetachTypeNetwork, cause *eps.EMMCause, release models.SessionReleaseRequest) {
	ue.TransitionTo(ctx, EMMDeregistrationInitiated)
	ue.startNetworkDetach(release)

	ueConn.Log(ctx).Info("UE deregistered", logger.RAT(metrics.RAT4G), zap.String("trigger", "network"))

	plain, err := (&eps.DetachRequestNetwork{TypeOfDetach: detachType, Cause: cause}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Detach Request", zap.Error(err))
		return
	}

	_ = ueConn.SendGuardedProtected(ctx, "Detach Request", plain, eps.SHTIntegrityProtectedCiphered)
}
