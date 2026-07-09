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
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleNGReset(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.NGReset) {
	logger.WithTrace(ctx, ran.Log).Debug("Received NG Reset with Cause", logger.Cause(causeToString(msg.Cause)))

	switch msg.ResetType.Present {
	case ngapType.ResetTypePresentNGInterface:
		logger.WithTrace(ctx, ran.Log).Debug("ResetType Present: NG Interface")
		// TS 38.413: NG Reset is initiated when one side has lost its
		// UE-associated logical NG-connection context. Treat as lower layer
		// failure so ongoing NAS procedures are aborted per TS 24.501.
		amfInstance.RemoveAllUeInRan(ctx, ran)
		logger.WithTrace(ctx, ran.Log).Debug("All UE Context in RAN have been removed")

		pkt, err := send.BuildNGResetAcknowledge(nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error building NG Reset Acknowledge", zap.Error(err))
			return
		}

		_ = ran.SendToRadio(ctx, send.NGAPProcedureNGResetAcknowledge, pkt)
	case ngapType.ResetTypePresentPartOfNGInterface:
		logger.WithTrace(ctx, ran.Log).Debug("ResetType Present: Part of NG Interface")

		partOfNGInterface := msg.ResetType.PartOfNGInterface
		if partOfNGInterface == nil {
			logger.WithTrace(ctx, ran.Log).Error("PartOfNGInterface is nil")
			return
		}

		var ueConn *amf.UeConn

		for _, ueAssociatedLogicalNGConnectionItem := range partOfNGInterface.List {
			if ueAssociatedLogicalNGConnectionItem.AMFUENGAPID != nil {
				logger.WithTrace(ctx, ran.Log).Debug("NG Reset with AMFUENGAPID", zap.Int64("AmfUeNgapID", ueAssociatedLogicalNGConnectionItem.AMFUENGAPID.Value))
				ueConn = amfInstance.FindUEByAmfUeNgapID(ran, models.AmfUeNgapID(ueAssociatedLogicalNGConnectionItem.AMFUENGAPID.Value))
			} else if ueAssociatedLogicalNGConnectionItem.RANUENGAPID != nil {
				logger.WithTrace(ctx, ran.Log).Debug("NG Reset with RANUENGAPID", zap.Int64("RanUeNgapID", ueAssociatedLogicalNGConnectionItem.RANUENGAPID.Value))
				ueConn = amfInstance.FindUEByRanUeNgapID(ran, models.RanUeNgapID(ueAssociatedLogicalNGConnectionItem.RANUENGAPID.Value))
			}

			if ueConn == nil {
				logger.WithTrace(ctx, ran.Log).Warn("Cannot not find UE Context")

				if ueAssociatedLogicalNGConnectionItem.AMFUENGAPID != nil {
					logger.WithTrace(ctx, ran.Log).Warn("AMFUENGAPID is not empty", zap.Int64("AmfUeNgapID", ueAssociatedLogicalNGConnectionItem.AMFUENGAPID.Value))
				}

				if ueAssociatedLogicalNGConnectionItem.RANUENGAPID != nil {
					logger.WithTrace(ctx, ran.Log).Warn("RANUENGAPID is not empty", zap.Int64("RanUeNgapID", ueAssociatedLogicalNGConnectionItem.RANUENGAPID.Value))
				}

				continue
			}

			err := amfInstance.RemoveUe(ctx, ueConn)
			if err != nil {
				logger.WithTrace(ctx, ueConn.Log).Error(err.Error())
			}
		}

		pkt, err := send.BuildNGResetAcknowledge(partOfNGInterface)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error building NG Reset Acknowledge", zap.Error(err))
			return
		}

		_ = ran.SendToRadio(ctx, send.NGAPProcedureNGResetAcknowledge, pkt)
	default:
		logger.WithTrace(ctx, ran.Log).Warn("Invalid ResetType", zap.Any("ResetType", msg.ResetType.Present))
	}
}
