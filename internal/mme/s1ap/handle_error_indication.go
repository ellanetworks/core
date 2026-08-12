// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// causeUnknownPairUES1APID is S1AP Cause Radio Network "unknown-pair-ue-s1ap-id"
// (TS 36.413): a UE-associated message whose eNB-UE-S1AP-ID does not match the
// one stored against its MME-UE-S1AP-ID.
var causeUnknownPairUES1APID = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUnknownPairUES1APID}

// resolveUE finds a UE-associated message's UE by its MME-UE-S1AP-ID and
// cross-checks the eNB-UE-S1AP-ID, returning (nil, nil, false) and an Error
// Indication to the sender otherwise (TS 36.413). The MME-UE-S1AP-ID map is
// shared across eNBs, so a hit is scoped to the sending association: this stops
// one eNB acting on a UE attached through another.
//
// Handlers must use the returned connection rather than re-reading ue.Conn(),
// which is nil in ECM-IDLE and can become nil at any point. A nil dereference
// panics the dispatch goroutine, and the dispatcher answers by aborting the
// association, dropping every other UE on that eNB.
// resolveUEQuiet resolves a UE-associated message that is the last message of
// its logical connection: TS 36.413 §10.6 answers an unknown one with a local
// release rather than an Error Indication, so the caller reports the miss
// itself.
func resolveUEQuiet(m *mme.MME, conn mme.S1APWriter, mmeID s1ap.MMEUES1APID, enbID s1ap.ENBUES1APID) (*mme.UeContext, *mme.UeConn, bool) {
	ue, ok := m.LookupUe(mmeID)
	if !ok {
		return nil, nil, false
	}

	ueConn := ue.Conn()
	if ueConn == nil || ueConn.Conn() != conn || ueConn.ENBUES1APID != enbID {
		return nil, nil, false
	}

	return ue, ueConn, true
}

func resolveUE(m *mme.MME, conn mme.S1APWriter, mmeID s1ap.MMEUES1APID, enbID s1ap.ENBUES1APID) (*mme.UeContext, *mme.UeConn, bool) {
	ue, ok := m.LookupUe(mmeID)
	if !ok {
		logger.MmeLog.Warn("UE-associated S1AP message with unknown MME-UE-S1AP-ID",
			zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint32("enb-ue-id", uint32(enbID)))
		sendErrorIndication(m, conn, &mmeID, &enbID, causeUnknownMMEUES1APID)

		return nil, nil, false
	}

	// Before the checks, so they and the handler see one connection rather than
	// whatever ue.active holds at each read.
	ueConn := ue.Conn()
	if ueConn == nil {
		logger.MmeLog.Warn("UE-associated S1AP message for an MME-UE-S1AP-ID with no active S1 connection",
			zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint32("enb-ue-id", uint32(enbID)))
		sendErrorIndication(m, conn, &mmeID, &enbID, causeUnknownMMEUES1APID)

		return nil, nil, false
	}

	if ueConn.Conn() != conn {
		logger.MmeLog.Warn("UE-associated S1AP message for an MME-UE-S1AP-ID on a different S1 association",
			zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint32("enb-ue-id", uint32(enbID)))
		sendErrorIndication(m, conn, &mmeID, &enbID, causeUnknownMMEUES1APID)

		return nil, nil, false
	}

	if ueConn.ENBUES1APID != enbID {
		logger.MmeLog.Warn("UE-associated S1AP message with an inconsistent eNB-UE-S1AP-ID",
			zap.Uint32("mme-ue-id", uint32(mmeID)),
			zap.Uint32("stored-enb-ue-id", uint32(ueConn.ENBUES1APID)),
			zap.Uint32("received-enb-ue-id", uint32(enbID)))
		sendErrorIndication(m, conn, &mmeID, &enbID, causeUnknownPairUES1APID)

		return nil, nil, false
	}

	return ue, ueConn, true
}

// causeMissingUES1APID answers a UE-associated message that omitted a UE S1AP
// ID: without it the MME cannot address a UE context, so the procedure is
// rejected (TS 36.413 §10.3.5).
var causeMissingUES1APID = s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: s1ap.CauseProtocolAbstractSyntaxErrorReject}

// resolveUEIDs is resolveUE for a message whose UE S1AP IDs carry ignore
// criticality and may therefore be absent.
func resolveUEIDs(m *mme.MME, conn mme.S1APWriter, mmeID *s1ap.MMEUES1APID, enbID *s1ap.ENBUES1APID) (*mme.UeContext, *mme.UeConn, bool) {
	if mmeID == nil || enbID == nil {
		logger.MmeLog.Warn("UE-associated S1AP message without both UE S1AP IDs")
		sendErrorIndication(m, conn, mmeID, enbID, causeMissingUES1APID)

		return nil, nil, false
	}

	return resolveUE(m, conn, *mmeID, *enbID)
}

