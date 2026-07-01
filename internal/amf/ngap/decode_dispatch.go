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

// handleDecodeReport sends ErrorIndication for fatal decode errors and
// returns false so the dispatcher skips the handler. Non-fatal errors
// (ignore-criticality) are logged without sending ErrorIndication.
func handleDecodeReport(ctx context.Context, ran *amf.Radio, report *decode.Report) bool {
	if !report.HasItems() {
		return true
	}

	if report.Fatal() {
		cd := report.ToCriticalityDiagnostics()

		if pkt, err := send.BuildErrorIndication(nil, nil, nil, &cd); err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error building error indication", zap.Error(err))
		} else if err := ran.SendToRan(ctx, send.NGAPProcedureErrorIndication, pkt); err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
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
