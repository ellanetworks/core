// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func HandleInitialUEMessage(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.InitialUEMessage) {
	// A gNB may reuse a RAN-UE-NGAP-ID before its prior UEContextRelease completes; drop
	// any stale conn first so a deferred UEContextReleaseComplete cannot remove the fresh
	// context (TS 38.413).
	amfInstance.DropStaleUe(ctx, ran, models.RanUeNgapID(msg.RANUENGAPID))

	ueConn, err := amfInstance.NewUeConn(ran, models.RanUeNgapID(msg.RANUENGAPID))
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("Failed to add Ran UE to the pool", zap.Error(err))
		return
	}

	logger.WithTrace(ctx, ueConn.Log).Debug("Added Ran UE to the pool", zap.Uint32("ran-ue-id", uint32(ueConn.RanUeNgapID)))

	reportDiagnostics(ctx, ran, ngap.ProcInitialUEMessage, ngap.TriggeringInitiatingMessage,
		ueAssociated(ngap.AMFUENGAPID(ueConn.AmfUeNgapID), msg.RANUENGAPID), msg.Diagnostics())

	ueConn.UpdateLocation(ctx, msg.UserLocationInformation)

	ueConn.UeContextRequest = msg.UEContextRequest != nil

	if amfInstance.NAS == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("NAS handler not set")
		return
	}

	resumeExistingContext(ctx, amfInstance, ueConn, msg)

	amfInstance.NAS.HandleNAS(ctx, ueConn, msg.NASPDU)

	// A NAS message that never established a UE context (undecodable, no usable mobile
	// identity, not a registration request, or a service request the AMF has no context
	// for) leaves a bare RAN connection; release it so an unauthenticated peer cannot
	// exhaust RAN-UE-NGAP-IDs. A message that bound a context is torn down on its own
	// registration path.
	if ueConn.UeContext() == nil {
		if rerr := amfInstance.RemoveUeConn(ctx, ueConn); rerr != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("failed to release bare RAN UE", zap.Error(rerr))
		}
	}
}

// resumeExistingContext optimistically binds an existing, integrity-verified context to a
// fresh connection when an initial NAS message cites a known 5G-S-TMSI in the NGAP IE, so
// the NAS layer need not re-resolve it. It binds nothing when the message cannot be
// authenticated for the cited context (TS 24.501), leaving the NAS layer to resolve or
// register on a fresh context.
func resumeExistingContext(ctx context.Context, amfInstance *amf.AMF, ueConn *amf.UeConn, msg *ngap.InitialUEMessage) {
	if msg.FiveGSTMSI == nil {
		return
	}

	logger.WithTrace(ctx, ueConn.Log).Debug("Receive 5G-S-TMSI")

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		logger.WithTrace(ctx, ueConn.Log).Error("Could not get operator info", zap.Error(err))
		return
	}

	// <5G-S-TMSI> := <AMF Set ID><AMF Pointer><5G-TMSI>
	// GUAMI := <MCC><MNC><AMF Region ID><AMF Set ID><AMF Pointer>
	// 5G-GUTI := <GUAMI><5G-TMSI>
	region, _, _, err := util.AMFIDToNGAP(operatorInfo.Guami.AmfID)
	if err != nil {
		logger.WithTrace(ctx, ueConn.Log).Error("invalid operator AMF id", zap.Error(err))
		return
	}

	amfID := util.AMFIDToModels(region, msg.FiveGSTMSI.AMFSetID, msg.FiveGSTMSI.AMFPointer)

	tmsi, err := etsi.NewTMSI(uint32(msg.FiveGSTMSI.FiveGTMSI))
	if err != nil {
		logger.WithTrace(ctx, ueConn.Log).Warn("invalid tmsi", zap.Error(err))
	}

	guti, err := etsi.NewGUTI5G(operatorInfo.Guami.PlmnID.Mcc, operatorInfo.Guami.PlmnID.Mnc, amfID, tmsi)
	if err != nil {
		logger.WithTrace(ctx, ueConn.Log).Warn("invalid guti", zap.Error(err))
	}

	amfUe, ok := amfInstance.LookupUeByGuti(operatorInfo.Guami, guti)
	if !ok {
		logger.WithTrace(ctx, ueConn.Log).Warn("Unknown UE", logger.GUTI(guti.String()))
		return
	}

	if !amfUe.ReuseForInboundNAS(msg.NASPDU) {
		// The message cites an existing context but is not authenticated for it. Do not
		// bind to or mutate the live context; the NAS layer registers it on a fresh
		// context pending authentication.
		logger.WithTrace(ctx, ueConn.Log).Info("Initial UE Message cites a known GUTI but is not authenticated for that context; registering on a fresh context", logger.GUTI(guti.String()))
		return
	}

	if amfUe.Conn() != nil {
		logger.WithTrace(ctx, ueConn.Log).Debug("Implicit Deregistration", zap.Uint32("ran-ue-id", uint32(ueConn.RanUeNgapID)))
	}

	logger.WithTrace(ctx, ueConn.Log).Debug("UeContext Attach UeConn", zap.Uint32("ran-ue-id", uint32(ueConn.RanUeNgapID)))
	amfInstance.AttachUeConn(amfUe, ueConn)
}
