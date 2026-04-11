package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandleInitialContextSetupResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.InitialContextSetupResponse) {
	ranUe := ran.FindUEByRanUeNgapID(msg.RANUENGAPID)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("RanUeNgapID", msg.RANUENGAPID), zap.Int64("AmfUeNgapID", msg.AMFUENGAPID))
		return
	}

	ranUe.TouchLastSeen()

	amfUe := ranUe.AmfUe()
	if amfUe == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("amfUe is nil")
		return
	}

	if len(msg.SetupItems) > 0 {
		logger.WithTrace(ctx, ranUe.Log).Debug("Send PDUSessionResourceSetupResponseTransfer to SMF")

		for _, item := range msg.SetupItems {
			pduSessionID, ok := validPDUSessionID(item.PDUSessionID.Value)
			if !ok {
				logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
				continue
			}

			transfer := item.PDUSessionResourceSetupResponseTransfer

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				continue
			}

			err := amfInstance.Smf.UpdateSmContextN2InfoPduResSetupRsp(ctx, smContext.Ref, transfer)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Error("SendUpdateSmContextN2Info[PDUSessionResourceSetupResponseTransfer] Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			}
		}
	}

	if len(msg.FailedToSetupItems) > 0 {
		logger.WithTrace(ctx, ranUe.Log).Debug("Send PDUSessionResourceSetupUnsuccessfulTransfer to SMF")

		for _, item := range msg.FailedToSetupItems {
			pduSessionID, ok := validPDUSessionID(item.PDUSessionID.Value)
			if !ok {
				logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
				continue
			}

			transfer := item.PDUSessionResourceSetupUnsuccessfulTransfer

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				continue
			}

			err := amfInstance.Smf.UpdateSmContextN2InfoPduResSetupFail(ctx, smContext.Ref, transfer)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Error("SendUpdateSmContextN2Info[PDUSessionResourceSetupUnsuccessfulTransfer] Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			}
		}
	}

	ranUe.RecvdInitialContextSetupResponse = true
}
