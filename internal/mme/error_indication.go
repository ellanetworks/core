// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// causeUnknownPairUES1APID is S1AP Cause Radio Network "unknown-pair-ue-s1ap-id"
// (TS 36.413): a UE-associated message whose eNB-UE-S1AP-ID does not match the
// one stored against its MME-UE-S1AP-ID.
var causeUnknownPairUES1APID = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUnknownPairUES1APID}

// resolveUE looks up a UE context for a UE-associated S1AP message and validates
// the AP ID pair (TS 36.413). At the MME the local AP ID is the MME-UE-S1AP-ID
// and the remote AP ID is the eNB-UE-S1AP-ID, so the UE is found by the
// MME-UE-S1AP-ID and the eNB-UE-S1AP-ID is then cross-checked.
//
//   - An MME-UE-S1AP-ID the MME does not hold is an unknown local AP ID.
//   - An MME-UE-S1AP-ID held by a UE on a different S1 association does not name
//     a UE-associated logical S1 connection on the receiving association, so on
//     that association it is an unknown local AP ID. The global MME-UE-S1AP-ID
//     map is shared across eNBs; this scopes resolution to the sender so one eNB
//     cannot act on a UE attached through another (TS 36.413).
//   - An MME-UE-S1AP-ID held by an ECM-IDLE UE no longer identifies an active
//     UE-associated logical S1 connection (the connection was released; the UE
//     re-establishes under a fresh AP ID), so it is also an unknown local AP ID.
//   - An eNB-UE-S1AP-ID different from the stored one is an inconsistent pair.
//
// On any of these an Error Indication carrying the received AP IDs is returned
// to the sender (TS 36.413) and the function returns (nil, false).
func (m *MME) resolveUE(conn nasWriter, mmeID s1ap.MMEUES1APID, enbID s1ap.ENBUES1APID) (*UeContext, bool) {
	ue, ok := m.lookupUe(mmeID)
	if !ok {
		logger.MmeLog.Warn("UE-associated S1AP message with unknown MME-UE-S1AP-ID",
			zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint32("enb-ue-id", uint32(enbID)))
		m.sendErrorIndication(conn, &mmeID, &enbID, causeUnknownMMEUES1APID)

		return nil, false
	}

	if ue.s1.conn != conn {
		logger.MmeLog.Warn("UE-associated S1AP message for an MME-UE-S1AP-ID on a different S1 association",
			zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint32("enb-ue-id", uint32(enbID)))
		m.sendErrorIndication(conn, &mmeID, &enbID, causeUnknownMMEUES1APID)

		return nil, false
	}

	if !ue.connected() {
		logger.MmeLog.Warn("UE-associated S1AP message for an MME-UE-S1AP-ID with no active S1 connection",
			zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint32("enb-ue-id", uint32(enbID)))
		m.sendErrorIndication(conn, &mmeID, &enbID, causeUnknownMMEUES1APID)

		return nil, false
	}

	if ue.s1.ENBUES1APID != enbID {
		logger.MmeLog.Warn("UE-associated S1AP message with an inconsistent eNB-UE-S1AP-ID",
			zap.Uint32("mme-ue-id", uint32(mmeID)),
			zap.Uint32("stored-enb-ue-id", uint32(ue.s1.ENBUES1APID)),
			zap.Uint32("received-enb-ue-id", uint32(enbID)))
		m.sendErrorIndication(conn, &mmeID, &enbID, causeUnknownPairUES1APID)

		return nil, false
	}

	return ue, true
}

// sendErrorIndication replies to the sending eNB with an ERROR INDICATION
// carrying the UE S1AP ID pair and a cause (TS 36.413).
func (m *MME) sendErrorIndication(conn nasWriter, mmeID *s1ap.MMEUES1APID, enbID *s1ap.ENBUES1APID, cause s1ap.Cause) {
	c := cause
	ind := &s1ap.ErrorIndication{MMEUES1APID: mmeID, ENBUES1APID: enbID, Cause: &c}

	b, err := ind.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Error Indication", zap.Error(err))
		return
	}

	if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamNonUE}); err != nil {
		logger.MmeLog.Error("failed to send Error Indication", zap.Error(err))
		return
	}

	// Error Indications are sent from resolution failures across many handlers,
	// some outside a request span; use a fresh root.
	m.logOutboundS1AP(context.Background(), conn, S1APProcedureErrorIndication, b)
}

// handleErrorIndication processes an ERROR INDICATION from the eNB (TS 36.413).
// A protocol error on a UE-associated S1 connection leaves it in an
// inconsistent state, so if the indication names a known UE the MME releases it
// to ECM-IDLE; the UE re-establishes on its next Service Request.
func (m *MME) handleErrorIndication(ctx context.Context, _ *sctp.SCTPConn, value []byte) {
	msg, err := s1ap.ParseErrorIndication(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Error Indication", zap.Error(err))
		return
	}

	fields := make([]zap.Field, 0, 4)
	if msg.MMEUES1APID != nil {
		fields = append(fields, zap.Uint32("mme-ue-id", uint32(*msg.MMEUES1APID)))
	}

	if msg.ENBUES1APID != nil {
		fields = append(fields, zap.Uint32("enb-ue-id", uint32(*msg.ENBUES1APID)))
	}

	if msg.Cause != nil {
		fields = append(fields, zap.String("cause", s1apCauseName(msg.Cause)))
	}

	logger.MmeLog.Warn("Error Indication", fields...)

	if msg.MMEUES1APID == nil {
		return
	}

	if ue, ok := m.lookupUe(*msg.MMEUES1APID); ok {
		m.releaseUEContext(ctx, ue, causeNASUnspecified)
	}
}
