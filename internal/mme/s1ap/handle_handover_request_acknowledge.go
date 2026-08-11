// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleHandoverRequestAcknowledge records the target's downlink endpoints and
// sends a HANDOVER COMMAND to the source, or fails the handover when no usable
// bearer was admitted (TS 36.413 §8.4.2).
func handleHandoverRequestAcknowledge(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	ack, err := s1ap.ParseHandoverRequestAcknowledge(value)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Handover Request Acknowledge", zap.Error(err))
		return
	}

	reportDiagnostics(m, ctx, radio.Conn, s1ap.ProcHandoverResourceAllocation, s1ap.TriggeringSuccessfulOutcome, nodeLevel(), ack.Diagnostics())

	if ack.MMEUES1APID == nil || ack.ENBUES1APID == nil {
		logger.From(ctx, logger.MmeLog).Warn("Handover Request Acknowledge without both UE S1AP IDs")
		sendErrorIndication(m, radio.Conn, ack.MMEUES1APID, ack.ENBUES1APID, causeMissingUES1APID)

		return
	}

	mmeUEID, enbUEID := *ack.MMEUES1APID, *ack.ENBUES1APID

	ue, ok := m.LookupUe(mmeUEID)
	if !ok {
		sendErrorIndication(m, radio.Conn, &mmeUEID, &enbUEID, causeUnknownMMEUES1APID)
		return
	}

	ue.TouchLastSeen()

	if !m.MatchAndSetTargetENB(ue, mmeUEID, enbUEID, radio.Conn) {
		logger.From(ctx, logger.MmeLog).Warn("Handover Request Acknowledge with no matching preparation; dropping",
			zap.Uint32("target-mme-ue-id", uint32(mmeUEID)))

		return
	}

	admitted := make([]mme.AdmittedERAB, 0, len(ack.ERABAdmitted))

	for _, it := range ack.ERABAdmitted {
		addr, ok := enbTransportAddress(it.TransportLayerAddress)
		if !ok {
			logger.From(ctx, logger.MmeLog).Warn("Handover Request Acknowledge E-RAB has an invalid target address; treating as failed",
				zap.Uint32("target-mme-ue-id", uint32(mmeUEID)), zap.Uint8("e-rab-id", uint8(it.ERABID)))

			continue
		}

		admitted = append(admitted, mme.AdmittedERAB{Ebi: uint8(it.ERABID), EnbFTEID: models.FTEID{TEID: uint32(it.GTPTEID), Addr: addr}})
	}

	targetCauses := failedERABCauses(ack.ERABFailedToSetup)

	if len(admitted) == 0 {
		logger.From(ctx, logger.MmeLog).Warn("Handover Request Acknowledge admitted no E-RAB; rejecting handover",
			zap.Uint32("target-mme-ue-id", uint32(mmeUEID)))
		mme.SendUEContextRelease(ctx, m, radio.Conn, mmeUEID, enbUEID, true, causeHOFailureInTarget)
		m.FailHandoverToSource(ctx, ue, causeHOFailureInTarget)

		return
	}

	unadmitted, sourceConn, sourceMMEID, sourceENBID, ok := m.MarkHandoverPrepared(ue, mmeUEID, radio.Conn, admitted)
	if !ok {
		return
	}

	// No source eNB: the UE is arriving from 5GS, and the source gNB is commanded
	// by the peer AMF rather than from here (TS 23.502 §4.11.1.2.1 steps 9-10).
	if sourceConn == nil {
		logger.From(ctx, logger.MmeLog).Info("Forward Relocation Response",
			zap.Uint32("target-mme-ue-id", uint32(mmeUEID)),
			zap.Int("admitted", len(admitted)),
			zap.Int("not-admitted", len(unadmitted)))
		m.FinishRelocationPreparation(ue, ack.TargetToSource, unadmitted)

		return
	}

	cmd := &s1ap.HandoverCommand{
		MMEUES1APID:    sourceMMEID,
		ENBUES1APID:    sourceENBID,
		HandoverType:   s1ap.HandoverTypeIntraLTE,
		ERABToRelease:  releaseItems(unadmitted, targetCauses),
		TargetToSource: ack.TargetToSource,
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal Handover Command", zap.Error(err))
		return
	}

	logger.From(ctx, logger.MmeLog).Info("Handover Command",
		zap.Uint32("mme-ue-id", uint32(sourceMMEID)),
		zap.Int("admitted", len(admitted)),
		zap.Int("released", len(unadmitted)))
	m.SendToRadio(ctx, sourceConn, mme.S1APProcedureHandoverCommand, b)
}

func failedERABCauses(failed []s1ap.ERABItem) map[uint8]s1ap.Cause {
	causes := make(map[uint8]s1ap.Cause, len(failed))

	for _, it := range failed {
		causes[uint8(it.ERABID)] = it.Cause
	}

	return causes
}

func releaseItems(unadmitted []mme.HandoverCandidate, targetCauses map[uint8]s1ap.Cause) []s1ap.ERABItem {
	if len(unadmitted) == 0 {
		return nil
	}

	out := make([]s1ap.ERABItem, 0, len(unadmitted))

	for _, c := range unadmitted {
		cause := causeHOFailureInTarget

		switch {
		case c.Cause != nil:
			cause = *c.Cause
		default:
			if reported, ok := targetCauses[c.Ebi]; ok {
				cause = reported
			}
		}

		out = append(out, s1ap.ERABItem{ERABID: s1ap.ERABID(c.Ebi), Cause: cause})
	}

	return out
}
