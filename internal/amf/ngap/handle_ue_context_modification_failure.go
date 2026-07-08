// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	gocontext "context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandleUEContextModificationFailure(ctx gocontext.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.UEContextModificationFailure) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.RANUENGAPID, msg.AMFUENGAPID)
	if ok {
		ueConn.TouchLastSeen()
		logger.WithTrace(ctx, ueConn.Log).Debug("Handle UE Context Modification Failure", zap.Int64("AmfUeNgapID", ueConn.AmfUeNgapID), zap.Int64("RanUeNgapID", ueConn.RanUeNgapID))
	}

	if msg.Cause != nil {
		logger.WithTrace(ctx, ran.Log).Debug("UE Context Modification Failure Cause", logger.Cause(causeToString(*msg.Cause)))
	}
}
