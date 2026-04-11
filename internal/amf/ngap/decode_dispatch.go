// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// handleDecodeReport sends an ErrorIndication for any items in report
// and returns false iff report.Fatal(), in which case the dispatcher
// must skip the handler.
func handleDecodeReport(ctx context.Context, ran *amf.Radio, report *decode.Report) bool {
	if !report.HasItems() {
		return true
	}

	cd := report.ToCriticalityDiagnostics()

	if err := ran.NGAPSender.SendErrorIndication(ctx, nil, &cd); err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
	}

	if report.Fatal() {
		logger.WithTrace(ctx, ran.Log).Error("fatal NGAP decode error",
			zap.Int64("procedureCode", report.ProcedureCode),
			zap.Int("ieErrors", len(report.Items)))

		return false
	}

	logger.WithTrace(ctx, ran.Log).Warn("non-fatal NGAP decode error",
		zap.Int64("procedureCode", report.ProcedureCode),
		zap.Int("ieErrors", len(report.Items)))

	return true
}
