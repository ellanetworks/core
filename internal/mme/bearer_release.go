// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// DeactivateBearer asks the UE to deactivate EPS bearer p with the given ESM
// cause and PTI. A disconnect or a non-default bearer releases only this PDN
// connection on timeout; the default bearer instead detaches the UE (TS 24.301 §6.4.4).
func (m *MME) DeactivateBearer(ctx context.Context, ue *UeContext, p *PdnConnection, esmCause eps.ESMCause, pti uint8, disconnecting bool) {
	// Snapshotted, not re-read: this runs off the dispatch goroutine and the
	// unlocked calls below let a concurrent release nil ue.Conn().
	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.MmeLog).Warn("deactivate EPS bearer: UE has no S1 connection",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.Ebi))

		return
	}

	ue.mu.Lock()

	if p.Deactivating {
		ue.mu.Unlock()

		return
	}

	p.Deactivating = true
	p.Disconnecting = disconnecting
	ue.mu.Unlock()

	plain, err := (&eps.DeactivateEPSBearerContextRequest{
		EPSBearerIdentity: eps.EPSBearerIdentity(p.Ebi),
		PTI:               nas.ProcedureTransactionIdentity(pti),
		Cause:             esmCause,
	}).MarshalBinary()
	if err != nil {
		ue.mu.Lock()
		p.Deactivating = false
		ue.mu.Unlock()

		logger.From(ctx, logger.MmeLog).Error("failed to build Deactivate EPS Bearer Context Request",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.Ebi), zap.Error(err))

		return
	}

	releaseOnly := ue.BearerReleaseOnly(p)

	write := func(wire []byte) error {
		ueConn.SendDownlinkNASTransport(ctx, wire)

		return nil
	}

	if releaseOnly {
		write = func(wire []byte) error {
			m.sendERABRelease(ctx, ueConn, p, wire)

			return nil
		}
	}

	if err := ueConn.SendProtected(plain, eps.SHTIntegrityProtectedCiphered, write); err != nil {
		ue.mu.Lock()
		p.Deactivating = false
		ue.mu.Unlock()

		ReportProtectFailure(ctx, ueConn, "Deactivate EPS Bearer Context Request", err)

		return
	}

	// TS 23.401 §5.4.4 (symmetric with the 5G TS 23.502 §4.3.4.2 step 2): release the
	// UPF user plane at the start of the deactivation so it drops any remaining packets
	// and frees the tunnel before the UE's DEACTIVATE EPS BEARER CONTEXT ACCEPT. The PDN
	// connection is retained for the ESM handshake (T3495); its later session release is
	// then an idempotent no-op.
	m.releaseAnchorSession(ctx, ue, p)

	if releaseOnly {
		// The eNB releases the radio bearer, but the NAS DEACTIVATE EPS BEARER
		// CONTEXT REQUEST still needs an answer: guard it with T3495 so it is
		// retransmitted, and on exhaustion release only this PDN connection
		// locally, leaving the UE attached (TS 24.301 §6.4.4.5).
		m.ArmESMGuardAbortOnly(ue, p, "Deactivate EPS Bearer Context Request", plain, eps.SHTIntegrityProtectedCiphered, func() {
			m.ReleasePDN(ctx, ue, p)
		})

		return
	}

	m.ArmESMGuard(ue, p, "Deactivate EPS Bearer Context Request", plain, eps.SHTIntegrityProtectedCiphered)
}

// DisconnectBearer tears down the UE's PDN connection p with a regular
// deactivation; the UE is not asked to re-establish it.
func (m *MME) DisconnectBearer(ctx context.Context, ue *UeContext, p *PdnConnection, esmCause eps.ESMCause, pti uint8) {
	m.DeactivateBearer(ctx, ue, p, esmCause, pti, true)
}

// sendERABRelease releases a UE's E-RAB at the eNB while the UE stays connected,
// carrying the DEACTIVATE EPS BEARER CONTEXT REQUEST in the NAS-PDU so the eNB
// both releases the radio bearer and delivers the NAS (TS 36.413 §8.2.3).
func (m *MME) sendERABRelease(ctx context.Context, ueConn *UeConn, p *PdnConnection, naspdu []byte) {
	cmd := &s1ap.ERABReleaseCommand{
		ERABToBeReleased: []s1ap.ERABItem{{
			ERABID: s1ap.ERABID(p.Ebi),
			Cause:  CauseNASNormalRelease,
		}},
		NASPDU: s1ap.NASPDU(naspdu),
	}

	if err := ueConn.SendERABRelease(ctx, cmd); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to send E-RAB Release Command", zap.Error(err))
		return
	}
}
