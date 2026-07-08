// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	gocontext "context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleErrorIndication(ctx gocontext.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.ErrorIndication) {
	if msg.Cause == nil && msg.CriticalityDiagnostics == nil {
		logger.WithTrace(ctx, ran.Log).Error("[ErrorIndication] both Cause IE and CriticalityDiagnostics IE are nil, should have at least one")
		return
	}

	if msg.Cause != nil {
		logger.WithTrace(ctx, ran.Log).Debug("Error Indication Cause", logger.Cause(causeToString(*msg.Cause)))
	}

	// A protocol error on a UE-associated NG connection leaves it in an inconsistent
	// state. TS 38.413 §8.7 is silent on the receive action; release a named UE to
	// CM-IDLE so it re-establishes cleanly on its next Service Request.
	if msg.AMFUENGAPID == nil {
		return
	}

	ueConn := amfInstance.FindUEByAmfUeNgapID(ran, *msg.AMFUENGAPID)
	if ueConn == nil {
		return
	}

	ueConn.ReleaseAction = amf.UeContextN2NormalRelease

	if err := ueConn.SendUEContextReleaseCommand(ctx, ngapType.CausePresentRadioNetwork, ngapType.CauseRadioNetworkPresentUnspecified); err != nil {
		logger.WithTrace(ctx, ueConn.Log).Error("failed to release UE on Error Indication", zap.Error(err))
	}
}
