package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleUEContextModificationResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.UEContextModificationResponse) {
	var ranUe *amf.RanUe

	if msg.RANUENGAPID != nil {
		ranUe = ran.FindUEByRanUeNgapID(*msg.RANUENGAPID)
		if ranUe == nil {
			if msg.AMFUENGAPID != nil {
				logger.WithTrace(ctx, ran.Log).Warn("No UE Context", zap.Int64("RanUeNgapID", *msg.RANUENGAPID), zap.Int64("AmfUeNgapID", *msg.AMFUENGAPID))
			} else {
				logger.WithTrace(ctx, ran.Log).Warn("No UE Context", zap.Int64("RanUeNgapID", *msg.RANUENGAPID))
			}
		}
	}

	if msg.AMFUENGAPID != nil {
		ranUe = amfInstance.FindRanUeByAmfUeNgapID(*msg.AMFUENGAPID)
		if ranUe == nil {
			logger.WithTrace(ctx, ran.Log).Warn("UE Context not found", zap.Int64("AmfUeNgapID", *msg.AMFUENGAPID))
			return
		}
	}

	if ranUe != nil {
		ranUe.Radio = ran
		ranUe.TouchLastSeen()
		logger.WithTrace(ctx, ranUe.Log).Debug("Handle UE Context Modification Response", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

		if msg.RRCState != nil {
			switch msg.RRCState.Value {
			case ngapType.RRCStatePresentInactive:
				logger.WithTrace(ctx, ranUe.Log).Debug("UE RRC State: Inactive")
			case ngapType.RRCStatePresentConnected:
				logger.WithTrace(ctx, ranUe.Log).Debug("UE RRC State: Connected")
			}
		}

		if msg.UserLocationInformation != nil {
			ranUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
		}
	}
}
