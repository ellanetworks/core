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
		// Unknown local MME-UE-S1AP-ID, e.g. a Handover Cancel freed the target
		// reservation while this acknowledge was in flight. TS 36.413 §10.6: an Error
		// Indication makes both nodes locally release the connection, freeing the
		// target eNB's reserved resources without a UE Context Release.
		sendErrorIndication(m, radio.Conn, &mmeUEID, &enbUEID, causeUnknownMMEUES1APID)
		return
	}

	ue.TouchLastSeen()

	if !m.MatchAndSetTargetENB(ue, mmeUEID, enbUEID, radio.Conn) {
		// A UE with no matching handover preparation: a duplicate or stale acknowledge,
		// e.g. for a UE whose association id is its active one. Releasing here would
		// drop a live UE, so drop the message; TS 36.413 §10.4 (response incompatible
		// with receiver state) calls for local error handling.
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

	// The causes the target gave for the bearers it refused, relayed per E-RAB in
	// the HANDOVER COMMAND so the source eNB learns why (TS 36.413 §9.1.5.2).
	targetCauses := failedERABCauses(ack)

	if len(admitted) == 0 {
		// No default bearer admitted: the handover is rejected (TS 23.401 §5.5.1.2.3).
		logger.From(ctx, logger.MmeLog).Warn("Handover Request Acknowledge admitted no E-RAB; rejecting handover",
			zap.Uint32("target-mme-ue-id", uint32(mmeUEID)))
		mme.SendUEContextRelease(ctx, m, radio.Conn, mmeUEID, enbUEID, true, causeHOFailureInTarget)
		m.FailHandoverToSource(ctx, ue, causeHOFailureInTarget)

		return
	}

	releaseEBIs, sourceConn, sourceMMEID, sourceENBID, ok := m.MarkHandoverPrepared(ue, mmeUEID, radio.Conn, admitted)
	if !ok {
		return
	}

	cmd := &s1ap.HandoverCommand{
		MMEUES1APID:    sourceMMEID,
		ENBUES1APID:    sourceENBID,
		HandoverType:   s1ap.HandoverTypeIntraLTE,
		ERABToRelease:  releaseItems(releaseEBIs, targetCauses),
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
		zap.Int("released", len(releaseEBIs)))
	m.SendToRadio(ctx, sourceConn, mme.S1APProcedureHandoverCommand, b)
}

// failedERABCauses indexes the causes the target eNB gave for the E-RABs it could
// not set up (TS 36.413 §9.1.5.5).
func failedERABCauses(ack *s1ap.HandoverRequestAcknowledge) map[uint8]s1ap.Cause {
	causes := make(map[uint8]s1ap.Cause, len(ack.ERABFailedToSetup))
	for _, it := range ack.ERABFailedToSetup {
		causes[uint8(it.ERABID)] = it.Cause
	}

	return causes
}

func releaseItems(ebis []uint8, targetCauses map[uint8]s1ap.Cause) []s1ap.ERABItem {
	if len(ebis) == 0 {
		return nil
	}

	out := make([]s1ap.ERABItem, 0, len(ebis))

	for _, ebi := range ebis {
		cause := causeHOFailureInTarget
		if reported, ok := targetCauses[ebi]; ok {
			cause = reported
		}

		out = append(out, s1ap.ERABItem{ERABID: s1ap.ERABID(ebi), Cause: cause})
	}

	return out
}
