// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// HandleNGReset releases the UE contexts an NG RESET names and answers with NG
// RESET ACKNOWLEDGE, which the gNB needs before it can reuse the released
// UE-NGAP-IDs (TS 38.413 §8.7.4). A whole-interface reset clears every UE on
// the association, a part-of-interface reset only the listed ones. The SCTP
// association stays up.
func HandleNGReset(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, req *ngap.NGReset) {
	cause := "(none)"
	if req.Cause != nil {
		cause = req.Cause.String()
	}

	if req.ResetType.All {
		// TS 38.413: NG Reset is initiated when one side has lost its
		// UE-associated logical NG-connection context. Treat as lower layer
		// failure so ongoing NAS procedures are aborted per TS 24.501.
		amfInstance.RemoveAllUeInRan(ctx, ran)

		logger.WithTrace(ctx, ran.Log).Info("NG Reset (whole interface)", zap.String("cause", cause))
		sendNGResetAcknowledge(ctx, ran, nil, req.Diagnostics())

		return
	}

	reset := releaseListedUEs(ctx, amfInstance, ran, req.ResetType.Part)

	logger.WithTrace(ctx, ran.Log).Info("NG Reset (part of interface)",
		zap.String("cause", cause),
		zap.Int("requested", len(req.ResetType.Part)),
		zap.Int("connections", len(reset)))

	// TS 38.413 §8.7.4.2.1: the acknowledge echoes the UE-associated logical
	// NG-connections that were reset.
	sendNGResetAcknowledge(ctx, ran, reset, req.Diagnostics())
}

// releaseListedUEs releases each UE the list names and returns the entries that
// matched, which is what the acknowledge echoes. An entry naming no UE this AMF
// holds is skipped: the gNB may be resetting a connection the AMF already lost.
func releaseListedUEs(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, list ngap.UEAssociatedLogicalNGConnectionList) ngap.UEAssociatedLogicalNGConnectionList {
	reset := make(ngap.UEAssociatedLogicalNGConnectionList, 0, len(list))

	for _, item := range list {
		ueConn := findUEForConnectionItem(amfInstance, ran, item)
		if ueConn == nil {
			logger.WithTrace(ctx, ran.Log).Warn("NG Reset names a UE this AMF does not hold",
				zap.Any("amf-ue-id", item.AMFUENGAPID), zap.Any("ran-ue-id", item.RANUENGAPID))

			continue
		}

		if err := amfInstance.RemoveUe(ctx, ueConn); err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("failed to remove UE named by NG Reset", zap.Error(err))

			continue
		}

		reset = append(reset, item)
	}

	return reset
}

// findUEForConnectionItem resolves one UE-associated logical NG-connection.
// Either identity may be absent (TS 38.413 §9.3.1.17); the AMF id is preferred
// because it is the one this AMF allocated.
func findUEForConnectionItem(amfInstance *amf.AMF, ran *amf.Radio, item ngap.UEAssociatedLogicalNGConnectionItem) *amf.UeConn {
	if item.AMFUENGAPID != nil {
		return amfInstance.FindUEByAmfUeNgapID(ran, models.AmfUeNgapID(*item.AMFUENGAPID))
	}

	if item.RANUENGAPID != nil {
		return amfInstance.FindUEByRanUeNgapID(ran, models.RanUeNgapID(*item.RANUENGAPID))
	}

	return nil
}

// sendNGResetAcknowledge answers an NG RESET with NG RESET ACKNOWLEDGE
// (TS 38.413 §9.2.6.12). connectionList is nil only for a whole-interface reset.
func sendNGResetAcknowledge(ctx context.Context, ran *amf.Radio, connectionList ngap.UEAssociatedLogicalNGConnectionList, diag ngap.Diagnostics) {
	ack := &ngap.NGResetAcknowledge{ConnectionList: connectionList}

	// §10.3.4.2 reports in the response message of the procedure where it has one.
	if diag.ReportRequired() {
		ack.CriticalityDiagnostics = &ngap.CriticalityDiagnostics{
			ProcedureCriticality:      ngap.Ptr(ngap.ProcedureCriticality(ngap.ProcNGReset)),
			IEsCriticalityDiagnostics: diag.Report(),
		}
	}

	b, err := ack.Marshal()
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("failed to marshal NG Reset Acknowledge", zap.Error(err))
		return
	}

	ran.SendToRadio(ctx, send.NGAPProcedureNGResetAcknowledge, b)
}
