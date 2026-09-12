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

// causeReleaseUnspecified stands in for an omitted Cause IE (TS 38.413 §9.3.1.2,
// CauseRadioNetwork "unspecified").
var causeReleaseUnspecified = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified}

func keepsConnectionForPendingDownlink(cause ngap.Cause, ueConn *amf.UeConn) bool {
	if cause.Group != ngap.CauseGroupRadioNetwork || cause.Value != ngap.CauseRadioNetworkUserInactivity {
		return false
	}

	return ueConn.MTSignallingPending()
}

// HandleUEContextReleaseRequest handles an NG-RAN-initiated UE Context Release
// Request (inactivity or radio-link failure), starting the release procedure
// (TS 38.413 §8.3.2).
func HandleUEContextReleaseRequest(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.UEContextReleaseRequest) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcUEContextReleaseRequest, ngap.TriggeringInitiatingMessage, ueAssociated(msg.AMFUENGAPID, msg.RANUENGAPID), msg.Diagnostics())

	logger.WithTrace(ctx, ueConn.Log()).Debug("Handle UE Context Release Request")

	// An omitted Cause is an ignore-criticality absence: the NG-RAN node has
	// dropped the radio connection either way, so the release proceeds under a
	// generic cause (§10.3.5).
	cause := causeReleaseUnspecified

	if msg.Cause != nil {
		cause = *msg.Cause

		fields := []zap.Field{logger.Cause(cause.String())}
		if ueConn.UeContext() != nil {
			fields = append(fields, logger.SUPI(ueConn.UeContext().Supi().String()))
		}

		logger.WithTrace(ctx, ueConn.Log()).Info("UE Context Release Cause", fields...)
	}

	if keepsConnectionForPendingDownlink(cause, ueConn) {
		ueConn.DeferRelease(cause)

		logger.WithTrace(ctx, ueConn.Log()).Info("keeping the NG connection: user inactivity reported while downlink traffic or signalling is pending")

		return
	}

	amfInstance.ReleaseOnRANRequest(ctx, ueConn, cause, reportedSessions(msg))
}

func reportedSessions(msg *ngap.UEContextReleaseRequest) []uint8 {
	if msg.PDUSessionResourceList == nil {
		return nil
	}

	ids := make([]uint8, 0, len(msg.PDUSessionResourceList))
	for _, item := range msg.PDUSessionResourceList {
		ids = append(ids, uint8(item.PDUSessionID))
	}

	return ids
}
