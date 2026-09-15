// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
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
	m.ReleaseUEContext(ctx, ue, s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASDetach})
}

func (m *MME) DetachSubscriber(ctx context.Context, imsi string) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return
	}

	ueConn := ue.Conn()
	if ueConn == nil || !m.UeConnected(ue) {
		ue.TransitionTo(ctx, EMMDeregistered)
		logger.From(ctx, logger.MmeLog).Info("releasing idle UE on subscriber deletion", logger.SUPIFromIMSI(imsi))
		m.ReleaseAllSessions(ctx, ue)
		m.RemoveUe(ue)

		return
	}

	ctx = logger.Into(ctx, ueConn.Log())

	if !ue.Secured() {
		logger.From(ctx, logger.MmeLog).Info("local detach of connected-but-unsecured UE on subscriber deletion")
		m.ReleaseUEContextLocally(ctx, ue, "subscriber deleted")

		return
	}

	ue.TransitionTo(ctx, EMMDeregistrationInitiated)

	logger.From(ctx, ueConn.Log()).Info("UE deregistered", logger.RAT(metrics.RAT4G), zap.String("trigger", "network"))

	plain, err := (&eps.DetachRequestNetwork{TypeOfDetach: eps.DetachTypeReattachNotRequired}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Detach Request", zap.Error(err))
		return
	}

	_ = ueConn.SendGuardedProtected(ctx, "Detach Request", plain, eps.SHTIntegrityProtectedCiphered)
}
