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

// DeactivateBearer asks the UE to deactivate EPS bearer p with the given ESM
// cause and PTI. A disconnect or a non-default bearer releases only this PDN
// connection on timeout; the default bearer instead detaches the UE (TS 24.301 §6.4.4).
func (m *MME) DeactivateBearer(ctx context.Context, ue *UeContext, p *PdnConnection, esmCause, pti uint8, disconnecting bool) {
	naspdu, err := m.ProtectDownlinkMessage(ue, &eps.DeactivateEPSBearerContextRequest{
		EPSBearerIdentity:            p.Ebi,
		ProcedureTransactionIdentity: pti,
		ESMCause:                     esmCause,
	})
	if err != nil {
		logger.MmeLog.Error("failed to protect Deactivate EPS Bearer Context Request", zap.Error(err))
		return
	}

	ue.mu.Lock()
	p.Deactivating = true
	p.Disconnecting = disconnecting
	ue.mu.Unlock()

	if disconnecting || p.Ebi != ue.DefaultEBI {
		m.sendERABRelease(ctx, ue, p, naspdu)
		// The eNB releases the radio bearer, but the NAS DEACTIVATE EPS BEARER
		// CONTEXT REQUEST still needs an answer: guard it with T3495 so it is
		// retransmitted, and on exhaustion release just this PDN connection
		// locally rather than dropping the UE (TS 24.301 §6.4.4.5).
		m.ArmESMGuardAbortOnly(ue, p, "Deactivate EPS Bearer Context Request", naspdu, func() {
			m.ReleasePDN(ue, p)
		})

		return
	}

	m.SendDownlink(ctx, ue, naspdu)
	m.ArmESMGuard(ue, p, "Deactivate EPS Bearer Context Request", naspdu)
}

// DisconnectBearer tears down the UE's PDN connection p with a regular
// deactivation; the UE is not asked to re-establish it.
func (m *MME) DisconnectBearer(ctx context.Context, ue *UeContext, p *PdnConnection, esmCause, pti uint8) {
	m.DeactivateBearer(ctx, ue, p, esmCause, pti, true)
}

// sendERABRelease releases a UE's E-RAB at the eNB while the UE stays connected,
// carrying the DEACTIVATE EPS BEARER CONTEXT REQUEST in the NAS-PDU so the eNB
// both releases the radio bearer and delivers the NAS (TS 36.413 §8.2.3).
func (m *MME) sendERABRelease(ctx context.Context, ue *UeContext, p *PdnConnection, naspdu []byte) {
	cmd := &s1ap.ERABReleaseCommand{
		MMEUES1APID: ue.S1.MMEUES1APID,
		ENBUES1APID: ue.S1.ENBUES1APID,
		ERABToBeReleased: []s1ap.ERABItem{{
			ERABID: s1ap.ERABID(p.Ebi),
			Cause:  CauseNASNormalRelease,
		}},
		NASPDU: s1ap.NASPDU(naspdu),
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal E-RAB Release Command", zap.Error(err))
		return
	}

	m.SendS1AP(ctx, ue, S1APProcedureERABReleaseCommand, b)
}
