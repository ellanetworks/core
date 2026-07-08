// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
)

func HandleUEContextReleaseComplete(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.UEContextReleaseComplete) {
	if msg.AMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMFUENGAPID IE (mandatory) is missing in UEContextReleaseComplete")
		return
	}

	if msg.RANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID IE (mandatory) is missing in UEContextReleaseComplete")
		return
	}

	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.RANUENGAPID, msg.AMFUENGAPID)
	if !ok {
		return
	}

	// The Complete arrived; cancel the release-supervision guard so it does not also
	// run the cleanup.
	ueConn.StopReleaseGuard()

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
	}

	ueConn.TouchLastSeen()

	amfInstance.ReleaseUeConn(ctx, ueConn)
}
