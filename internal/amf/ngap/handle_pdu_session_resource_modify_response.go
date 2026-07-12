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

func HandlePDUSessionResourceModifyResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.PDUSessionResourceModifyResponse) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.RANUENGAPID, msg.AMFUENGAPID)
	if !ok {
		return
	}

	ueConn.TouchLastSeen()
	logger.WithTrace(ctx, ueConn.Log).Debug("Handle PDUSessionResourceModifyResponse", zap.Int64("AmfUeNgapID", int64(ueConn.AmfUeNgapID)))

	ueConn.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
}
