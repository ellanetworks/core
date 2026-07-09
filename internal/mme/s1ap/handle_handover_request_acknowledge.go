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

	ue, ok := m.LookupUe(ack.MMEUES1APID)
	if !ok {
		// Unknown local MME-UE-S1AP-ID, e.g. a Handover Cancel freed the target
		// reservation while this acknowledge was in flight. TS 36.413 §10.6: an Error
		// Indication makes both nodes locally release the connection, freeing the
		// target eNB's reserved resources without a UE Context Release.
		sendErrorIndication(m, radio.Conn, &ack.MMEUES1APID, &ack.ENBUES1APID, causeUnknownMMEUES1APID)
		return
	}

	ue.TouchLastSeen()

	if !m.MatchAndSetTargetENB(ue, ack.MMEUES1APID, ack.ENBUES1APID, radio.Conn) {
		// A UE with no matching handover preparation: a duplicate or stale acknowledge,
		// e.g. for a UE whose association id is its active one. Releasing here would
		// drop a live UE, so drop the message; TS 36.413 §10.4 (response incompatible
		// with receiver state) calls for local error handling.
		logger.From(ctx, logger.MmeLog).Warn("Handover Request Acknowledge with no matching preparation; dropping",
			zap.Uint32("target-mme-ue-id", uint32(ack.MMEUES1APID)))

		return
	}

	admitted := make([]mme.AdmittedERAB, 0, len(ack.ERABAdmitted))

	for _, it := range ack.ERABAdmitted {
		addr, ok := enbTransportAddress(it.TransportLayerAddress)
		if !ok {
			logger.From(ctx, logger.MmeLog).Warn("Handover Request Acknowledge E-RAB has an invalid target address; treating as failed",
				zap.Uint32("target-mme-ue-id", uint32(ack.MMEUES1APID)), zap.Uint8("e-rab-id", uint8(it.ERABID)))

			continue
		}

		admitted = append(admitted, mme.AdmittedERAB{Ebi: uint8(it.ERABID), EnbFTEID: models.FTEID{TEID: uint32(it.GTPTEID), Addr: addr}})
	}

	// Each E-RAB is a PDN's default bearer, so a rejected one releases its PDN
	// (TS 23.401 §5.5.1.2.2 step 15).
	releaseEBIs := failedHandoverEBIs(ack, admitted)

	if len(admitted) == 0 {
		// No default bearer admitted: the handover is rejected (TS 23.401 §5.5.1.2.3).
		logger.From(ctx, logger.MmeLog).Warn("Handover Request Acknowledge admitted no E-RAB; rejecting handover",
			zap.Uint32("target-mme-ue-id", uint32(ack.MMEUES1APID)))
		mme.SendUEContextRelease(ctx, m, radio.Conn, ack.MMEUES1APID, ack.ENBUES1APID, true, causeHOFailureInTarget)
		m.FailHandoverToSource(ctx, ue, causeHOFailureInTarget)

		return
	}

	sourceConn, sourceMMEID, sourceENBID, ok := m.MarkHandoverPrepared(ue, ack.MMEUES1APID, radio.Conn, admitted, releaseEBIs)
	if !ok {
		return
	}

	cmd := &s1ap.HandoverCommand{
		MMEUES1APID:    sourceMMEID,
		ENBUES1APID:    sourceENBID,
		HandoverType:   s1ap.HandoverTypeIntraLTE,
		ERABToRelease:  releaseItems(releaseEBIs),
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

// failedHandoverEBIs returns the EPS bearer identities the target eNB reported
// failed to set up and that are absent from the admitted set.
func failedHandoverEBIs(ack *s1ap.HandoverRequestAcknowledge, admitted []mme.AdmittedERAB) []uint8 {
	admittedSet := make(map[uint8]struct{}, len(admitted))
	for _, a := range admitted {
		admittedSet[a.Ebi] = struct{}{}
	}

	seen := make(map[uint8]struct{})

	var out []uint8

	add := func(ebi uint8) {
		if _, ok := admittedSet[ebi]; ok {
			return
		}

		if _, ok := seen[ebi]; ok {
			return
		}

		seen[ebi] = struct{}{}
		out = append(out, ebi)
	}

	for _, it := range ack.ERABFailedToSetup {
		add(uint8(it.ERABID))
	}

	return out
}

// releaseItems renders EPS bearer identities as the E-RABs to Release List of a
// HANDOVER COMMAND (TS 36.413 §9.1.5.2).
func releaseItems(ebis []uint8) []s1ap.ERABItem {
	if len(ebis) == 0 {
		return nil
	}

	out := make([]s1ap.ERABItem, 0, len(ebis))
	for _, ebi := range ebis {
		out = append(out, s1ap.ERABItem{ERABID: s1ap.ERABID(ebi), Cause: causeHOFailureInTarget})
	}

	return out
}
