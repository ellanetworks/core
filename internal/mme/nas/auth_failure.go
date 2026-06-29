// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// handleAuthenticationFailure re-synchronises SQN and issues a new AUTHENTICATION
// REQUEST on a first synch failure (#21) with a valid AUTS; every other case — MAC
// failure (#20), a repeated synch failure, or an invalid AUTS — is rejected
// (TS 24.301).
func handleAuthenticationFailure(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	m.StopNASGuard(ue)

	resp, err := eps.ParseAuthenticationFailure(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Authentication Failure", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Authentication Failure",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)), zap.Uint8("emm-cause", resp.Cause))

	if resp.Cause == mme.EmmCauseSynchFailure && len(resp.AUTS) > 0 && !ue.ResyncTried() && ue.AuthVector != nil {
		ue.SetResyncTried(true)

		logger.MmeLog.Info("re-synchronising SQN, re-authenticating", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)))

		// The credential authority resyncs from AUTS and issues a fresh vector.
		if err := sendAuthRequest(m, ctx, ue, hex.EncodeToString(resp.AUTS), hex.EncodeToString(ue.AuthVector.RAND[:])); err != nil {
			logger.MmeLog.Warn("SQN re-synchronisation failed", zap.Error(err))
			rejectAuthentication(m, ctx, ue)
		}

		return
	}

	// MAC failure, a repeated synch failure, or a bad AUTS: the UE attaches with
	// its IMSI, so per TS 24.301 the network aborts with a reject.
	rejectAuthentication(m, ctx, ue)
}

// rejectAuthentication sends AUTHENTICATION REJECT (TS 24.301) and
// releases the UE's S1 context.
func rejectAuthentication(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	metrics.RegistrationAttempt(metrics.RAT4G, attachTypeName(ue), metrics.ResultReject)

	logger.MmeLog.Info("authentication rejected",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)), zap.String("imsi", ue.IMSI()))
	m.SendDownlinkMessage(ctx, ue, &eps.AuthenticationReject{})
	m.ReleaseUEContext(ctx, ue, mme.CauseNASUnspecified)
}