// sendErrorIndication replies to the sending eNB with an ERROR INDICATION
// carrying the UE S1AP ID pair and a cause (TS 36.413).
func sendErrorIndication(m *mme.MME, conn mme.S1APWriter, mmeID *s1ap.MMEUES1APID, enbID *s1ap.ENBUES1APID, cause s1ap.Cause) {
	c := cause
	emitErrorIndication(m, context.Background(), conn, &s1ap.ErrorIndication{MMEUES1APID: mmeID, ENBUES1APID: enbID, Cause: &c})
}

// handleParseError reports a failed decode of an eNB-initiated initiating
// message with an ERROR INDICATION. It must not be used in reply to an ERROR
// INDICATION, to avoid a loop.
//
// An abstract syntax error carries the cause and the per-IE diagnostics the
// rejection must report (TS 36.413 §10.3.5); where the message is UE
// associated, the UE S1AP IDs that did decode address it (§8.7.2.2).
func handleParseError(m *mme.MME, conn mme.S1APWriter, proc s1ap.ProcedureCode, err error) {
	logger.MmeLog.Warn("failed to decode S1AP message",
		zap.Int("procedure-code", int(proc)),
		zap.Error(err))

	sendParseErrorIndication(m, context.Background(), conn, proc, err)
}

// sendParseErrorIndication reports a failed decode with an ERROR INDICATION.
func sendParseErrorIndication(m *mme.MME, ctx context.Context, conn mme.S1APWriter, proc s1ap.ProcedureCode, err error) {
	trigger := s1ap.TriggeringInitiatingMessage
	crit := s1ap.CriticalityReject

	ase, ok := errors.AsType[*s1ap.AbstractSyntaxError](err)
	if !ok {
		emitErrorIndication(m, ctx, conn, &s1ap.ErrorIndication{
			Cause: &s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: s1ap.CauseProtocolTransferSyntaxError},
			CriticalityDiagnostics: &s1ap.CriticalityDiagnostics{
				ProcedureCode:        &proc,
				TriggeringMessage:    &trigger,
				ProcedureCriticality: &crit,
			},
		})

		return
	}

	diag := ase.ErrorIndicationDiagnostics()
	mmeID, enbID := ase.UEIDs()

	emitErrorIndication(m, ctx, conn, &s1ap.ErrorIndication{
		MMEUES1APID:            mmeID,
		ENBUES1APID:            enbID,
		Cause:                  &ase.Cause,
		CriticalityDiagnostics: &diag,
	})
}

// reportDiagnostics tells the sender about abstract syntax errors the message
// survived. TS 36.413 §10.3.4.2 requires reporting a not-comprehended IE
// marked notify; ignore-criticality entries are carried silently and
// §9.2.1.21 forbids naming them.
func reportDiagnostics(m *mme.MME, ctx context.Context, conn mme.S1APWriter, proc s1ap.ProcedureCode,
	trigger s1ap.TriggeringMessage, ids ueIDs, diag s1ap.Diagnostics,
) {
	if !diag.ReportRequired() {
		return
	}

	crit := s1ap.ProcedureCriticality(proc)

	emitErrorIndication(m, ctx, conn, &s1ap.ErrorIndication{
		MMEUES1APID: ids.mme,
		ENBUES1APID: ids.enb,
		Cause:       &s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: s1ap.CauseProtocolAbstractSyntaxErrorIgnoreAndNotify},
		CriticalityDiagnostics: &s1ap.CriticalityDiagnostics{
			ProcedureCode:             &proc,
			TriggeringMessage:         &trigger,
			ProcedureCriticality:      &crit,
			IEsCriticalityDiagnostics: diag.Report(),
		},
	})
}

// ueIDs names the association a report concerns (TS 36.413 §8.7.2.2); both
// fields are nil for node-level signalling.
type ueIDs struct {
	mme *s1ap.MMEUES1APID
	enb *s1ap.ENBUES1APID
}

// nodeLevel is the empty association, for procedures with no UE context.
func nodeLevel() ueIDs { return ueIDs{} }

func ueAssociated(mmeID s1ap.MMEUES1APID, enbID s1ap.ENBUES1APID) ueIDs {
	return ueIDs{mme: &mmeID, enb: &enbID}
}

// rejectedUEIDs returns the UE S1AP IDs recovered from a rejected message, so
// a UE-associated unsuccessful outcome can name the association it concerns.
func rejectedUEIDs(err error) (*s1ap.MMEUES1APID, *s1ap.ENBUES1APID) {
	ase, ok := errors.AsType[*s1ap.AbstractSyntaxError](err)
	if !ok {
		return nil, nil
	}

	return ase.UEIDs()
}

