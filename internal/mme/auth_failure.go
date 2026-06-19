// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// onAuthenticationFailure handles an AUTHENTICATION FAILURE (TS 24.301).
// A first synch failure (#21) with a valid AUTS triggers SQN
// re-synchronisation and a new AUTHENTICATION REQUEST; every other case — MAC
// failure (#20), a repeated synch failure, or an invalid AUTS — is rejected.
func (m *MME) onAuthenticationFailure(ctx context.Context, ue *UeContext, plain []byte) {
	m.stopNASGuard(ue)

	resp, err := eps.ParseAuthenticationFailure(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Authentication Failure", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Authentication Failure",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.Uint8("emm-cause", resp.Cause))

	if resp.Cause == emmCauseSynchFailure && len(resp.AUTS) > 0 && !ue.resyncTried && ue.authVector != nil {
		ue.resyncTried = true

		logger.MmeLog.Info("re-synchronising SQN, re-authenticating", zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)))

		// The credential authority resyncs from AUTS and issues a fresh vector.
		if err := m.sendAuthRequest(ctx, ue, hex.EncodeToString(resp.AUTS), hex.EncodeToString(ue.authVector.RAND[:])); err != nil {
			logger.MmeLog.Warn("SQN re-synchronisation failed", zap.Error(err))
			m.rejectAuthentication(ctx, ue)
		}

		return
	}

	// MAC failure, a repeated synch failure, or a bad AUTS: the UE attaches with
	// its IMSI, so per TS 24.301 the network aborts with a reject.
	m.rejectAuthentication(ctx, ue)
}

// rejectAuthentication sends AUTHENTICATION REJECT (TS 24.301) and
// releases the UE's S1 context.
func (m *MME) rejectAuthentication(ctx context.Context, ue *UeContext) {
	logger.MmeLog.Info("authentication rejected",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi))
	m.sendDownlinkMessage(ctx, ue, &eps.AuthenticationReject{})
	m.releaseUEContext(ctx, ue, causeNASUnspecified)
}
