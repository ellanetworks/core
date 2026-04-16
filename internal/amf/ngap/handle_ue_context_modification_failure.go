package ngap

import (
	gocontext "context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandleUEContextModificationFailure(ctx gocontext.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.UEContextModificationFailure) {
	ranUe, ok := resolveUE(ctx, ran, msg.RANUENGAPID, msg.AMFUENGAPID)
	if ok {
		ranUe.TouchLastSeen()
		logger.WithTrace(ctx, ranUe.Log).Debug("Handle UE Context Modification Failure", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))
	}

	if msg.Cause != nil {
		logger.WithTrace(ctx, ran.Log).Debug("UE Context Modification Failure Cause", logger.Cause(causeToString(*msg.Cause)))
	}
}
