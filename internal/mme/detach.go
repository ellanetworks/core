// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

func (m *MME) DetachUEAfterPathSwitchFailure(ctx context.Context, ue *UeContext) {
	if ue == nil {
		return
	}

	logger.From(ctx, logger.MmeLog).Warn("detaching UE: no EPS bearer could be switched during path switch",
		zap.String("imsi", ue.IMSI()))

	ue.TransitionTo(EMMDeregistered)
	m.ReleaseUEContext(ctx, ue, s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASDetach})
}

func (m *MME) DetachSubscriber(ctx context.Context, imsi string) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return
	}

	ueConn := ue.Conn()
	if ueConn == nil || !m.UeConnected(ue) {
		ue.TransitionTo(EMMDeregistered)
		logger.From(ctx, logger.MmeLog).Info("releasing idle UE on subscriber deletion", zap.String("imsi", imsi))
		m.ReleaseAllSessions(ctx, ue)
		m.RemoveUe(ue)

		return
	}

	if !ue.Secured() {
		logger.From(ctx, logger.MmeLog).Info("local detach of connected-but-unsecured UE on subscriber deletion",
			zap.String("imsi", imsi))
		m.ReleaseUEContextLocally(ue, "subscriber deleted")

		return
	}

	ue.TransitionTo(EMMDeregistrationInitiated)

	logger.From(ctx, ueConn.Log()).Info("network-initiated detach (subscriber deleted)",
		zap.String("imsi", imsi))

	plain, err := (&eps.DetachRequestNetwork{TypeOfDetach: eps.DetachTypeReattachNotRequired}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Detach Request", zap.Error(err))
		return
	}

	_ = ueConn.SendGuardedProtected(ctx, "Detach Request", plain, eps.SHTIntegrityProtectedCiphered)
}
