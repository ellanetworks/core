// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceNotify(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.PDUSessionResourceNotify) {
	ranUe, ok := resolveUE(ctx, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	ranUe.TouchLastSeen()
	logger.WithTrace(ctx, ranUe.Log).Debug("Handle PDUSessionResourceNotify", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))

	amfUe := ranUe.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("amfUe is nil")
		return
	}

	if msg.UserLocationInformation != nil {
		ranUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
	}

	if msg.HasNotifyList {
		// QoS flow-level notifications — forwarding to SMF is not yet implemented.
		logger.WithTrace(ctx, ranUe.Log).Warn("PDUSessionResourceNotifyList received but QoS flow notification forwarding is not implemented")
	}

	for _, item := range msg.PDUSessionResourceReleasedItems {
		pduSessionID, ok := validPDUSessionID(item.PDUSessionID.Value)
		if !ok {
			logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
			continue
		}

		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
			continue
		}

		err := amfInstance.Smf.DeactivateSmContext(ctx, smContext.Ref)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("DeactivateSmContext failed", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			continue
		}

		amfUe.SetSmContextInactive(pduSessionID)

		logger.WithTrace(ctx, ranUe.Log).Info("deactivated PDU session released by gNB", zap.Uint8("PduSessionID", pduSessionID))
	}
}
