package ngap

import (
	gocontext "context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandleUEContextModificationFailure(ctx gocontext.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.UEContextModificationFailure) {
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
		}
	}

	if ranUe != nil {
		ranUe.Radio = ran
		ranUe.TouchLastSeen()
		logger.WithTrace(ctx, ranUe.Log).Debug("Handle UE Context Modification Failure", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))
	}

	if msg.Cause != nil {
		logger.WithTrace(ctx, ran.Log).Debug("UE Context Modification Failure Cause", logger.Cause(causeToString(*msg.Cause)))
	}
}
