// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// handleDecodeReport returns false so the dispatcher skips the handler on a
// fatal decode error, and true otherwise. On a fatal error it answers an
// initiating message with an ErrorIndication but leaves a response to local
// error handling (TS 38.413 §10.3.4.2, §10.3.5). Non-fatal errors
// (ignore-criticality) are logged without an ErrorIndication.
func handleDecodeReport(ctx context.Context, ran *amf.Radio, report *decode.Report) bool {
	if !report.HasItems() {
		return true
	}

	if report.Fatal() {
		if report.FromInitiatingMessage() {
			cd := report.ToCriticalityDiagnostics()

			if pkt, err := send.BuildErrorIndication(nil, nil, nil, &cd); err != nil {
				logger.WithTrace(ctx, ran.Log).Error("error building error indication", zap.Error(err))
			} else {
				ran.SendToRadio(ctx, send.NGAPProcedureErrorIndication, pkt)
			}
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
