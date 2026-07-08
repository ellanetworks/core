// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.RANUENGAPID, msg.AMFUENGAPID)
	if !ok {
		return
	}

	ueConn.TouchLastSeen()
	logger.WithTrace(ctx, ueConn.Log).Debug("Handle UE Context Modification Response", zap.Int64("AmfUeNgapID", ueConn.AmfUeNgapID), zap.Int64("RanUeNgapID", ueConn.RanUeNgapID))

	if msg.RRCState != nil {
		switch msg.RRCState.Value {
		case ngapType.RRCStatePresentInactive:
			logger.WithTrace(ctx, ueConn.Log).Debug("UE RRC State: Inactive")
		case ngapType.RRCStatePresentConnected:
			logger.WithTrace(ctx, ueConn.Log).Debug("UE RRC State: Connected")
		}
	}

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
	}
}
