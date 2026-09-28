// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func HandleUEContextModificationResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.UEContextModificationResponse) {
	ueConn, ok := resolveUEIDs(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcUEContextModification, ngap.TriggeringSuccessfulOutcome, ueAssociated(*msg.AMFUENGAPID, *msg.RANUENGAPID), msg.Diagnostics())

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	ueConn.TouchLastSeen()
}

func HandleUEContextModificationFailure(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.UEContextModificationFailure) {
	ueConn, ok := resolveUEIDs(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcUEContextModification, ngap.TriggeringUnsuccessfulOutcome, ueAssociated(*msg.AMFUENGAPID, *msg.RANUENGAPID), msg.Diagnostics())

	ueConn.TouchLastSeen()
	ueConn.ForgetUEAMBR()

	fields := []zap.Field{}
	if msg.Cause != nil {
		fields = append(fields, logger.Cause(msg.Cause.String()))
	}

	ueConn.Log(ctx).Warn("NG-RAN node refused the UE Context Modification", fields...)
}
