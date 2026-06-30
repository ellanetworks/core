// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// causeUnknownPairUES1APID is S1AP Cause Radio Network "unknown-pair-ue-s1ap-id"
// (TS 36.413): a UE-associated message whose eNB-UE-S1AP-ID does not match the
// one stored against its MME-UE-S1AP-ID.
var causeUnknownPairUES1APID = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUnknownPairUES1APID}

// resolveUE finds a UE-associated message's UE by its MME-UE-S1AP-ID and
// cross-checks the eNB-UE-S1AP-ID, returning (nil, false) and an Error Indication
// to the sender otherwise (TS 36.413). The MME-UE-S1AP-ID map is shared across
// eNBs, so a hit is scoped to the sending association: this stops one eNB acting
// on a UE attached through another.
func resolveUE(m *mme.MME, conn mme.NasWriter, mmeID s1ap.MMEUES1APID, enbID s1ap.ENBUES1APID) (*mme.UeContext, bool) {
	ue, ok := m.LookupUe(mmeID)
	if !ok {
		logger.MmeLog.Warn("UE-associated S1AP message with unknown MME-UE-S1AP-ID",
			zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint32("enb-ue-id", uint32(enbID)))
		sendErrorIndication(m, conn, &mmeID, &enbID, causeUnknownMMEUES1APID)

		return nil, false
	}

	if ue.S1.Conn() != conn {
		logger.MmeLog.Warn("UE-associated S1AP message for an MME-UE-S1AP-ID on a different S1 association",
			zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint32("enb-ue-id", uint32(enbID)))
		sendErrorIndication(m, conn, &mmeID, &enbID, causeUnknownMMEUES1APID)

		return nil, false
	}

	if !ue.Connected() {
		logger.MmeLog.Warn("UE-associated S1AP message for an MME-UE-S1AP-ID with no active S1 connection",
			zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint32("enb-ue-id", uint32(enbID)))
		sendErrorIndication(m, conn, &mmeID, &enbID, causeUnknownMMEUES1APID)

		return nil, false
	}

	if ue.S1.ENBUES1APID != enbID {
		logger.MmeLog.Warn("UE-associated S1AP message with an inconsistent eNB-UE-S1AP-ID",
			zap.Uint32("mme-ue-id", uint32(mmeID)),
			zap.Uint32("stored-enb-ue-id", uint32(ue.S1.ENBUES1APID)),
			zap.Uint32("received-enb-ue-id", uint32(enbID)))
		sendErrorIndication(m, conn, &mmeID, &enbID, causeUnknownPairUES1APID)

		return nil, false
	}

	return ue, true
}

// sendErrorIndication replies to the sending eNB with an ERROR INDICATION
// carrying the UE S1AP ID pair and a cause (TS 36.413).
func sendErrorIndication(m *mme.MME, conn mme.NasWriter, mmeID *s1ap.MMEUES1APID, enbID *s1ap.ENBUES1APID, cause s1ap.Cause) {
	c := cause
	emitErrorIndication(m, conn, &s1ap.ErrorIndication{MMEUES1APID: mmeID, ENBUES1APID: enbID, Cause: &c})
}

// handleParseError reports a failed decode of an eNB-initiated initiating message
// with an ERROR INDICATION carrying Cause "transfer-syntax-error" and Criticality
// Diagnostics naming the procedure (TS 36.413 §10.4). It must not be used in reply
// to an ERROR INDICATION, to avoid a loop.
func handleParseError(m *mme.MME, conn mme.NasWriter, proc s1ap.ProcedureCode, err error) {
	logger.MmeLog.Warn("failed to decode S1AP message",
		zap.Int("procedure-code", int(proc)),
		zap.Error(err))

	crit := s1ap.CriticalityReject
	trigger := s1ap.TriggeringInitiatingMessage

	emitErrorIndication(m, conn, &s1ap.ErrorIndication{
		Cause: &s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: s1ap.CauseProtocolTransferSyntaxError},
		CriticalityDiagnostics: &s1ap.CriticalityDiagnostics{
			ProcedureCode:        &proc,
			TriggeringMessage:    &trigger,
			ProcedureCriticality: &crit,
		},
	})
}

// emitErrorIndication marshals and sends an ERROR INDICATION to the eNB.
func emitErrorIndication(m *mme.MME, conn mme.NasWriter, ind *s1ap.ErrorIndication) {
	b, err := ind.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Error Indication", zap.Error(err))
		return
	}

	if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: mme.S1apWirePPID, Stream: mme.S1apStreamNonUE}); err != nil {
		logger.MmeLog.Error("failed to send Error Indication", zap.Error(err))
		return
	}

	// Resolution failures fire from many handlers, some outside a request span;
	// fresh root.
	m.LogOutboundS1AP(context.Background(), conn, mme.S1APProcedureErrorIndication, b)
}

// handleErrorIndication processes an ERROR INDICATION from the eNB (TS 36.413).
// A protocol error on a UE-associated S1 connection leaves it in an
// inconsistent state, so if the indication names a known UE the MME releases it
// to ECM-IDLE; the UE re-establishes on its next Service Request.
func handleErrorIndication(m *mme.MME, ctx context.Context, _ *sctp.SCTPConn, value []byte) {
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
		fields = append(fields, zap.String("cause", mme.S1apCauseName(msg.Cause)))
	}

	logger.MmeLog.Warn("Error Indication", fields...)

	if msg.MMEUES1APID == nil {
		return
	}

	if ue, ok := m.LookupUe(*msg.MMEUES1APID); ok {
		m.ReleaseUEContext(ctx, ue, mme.CauseNASUnspecified)
	}
}
