// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// handleDecodeReport returns false so the dispatcher skips the handler on a
// fatal decode error, and true otherwise. On a fatal error it answers an
// initiating message per respondToFatalReport but leaves a response to local
// error handling (TS 38.413 §10.3.4.2, §10.3.5). Non-fatal errors
// (ignore-criticality) are logged without a response.
func handleDecodeReport(ctx context.Context, ran *amf.Radio, report *decode.Report) bool {
	if !report.HasItems() {
		return true
	}

	if report.Fatal() {
		if report.FromInitiatingMessage() {
			respondToFatalReport(ctx, ran, report)
		}

		logger.WithTrace(ctx, ran.Log).Error("fatal NGAP decode error",
			zap.Int64("procedureCode", report.ProcedureCode),
			zap.Int("ieErrors", len(report.Items)))

		return false
	}

	logger.WithTrace(ctx, ran.Log).Warn("non-fatal NGAP decode error, ignoring",
		zap.Int64("procedureCode", report.ProcedureCode),
		zap.Int("ieErrors", len(report.Items)))

	return true
}

// respondToFatalReport answers a fatal decode of an initiating message, reporting
// the offending IEs in Criticality Diagnostics. Every procedure still decoded
// here falls back to the Error Indication of TS 38.413 §10.3.4.2; a procedure
// that defines an unsuccessful outcome rejects with that instead once it moves
// to the in-house codec, as NG Setup does in receive_ng_setup.go.
func respondToFatalReport(ctx context.Context, ran *amf.Radio, report *decode.Report) {
	cd := report.ToCriticalityDiagnostics()

	emitErrorIndication(ctx, ran, &ngap.ErrorIndication{
		Cause:                  &ngap.Cause{Group: ngap.CauseGroupProtocol, Value: ngap.CauseProtocolAbstractSyntaxErrorReject},
		CriticalityDiagnostics: fromReferenceDiagnostics(cd),
	})
}

// sendProtocolErrorIndication answers a PDU the receiver could not decode, or one
// carrying an unknown Procedure Code, with a cause-only Error Indication (TS 38.413
// §10.2). It carries no Criticality Diagnostics because a transfer-syntax error
// decodes nothing to cite; it applies where a decode failed outright.
func sendProtocolErrorIndication(ctx context.Context, ran *amf.Radio, cause int) {
	emitErrorIndication(ctx, ran, &ngap.ErrorIndication{
		Cause: &ngap.Cause{Group: ngap.CauseGroupProtocol, Value: cause},
	})
}

// respondToUnknownProcedure answers an initiating message whose Procedure Code the
// AMF does not comprehend, keyed on the received criticality (TS 38.413 §10.3.4.1):
// Reject or Ignore-and-Notify draw an Error Indication carrying Criticality
// Diagnostics (Procedure Code, Triggering Message, Procedure Criticality); Ignore is
// dropped silently. Most procedures a gNB sends that the AMF does not handle are
// criticality Ignore, so this must not answer them.
func respondToUnknownProcedure(ctx context.Context, ran *amf.Radio, im *ngapType.InitiatingMessage) {
	var cause int

	switch im.Criticality.Value {
	case ngapType.CriticalityPresentReject:
		cause = ngap.CauseProtocolAbstractSyntaxErrorReject
	case ngapType.CriticalityPresentNotify:
		cause = ngap.CauseProtocolAbstractSyntaxErrorIgnoreAndNotify
	default:
		return
	}

	proc := ngap.ProcedureCode(im.ProcedureCode.Value)
	trigger := ngap.TriggeringInitiatingMessage
	crit := ngap.Criticality(im.Criticality.Value)

	emitErrorIndication(ctx, ran, &ngap.ErrorIndication{
		Cause: &ngap.Cause{Group: ngap.CauseGroupProtocol, Value: cause},
		CriticalityDiagnostics: &ngap.CriticalityDiagnostics{
			ProcedureCode:        &proc,
			TriggeringMessage:    &trigger,
			ProcedureCriticality: &crit,
		},
	})
}

// fromReferenceDiagnostics converts the Criticality Diagnostics the reference
// decoder's reports still build. The two enumerations are numerically
// identical, so this is a re-typing; it goes with the last procedure that
// reports through decode.Report.
func fromReferenceDiagnostics(cd ngapType.CriticalityDiagnostics) *ngap.CriticalityDiagnostics {
	out := &ngap.CriticalityDiagnostics{}

	if cd.ProcedureCode != nil {
		out.ProcedureCode = ngap.Ptr(ngap.ProcedureCode(cd.ProcedureCode.Value))
	}

	if cd.TriggeringMessage != nil {
		out.TriggeringMessage = ngap.Ptr(ngap.TriggeringMessage(cd.TriggeringMessage.Value))
	}

	if cd.ProcedureCriticality != nil {
		out.ProcedureCriticality = ngap.Ptr(ngap.Criticality(cd.ProcedureCriticality.Value))
	}

	if cd.IEsCriticalityDiagnostics == nil {
		return out
	}

	for _, item := range cd.IEsCriticalityDiagnostics.List {
		out.IEsCriticalityDiagnostics = append(out.IEsCriticalityDiagnostics, ngap.CriticalityDiagnosticsIEItem{
			IECriticality: ngap.Criticality(item.IECriticality.Value),
			IEID:          ngap.ProtocolIEID(item.IEID.Value),
			TypeOfError:   ngap.TypeOfError(item.TypeOfError.Value),
		})
	}

	return out
}
