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

func HandleUplinkNasTransport(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.UplinkNASTransport) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	ueConn.TouchLastSeen()

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		err := amfInstance.RemoveUeConn(ctx, ueConn)
		if err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("error removing ran ue context", zap.Error(err))
		}

		logger.WithTrace(ctx, ueConn.Log).Error("No UE Context of UeConn", zap.Int64("ranUeNgapID", msg.RANUENGAPID), zap.Int64("amfUeNgapID", msg.AMFUENGAPID))

		return
	}

	if msg.UserLocationInformation.Kind() != decode.UserLocationKindUnknown {
		ueConn.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation.Raw())
	}

	if amfInstance.NAS == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("NAS handler not set")
		return
	}

	amfInstance.NAS.HandleNAS(ctx, ueConn, msg.NASPDU)
}