// rejectWithFailure answers an undecodable initiating message. TS 36.413
// §10.3.4.2, §10.3.5 and §10.3.6 reject using the procedure's own unsuccessful
// outcome, and each says that where the received information is insufficient to
// build it the receiver "shall instead terminate the procedure and initiate the
// Error Indication procedure".
//
// build may be nil for a procedure that defines no unsuccessful outcome.
func rejectWithFailure(m *mme.MME, ctx context.Context, conn mme.S1APWriter, proc s1ap.ProcedureCode, err error,
	build func(cause s1ap.Cause, diag *s1ap.CriticalityDiagnostics) ([]byte, error),
	msgType mme.S1APProcedure,
) {
	log := logger.From(ctx, logger.MmeLog)
	log.Warn("failed to decode S1AP message", zap.Int("procedure-code", int(proc)), zap.Error(err))

	cause := s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: s1ap.CauseProtocolTransferSyntaxError}

	var diag *s1ap.CriticalityDiagnostics

	ase, isAbstract := errors.AsType[*s1ap.AbstractSyntaxError](err)
	if isAbstract {
		d := ase.OutcomeDiagnostics()
		cause, diag = ase.Cause, &d
	}

	if build != nil && isAbstract && ase.HasUnsuccessfulOutcome() {
		out, buildErr := build(cause, diag)
		if buildErr == nil {
			m.SendToRadio(ctx, conn, msgType, out)

			return
		}

		log.Warn("cannot build the unsuccessful outcome; reporting by Error Indication", zap.Error(buildErr))
	}

	sendParseErrorIndication(m, ctx, conn, proc, err)
}

// sendProtocolErrorIndication answers a PDU the MME could not decode with a cause-only
// ERROR INDICATION (TS 36.413 §10.2). It carries no Criticality Diagnostics because a
// transfer-syntax error decodes nothing to cite; it applies where a decode failed
// outright.
func sendProtocolErrorIndication(m *mme.MME, conn mme.S1APWriter, cause int) {
	emitErrorIndication(m, context.Background(), conn, &s1ap.ErrorIndication{
		Cause: &s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: cause},
	})
}

// respondToUnknownProcedure answers an initiating message whose Procedure Code the MME
// does not comprehend, keyed on the received criticality (TS 36.413 §10.3.4.1): Reject
// or Ignore-and-Notify draw an ERROR INDICATION carrying Criticality Diagnostics
// (Procedure Code, Triggering Message, Procedure Criticality); Ignore is dropped
// silently, as most procedures an eNB sends that the MME does not handle are.
func respondToUnknownProcedure(m *mme.MME, conn mme.S1APWriter, im *s1ap.InitiatingMessage) {
	var cause int

	switch im.Criticality {
	case s1ap.CriticalityReject:
		cause = s1ap.CauseProtocolAbstractSyntaxErrorReject
	case s1ap.CriticalityNotify:
		cause = s1ap.CauseProtocolAbstractSyntaxErrorIgnoreAndNotify
	default:
		return
	}

	proc := im.ProcedureCode
	trigger := s1ap.TriggeringInitiatingMessage
	crit := im.Criticality

	emitErrorIndication(m, context.Background(), conn, &s1ap.ErrorIndication{
		Cause: &s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: cause},
		CriticalityDiagnostics: &s1ap.CriticalityDiagnostics{
			ProcedureCode:        &proc,
			TriggeringMessage:    &trigger,
			ProcedureCriticality: &crit,
		},
	})
}

// emitErrorIndication sends an ERROR INDICATION. Callers outside a request
// span pass a fresh root.
func emitErrorIndication(m *mme.MME, ctx context.Context, conn mme.S1APWriter, ind *s1ap.ErrorIndication) {
	b, err := ind.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal Error Indication", zap.Error(err))

		return
	}

	m.SendToRadio(ctx, conn, mme.S1APProcedureErrorIndication, b)
}

// handleErrorIndication processes an ERROR INDICATION from the eNB (TS 36.413). A
// protocol error on a UE-associated S1 connection leaves it in an inconsistent
// state, so if the indication names a known UE the MME releases it to ECM-IDLE.
func handleErrorIndication(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseErrorIndication(value)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Error Indication", zap.Error(err))
		return
	}

	// TS 36.413 §8.7.2.2 — "shall contain at least either the Cause IE or the
	// Criticality Diagnostics IE" — binds the sender, and s1ap.ErrorIndication
	// holds this MME to it on the way out. On the way in it is only a protocol
	// violation to report: §10.5 asks for local error handling, and an
	// indication naming a UE still says its S1 connection is inconsistent, so
	// dropping it here would strand the UE that the release below exists to
	// clean up.
	if msg.Cause == nil && msg.CriticalityDiagnostics == nil {
		logger.From(ctx, logger.MmeLog).Error("Error Indication carries neither Cause nor Criticality Diagnostics")
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

	logger.From(ctx, logger.MmeLog).Warn("Error Indication", fields...)

	if msg.MMEUES1APID == nil {
		return
	}

	ue, ok := m.LookupUe(*msg.MMEUES1APID)
	if !ok {
		return
	}

	// The MME-UE-S1AP-ID space is shared across eNBs, so releasing on the id
	// alone would let any eNB tear down a UE attached through another. One read,
	// not two: a detach between them leaves the second dereferencing nil.
	if ueConn := ue.Conn(); ueConn == nil || ueConn.Conn() != radio.Conn {
		logger.From(ctx, logger.MmeLog).Warn("Error Indication for an MME-UE-S1AP-ID on a different S1 association",
			zap.Uint32("mme-ue-id", uint32(*msg.MMEUES1APID)))

		return
	}

	m.ReleaseUEContext(ctx, ue, mme.CauseNASUnspecified)
}
