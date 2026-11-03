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
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func authenticateOrReject(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn) {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		failAuthentication(ctx, m, ue, ueConn, err)

		return
	}

	startAuthentication(ctx, m, ue, ueConn, plmn)
}

func startAuthentication(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, servingPLMN models.PlmnID) {
	if err := m.AdmitVisitor(ctx, ue, servingPLMN); err != nil {
		failAuthentication(ctx, m, ue, ueConn, err)

		return
	}

	// resyncTried scopes to one authentication exchange's consecutive synch
	// failures, so a fresh procedure starts with a full budget (TS 24.301 §5.4.2.7).
	ueConn.SetResyncTried(false)

	// A new authentication carries an eKSI distinct from the stored one, so the UE keeps
	// its current context usable until the new one is taken into use (TS 24.301 §5.4.2.4).
	ueConn.AuthEksi = nas.KeySetIdentifier{Value: mme.SelectEksi(citedEksi(ueConn), ue.StoredEksi())}

	sendAuthRequest(ctx, m, ue, ueConn, servingPLMN, nil, func(ctx context.Context, err error) {
		failAuthentication(ctx, m, ue, ueConn, err)
	})
}

func citedEksi(ueConn *mme.UeConn) uint8 {
	if ueConn == nil || ueConn.AttachRequestPlain == nil {
		return nas.NoKeyAvailable
	}

	req, err := eps.ParseAttachRequest(ueConn.AttachRequestPlain)
	if err != nil {
		return nas.NoKeyAvailable
	}

	return req.NASKeySetIdentifier.Value
}

func failAuthentication(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, err error) {
	if r, ok := roamingRejection(ue, err); ok {
		logger.From(ctx, logger.MmeLog).Info("attach rejected: roaming", logger.Cause(r.Reason), zap.Error(err))
		sendAttachRejection(ctx, m, ue, ueConn, r, nil)

		return
	}

	cause, permanent := attachRejectCauseForAuthFailure(err)
	if !permanent {
		logger.LogRegistrationAttempt(ctx, logger.MmeLog, metrics.RAT4G, attachTypeName(ue), logger.RegistrationRejected,
			logger.Cause("transient authentication failure; UE retries when T3411 expires"), zap.Error(err))
		ueConn.StopNASGuard(ctx)

		m.ReleaseUEContext(ctx, ue, mme.CauseNASUnspecified)

		return
	}

	logger.From(ctx, logger.MmeLog).Info("attach rejected: cannot authenticate subscriber", zap.Error(err))
	rejectAttach(ctx, m, ue, ueConn, cause)
}

func roamingRejection(ue *mme.UeContext, err error) (*mme.Rejection, bool) {
	var r *mme.Rejection
	if errors.As(err, &r) {
		return r, true
	}

	if !ue.IsVisitor() {
		return nil, false
	}

	return mme.HomeNetworkRejection(err), true
}

func attachRejectCauseForAuthFailure(err error) (eps.EMMCause, bool) {
	if errors.Is(err, udm.ErrSubscriberUnknown) {
		return eps.EMMCauseIllegalUE, true
	}

	return 0, false
}

// sendAuthRequest sends an AUTHENTICATION REQUEST; a set resync pair drives an
// AUTS re-synchronisation.
func sendAuthRequest(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, servingPLMN models.PlmnID, resync *mme.Resync, fail func(context.Context, error)) {
	m.RequestAuthenticationVector(ctx, ue, ueConn, servingPLMN, resync, func(ctx context.Context, vec *udm.EPSAV, err error) {
		if err != nil {
			fail(ctx, err)
			return
		}

		c := ueConn
		c.AuthVector = vec

		logger.From(ctx, logger.MmeLog).Info("Authentication Request")
		c.SendGuardedMessage(ctx, "Authentication Request", &eps.AuthenticationRequest{NASKeySetIdentifier: c.AuthEksi, RAND: vec.RAND, AUTN: vec.AUTN})
	})
}
