// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceReleaseResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.PDUSessionResourceReleaseResponse) {
	if msg.AMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMFUENGAPID IE (mandatory) is missing in PDUSessionResourceReleaseResponse")
		return
	}

	if msg.RANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID IE (mandatory) is missing in PDUSessionResourceReleaseResponse")
		return
	}

	ueConn := amfInstance.FindUEByRanUeNgapID(ran, models.RanUeNgapID(*msg.RANUENGAPID))
	if ueConn == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("AmfUeNgapID", *msg.AMFUENGAPID), zap.Int64("RanUeNgapID", *msg.RANUENGAPID))
		return
	}

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
	}

	ueConn.TouchLastSeen()

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("amfUe is nil")
		return
	}

	if len(msg.PDUSessionResourceReleasedItems) > 0 {
		logger.WithTrace(ctx, ueConn.Log).Debug("Send PDUSessionResourceReleaseResponseTransfer to SMF")

		for _, item := range msg.PDUSessionResourceReleasedItems {
			pduSessionID, ok := validPDUSessionID(item.PDUSessionID.Value)
			if !ok {
				logger.WithTrace(ctx, ueConn.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
				continue
			}

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				logger.WithTrace(ctx, ueConn.Log).Warn("SmContext not found during release response (may already be removed by SMF)",
					zap.Uint8("PduSessionID", pduSessionID))
			}

			if smContext != nil {
				err := amfInstance.Session.UpdateSmContextN2InfoPduResRelRsp(ctx, smContext.Ref)
				if err != nil {
					logger.WithTrace(ctx, ueConn.Log).Error("SendUpdateSmContextN2InfoPduResRelRsp failed", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
				}
			}

			amfUe.SetSmContextInactive(pduSessionID)
		}
	}
}
