// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/aper"
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

// respondToFatalReport answers a fatal decode of an initiating message,
// reporting the offending IEs in Criticality Diagnostics. A procedure that
// defines an unsuccessful-outcome message is rejected with that message, which
// TS 38.413 §10.3.5 requires in preference to the Error Indication the
// remaining procedures fall back to (§10.3.4.2).
func respondToFatalReport(ctx context.Context, ran *amf.Radio, report *decode.Report) {
	cd := report.ToCriticalityDiagnostics()

	if report.ProcedureCode == ngapType.ProcedureCodeNGSetup {
		sendNGSetupFailure(ctx, ran, &ngapType.Cause{
			Present: ngapType.CausePresentProtocol,
			Protocol: &ngapType.CauseProtocol{
				Value: ngapType.CauseProtocolPresentAbstractSyntaxErrorReject,
			},
		}, &cd)

		return
	}

	pkt, err := send.BuildErrorIndication(nil, nil, nil, &cd)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error building error indication", zap.Error(err))
		return
	}

	ran.SendToRadio(ctx, send.NGAPProcedureErrorIndication, pkt)
}

// sendProtocolErrorIndication answers a PDU the receiver could not decode, or one
// carrying an unknown Procedure Code, with a cause-only Error Indication (TS 38.413
// §10.2, §10.3.4.1). It needs nothing from the offending PDU, so it applies where a
// decode failed outright.
func sendProtocolErrorIndication(ctx context.Context, ran *amf.Radio, cause aper.Enumerated) {
	pkt, err := send.BuildErrorIndication(nil, nil, &ngapType.Cause{
		Present:  ngapType.CausePresentProtocol,
		Protocol: &ngapType.CauseProtocol{Value: cause},
	}, nil)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error building error indication", zap.Error(err))
		return
	}

	ran.SendToRadio(ctx, send.NGAPProcedureErrorIndication, pkt)
}
