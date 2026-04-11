package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandleNasNonDeliveryIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.NASNonDeliveryIndication) {
	ranUe := ran.FindUEByRanUeNgapID(msg.RANUENGAPID)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("RanUeNgapID", msg.RANUENGAPID))
		return
	}

	logger.WithTrace(ctx, ranUe.Log).Debug("Handle NAS Non Delivery Indication", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID), zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), logger.Cause(causeToString(msg.Cause)))
	ranUe.TouchLastSeen()

	err := nas.HandleNAS(ctx, amfInstance, ranUe, msg.NASPDU)
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error handling NAS", zap.Error(err))
	}
}
