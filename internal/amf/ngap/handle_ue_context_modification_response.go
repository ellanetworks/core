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
	ranUe, ok := resolveUE(ctx, ran, msg.RANUENGAPID, msg.AMFUENGAPID)
	if !ok {
		return
	}

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
