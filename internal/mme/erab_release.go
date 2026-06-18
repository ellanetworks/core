// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// deactivateBearer asks the UE to deactivate one EPS bearer, carrying the
// DEACTIVATE EPS BEARER CONTEXT REQUEST with the given ESM cause and procedure
// transaction identity (TS 24.301 §6.4.4).
//
// When the deactivation leaves the UE connected — a PDN disconnect, or a
// reactivation of an additional PDN — the NAS rides in an S1AP E-RAB RELEASE
// COMMAND so the eNB releases that radio bearer in the same step (TS 23.401
// §5.10.3 "Deactivate Bearer Request"). When it instead deactivates the attach
// (first) bearer with reactivation requested, the UE re-attaches and the full UE
// Context Release that follows tears down the radio bearers, so the NAS is sent
// on a Downlink NAS Transport and guarded like other common procedures.
func (m *MME) deactivateBearer(ue *UeContext, p *pdnConnection, esmCause, pti uint8, disconnecting bool) {
	naspdu, err := m.protectDownlink(ue, &eps.DeactivateEPSBearerContextRequest{
		EPSBearerIdentity:            p.ebi,
		ProcedureTransactionIdentity: pti,
		ESMCause:                     esmCause,
	})
	if err != nil {
		logger.MmeLog.Error("failed to protect Deactivate EPS Bearer Context Request", zap.Error(err))
		return
	}

	p.deactivating = true
	p.disconnecting = disconnecting

	if disconnecting || p.ebi != ue.defaultEBI {
		m.sendERABRelease(ue, p, naspdu)
		return
	}

	m.sendDownlink(ue, naspdu)
	m.armNASGuard(ue, "Deactivate EPS Bearer Context Request", naspdu)
}

// sendERABRelease releases a UE's E-RAB at the eNB while the UE stays connected,
// carrying the DEACTIVATE EPS BEARER CONTEXT REQUEST in the NAS-PDU so the eNB
// both releases the radio bearer and delivers the NAS (TS 36.413 §8.2.3).
func (m *MME) sendERABRelease(ue *UeContext, p *pdnConnection, naspdu []byte) {
	cmd := &s1ap.ERABReleaseCommand{
		MMEUES1APID: ue.MMEUES1APID,
		ENBUES1APID: ue.ENBUES1APID,
		ERABToBeReleased: []s1ap.ERABItem{{
			ERABID: s1ap.ERABID(p.ebi),
			Cause:  causeNASNormalRelease,
		}},
		NASPDU: s1ap.NASPDU(naspdu),
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal E-RAB Release Command", zap.Error(err))
		return
	}

	m.sendS1AP(ue, S1APProcedureERABReleaseCommand, b)
}

// handleERABReleaseResponse logs the eNB's confirmation that the radio bearer was
// released (TS 36.413 §8.2.3). The PDN connection's session is released when the
// UE answers the DEACTIVATE EPS BEARER CONTEXT REQUEST (onDeactivateBearerAccept).
func (m *MME) handleERABReleaseResponse(conn nasWriter, value []byte) {
	msg, err := s1ap.ParseERABReleaseResponse(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode E-RAB Release Response", zap.Error(err))
		return
	}

	ue, ok := m.resolveUE(conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	for _, erab := range msg.ERABReleased {
		logger.MmeLog.Info("E-RAB released at eNB",
			zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", ue.imsi),
			zap.Uint8("e-rab-id", uint8(erab.ERABID)))
	}
}
