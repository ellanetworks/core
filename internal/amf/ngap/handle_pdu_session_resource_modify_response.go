package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceModifyResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.PDUSessionResourceModifyResponse) {
	ranUe, ok := resolveUE(ctx, ran, msg.RANUENGAPID, msg.AMFUENGAPID)
	if !ok {
		return
	}

	ranUe.TouchLastSeen()
	logger.WithTrace(ctx, ranUe.Log).Debug("Handle PDUSessionResourceModifyResponse", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))

	if msg.UserLocationInformation != nil {
		ranUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
	}
}
