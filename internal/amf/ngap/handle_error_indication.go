// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// ueIDs names the association a report concerns: §8.7.5.2 requires both UE
// NGAP IDs on an Error Indication triggered by UE-associated signalling. Both
// fields are nil for node-level signalling.
type ueIDs struct {
	amf *ngap.AMFUENGAPID
	ran *ngap.RANUENGAPID
}

func ueAssociated(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID) ueIDs {
	return ueIDs{amf: &amfID, ran: &ranID}
}

// reportDiagnostics tells the sender about abstract syntax errors the message
// survived. TS 38.413 §10.3.4.2 requires reporting a not-comprehended IE
// marked notify; ignore-criticality entries are carried silently and
// §9.3.1.3 forbids naming them.
func reportDiagnostics(ctx context.Context, ran *amf.Radio, proc ngap.ProcedureCode,
	trigger ngap.TriggeringMessage, ids ueIDs, diag ngap.Diagnostics,
) {
	if !diag.ReportRequired() {
		return
	}

	crit := ngap.ProcedureCriticality(proc)

	emitErrorIndication(ctx, ran, &ngap.ErrorIndication{
		AMFUENGAPID: ids.amf,
		RANUENGAPID: ids.ran,
		Cause:       &ngap.Cause{Group: ngap.CauseGroupProtocol, Value: ngap.CauseProtocolAbstractSyntaxErrorIgnoreAndNotify},
		CriticalityDiagnostics: &ngap.CriticalityDiagnostics{
			ProcedureCode:             &proc,
			TriggeringMessage:         &trigger,
			ProcedureCriticality:      &crit,
			IEsCriticalityDiagnostics: diag.Report(),
		},
	})
}

// emitErrorIndication marshals and sends an ERROR INDICATION.
func emitErrorIndication(ctx context.Context, ran *amf.Radio, ind *ngap.ErrorIndication) {
	b, err := ind.Marshal()
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("failed to marshal Error Indication", zap.Error(err))

		return
	}

	ran.SendToRadio(ctx, send.NGAPProcedureErrorIndication, b)
}

// sendErrorIndication answers a UE-associated message the AMF cannot act on,
// naming the association it concerns (TS 38.413 §8.7.5.2).
func sendErrorIndication(ctx context.Context, ran *amf.Radio, amfID *ngap.AMFUENGAPID, ranID *ngap.RANUENGAPID, cause ngap.Cause) {
	c := cause
	emitErrorIndication(ctx, ran, &ngap.ErrorIndication{AMFUENGAPID: amfID, RANUENGAPID: ranID, Cause: &c})
}

// sendParseErrorIndication reports a failed decode with an ERROR INDICATION.
//
// An abstract syntax error carries the cause and the per-IE diagnostics the
// rejection must report (TS 38.413 §10.3.5); where the message is UE
// associated, the UE NGAP IDs that did decode address it (§8.7.5.2). Octets
// that did not decode at all leave nothing to cite beyond the procedure
// (§10.2).
func sendParseErrorIndication(ctx context.Context, ran *amf.Radio, proc ngap.ProcedureCode, err error) {
	trigger := ngap.TriggeringInitiatingMessage

	var ase *ngap.AbstractSyntaxError
	if !errors.As(err, &ase) {
		crit := ngap.ProcedureCriticality(proc)

		emitErrorIndication(ctx, ran, &ngap.ErrorIndication{
			Cause: &ngap.Cause{Group: ngap.CauseGroupProtocol, Value: ngap.CauseProtocolTransferSyntaxError},
			CriticalityDiagnostics: &ngap.CriticalityDiagnostics{
				ProcedureCode:        &proc,
				TriggeringMessage:    &trigger,
				ProcedureCriticality: &crit,
			},
		})

		return
	}

	diag := ase.ErrorIndicationDiagnostics()
	amfID, ranID := ase.UEIDs()

	emitErrorIndication(ctx, ran, &ngap.ErrorIndication{
		AMFUENGAPID:            amfID,
		RANUENGAPID:            ranID,
		Cause:                  &ase.Cause,
		CriticalityDiagnostics: &diag,
	})
}

// handleErrorIndication processes an ERROR INDICATION from the gNB
// (TS 38.413 §8.7.5). A protocol error on a UE-associated NG connection leaves
// it in an inconsistent state, so if the indication names a known UE the AMF
// releases it to CM-IDLE, where it re-establishes cleanly on its next Service
// Request.
func HandleErrorIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.ErrorIndication) {
	// TS 38.413 §8.7.5.2 — "shall contain at least either the Cause IE or the
	// Criticality Diagnostics IE" — binds the sender, and ngap.ErrorIndication
	// holds this AMF to it on the way out. On the way in it is only a protocol
	// violation to report: §10.5 asks for local error handling, and an
	// indication naming a UE still says its NG connection is inconsistent, so
	// dropping it here would strand the UE that the release below exists to
	// clean up.
	if msg.Cause == nil && msg.CriticalityDiagnostics == nil {
		logger.WithTrace(ctx, ran.Log).Error("Error Indication carries neither Cause nor Criticality Diagnostics")
	}

	fields := make([]zap.Field, 0, 3)
	if msg.AMFUENGAPID != nil {
		fields = append(fields, zap.Uint64("amf-ue-id", uint64(*msg.AMFUENGAPID)))
	}

	if msg.RANUENGAPID != nil {
		fields = append(fields, zap.Uint32("ran-ue-id", uint32(*msg.RANUENGAPID)))
	}

	if msg.Cause != nil {
		fields = append(fields, zap.String("cause", msg.Cause.String()))
	}

	logger.WithTrace(ctx, ran.Log).Warn("Error Indication", fields...)

	if msg.AMFUENGAPID == nil {
		return
	}

	// The lookup is scoped to the sending radio: the AMF UE NGAP ID space is
	// shared across gNBs, so resolving on the id alone would let any gNB tear
	// down a UE attached through another.
	ueConn := amfInstance.FindUEByAmfUeNgapID(ran, models.AmfUeNgapID(*msg.AMFUENGAPID))
	if ueConn == nil {
		return
	}

	ueConn.ReleaseAction = amf.UeContextN2NormalRelease

	// The release command still takes the reference decoder's cause constants;
	// it moves when UE Context Release migrates.
	ueConn.SendUEContextReleaseCommand(ctx, ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified})
}
