package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func HandleUplinkNasTransport(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.UplinkNASTransport) {
	ranUe, ok := resolveUE(ctx, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	ranUe.TouchLastSeen()

	amfUe := ranUe.AmfUe()
	if amfUe == nil {
		// No AMF UE bound, so no NAS state to clean up. Pass ReleaseNormal
		// — the cause is moot when there is no NAS connection to release.
		err := ranUe.Remove(ctx)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("error removing ran ue context", zap.Error(err))
		}

		logger.WithTrace(ctx, ranUe.Log).Error("No UE Context of RanUe", zap.Int64("ranUeNgapID", msg.RANUENGAPID), zap.Int64("amfUeNgapID", msg.AMFUENGAPID))

		return
	}

	if msg.UserLocationInformation.Kind() != decode.UserLocationKindUnknown {
		ranUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation.Raw())
	}

	if amfInstance.NAS == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("NAS handler not set")
		return
	}

	err := amfInstance.NAS.HandleNAS(ctx, ranUe, msg.NASPDU)
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error handling NAS message", zap.Error(err))
		sendStatus5GMM(ctx, ranUe, nasMessage.Cause5GMMProtocolErrorUnspecified)
	}
}

func sendStatus5GMM(ctx context.Context, ranUe *amf.RanUe, cause uint8) {
	pdu, err := message.BuildStatus5GMM(cause)
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("failed to build 5GMM STATUS", zap.Error(err))
		return
	}

	if err := ranUe.SendDownlinkNasTransport(ctx, pdu, nil); err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("failed to send 5GMM STATUS", zap.Error(err))
	}
}
