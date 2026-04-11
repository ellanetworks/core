package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceModifyResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.PDUSessionResourceModifyResponse) {
	var ranUe *amf.RanUe

	if msg.RANUENGAPID != nil {
		ranUe = ran.FindUEByRanUeNgapID(*msg.RANUENGAPID)
		if ranUe == nil {
			logger.WithTrace(ctx, ran.Log).Warn("No UE Context", zap.Int64("RanUeNgapID", *msg.RANUENGAPID))
		}
	}

	if msg.AMFUENGAPID != nil {
		ranUe = amfInstance.FindRanUeByAmfUeNgapID(*msg.AMFUENGAPID)
		if ranUe == nil {
			logger.WithTrace(ctx, ran.Log).Warn("No UE Context", zap.Int64("AmfUeNgapID", *msg.AMFUENGAPID))
			return
		}
	}

	if ranUe == nil {
		return
	}

	ranUe.Radio = ran
	ranUe.TouchLastSeen()
	logger.WithTrace(ctx, ranUe.Log).Debug("Handle PDUSessionResourceModifyResponse", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))

	if msg.UserLocationInformation != nil {
		ranUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
	}
}
