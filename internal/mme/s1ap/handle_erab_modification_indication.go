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

// causeERABModOmittedERAB is the UE Context Release cause when an E-RAB
// Modification Indication omits an established E-RAB (TS 36.413 §8.2.4.4): no
// specific radio-network cause applies, so "unspecified".
var causeERABModOmittedERAB = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUnspecified}

// handleERABModificationIndication relocates the downlink S1-U endpoint of the
// UE's established E-RABs to the addresses the eNB reports (the DC / secondary-node
// bearer-relocation case) and confirms them (TS 36.413 §8.2.4). Per §8.2.4.4, an
// indication that repeats an E-RAB ID or omits an E-RAB already in the UE context
// is abnormal and triggers a UE Context Release rather than a modification.
func handleERABModificationIndication(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseERABModificationIndication(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcERABModificationIndication, err)
		return
	}

	ue, ok := m.LookupUe(msg.MMEUES1APID)
	if !ok {
		// The procedure has no failure message; an unresolvable UE is dropped.
		logger.From(ctx, logger.MmeLog).Warn("E-RAB Modification Indication for unknown UE",
			zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))

		return
	}

	ue.TouchLastSeen()

	if id, dup := duplicateModifiedERABID(msg); dup {
		logger.From(ctx, logger.MmeLog).Warn("E-RAB Modification Indication repeats an E-RAB ID; releasing UE context",
			zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)), zap.Uint8("e-rab-id", uint8(id)))
		m.ReleaseUEContext(ctx, ue, causeMultipleERABInstances)

		return
	}

	if ebi, omitted := omittedEstablishedERAB(ue, msg); omitted {
		logger.From(ctx, logger.MmeLog).Warn("E-RAB Modification Indication omits an established E-RAB; releasing UE context",
			zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)), zap.Uint8("e-rab-id", ebi))
		m.ReleaseUEContext(ctx, ue, causeERABModOmittedERAB)

		return
	}

	modified := modifyBearerDownlinks(m, ctx, ue, msg.ToBeModified)

	confirm := &s1ap.ERABModificationConfirm{
		MMEUES1APID:   msg.MMEUES1APID,
		ENBUES1APID:   msg.ENBUES1APID,
		ModifiedERABs: modified,
	}

	b, err := confirm.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal E-RAB Modification Confirm", zap.Error(err))
		return
	}

	logger.From(ctx, logger.MmeLog).Info("E-RAB Modification Indication",
		zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)),
		zap.Int("e-rabs-modified", len(modified)))

	m.SendS1APConn(ctx, radio.Conn, mme.S1APProcedureERABModificationConfirm, b)
}

// modifyBearerDownlinks relocates each listed E-RAB's downlink to the eNB S1-U
// endpoint it names, returning the E-RABs successfully relocated. Reuses the same
// user-plane path as a Path Switch (TS 36.413 §8.2.4.2).
func modifyBearerDownlinks(m *mme.MME, ctx context.Context, ue *mme.UeContext, items []s1ap.ERABToBeModifiedItemBearerModInd) []s1ap.ERABID {
	var modified []s1ap.ERABID

	for _, erab := range items {
		p := m.LookupPDN(ue, uint8(erab.ERABID))
		if p == nil {
			logger.From(ctx, logger.MmeLog).Warn("E-RAB Modification Indication lists an unknown E-RAB; skipped",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", uint8(erab.ERABID)))

			continue
		}

		addr, ok := enbTransportAddress(erab.TransportLayerAddress)
		if !ok {
			logger.From(ctx, logger.MmeLog).Warn("E-RAB Modification Indication has an invalid eNB transport address; skipped",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", uint8(erab.ERABID)))

			continue
		}

		fteid := models.FTEID{TEID: uint32(erab.DLGTPTEID), Addr: addr}

		if err := m.Session.ModifyEPSSession(ctx, ue.IMSI(), p.Ebi, fteid); err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to relocate an E-RAB downlink",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", uint8(erab.ERABID)), zap.Error(err))

			continue
		}

		p.EnbFTEID = fteid

		modified = append(modified, erab.ERABID)

		logger.From(ctx, logger.MmeLog).Debug("E-RAB downlink relocated",
			zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", uint8(erab.ERABID)), zap.String("enb-s1u", addr.String()))
	}

	return modified
}

// duplicateModifiedERABID reports the first E-RAB ID appearing more than once
// across the indication's to-be-modified and not-to-be-modified lists (TS 36.413 §8.2.4.4).
func duplicateModifiedERABID(msg *s1ap.ERABModificationIndication) (s1ap.ERABID, bool) {
	seen := make(map[s1ap.ERABID]struct{})

	for _, it := range msg.ToBeModified {
		if _, dup := seen[it.ERABID]; dup {
			return it.ERABID, true
		}

		seen[it.ERABID] = struct{}{}
	}

	for _, it := range msg.NotToBeModified {
		if _, dup := seen[it.ERABID]; dup {
			return it.ERABID, true
		}

		seen[it.ERABID] = struct{}{}
	}

	return 0, false
}

// omittedEstablishedERAB reports an E-RAB in the UE context that the indication
// lists in neither its to-be-modified nor its not-to-be-modified list (TS 36.413 §8.2.4.4).
func omittedEstablishedERAB(ue *mme.UeContext, msg *s1ap.ERABModificationIndication) (uint8, bool) {
	present := make(map[uint8]struct{})

	for _, it := range msg.ToBeModified {
		present[uint8(it.ERABID)] = struct{}{}
	}

	for _, it := range msg.NotToBeModified {
		present[uint8(it.ERABID)] = struct{}{}
	}

	for _, ebi := range ue.ActiveEBIs() {
		if _, ok := present[ebi]; !ok {
			return ebi, true
		}
	}

	return 0, false
}
