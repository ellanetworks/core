// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandleNasNonDeliveryIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.NASNonDeliveryIndication) {
	ranUe, ok := resolveUE(ctx, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	logger.WithTrace(ctx, ranUe.Log).Debug("Handle NAS Non Delivery Indication", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID), zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), logger.Cause(causeToString(msg.Cause)))
	ranUe.TouchLastSeen()

	if amfInstance.NAS == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("NAS handler not set")
		return
	}

	err := amfInstance.NAS.HandleNAS(ctx, ranUe, msg.NASPDU)
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error handling NAS", zap.Error(err))
	}
}
