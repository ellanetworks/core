// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"encoding/binary"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapConvert"
	"go.uber.org/zap"
)

func HandleInitialUEMessage(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.InitialUEMessage) {
	ueConn := amfInstance.FindUEByRanUeNgapID(ran, msg.RANUENGAPID)
	if ueConn != nil {
		// gNB reused a RAN UE NGAP ID before completing the previous UEContextRelease.
		// Drop the stale ueConn so a deferred UEContextReleaseComplete carrying the old
		// AMF UE NGAP ID cannot remove the freshly created context.
		logger.WithTrace(ctx, ueConn.Log).Debug("RAN UE NGAP ID reused in InitialUEMessage, removing stale UeConn",
			zap.Int64("RanUeNgapID", ueConn.RanUeNgapID),
			zap.Int64("AmfUeNgapID", ueConn.AmfUeNgapID))

		err := amfInstance.RemoveUeConn(ctx, ueConn)
		if err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error(err.Error())
		}

		ueConn = nil
	}

	if ueConn == nil {
		var err error

		ueConn, err = amfInstance.NewUeConn(ran, msg.RANUENGAPID)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("Failed to add Ran UE to the pool", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ueConn.Log).Debug("Added Ran UE to the pool", zap.Int64("RanUeNgapID", ueConn.RanUeNgapID))
	}

	ueConn.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation.Raw())

	ueConn.UeContextRequest = msg.UEContextRequest

	if amfInstance.NAS == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("NAS handler not set")
		return
	}

	// Route by message class (mirrors the MME's S1AP peek). A SERVICE REQUEST is resolved
	// and answered (accept, or SERVICE REJECT #9 when no context) by its dedicated handler,
	// without the optimistic resume or the mint gate — it never mints a context.
	if amfInstance.NAS.IsServiceRequest(msg.NASPDU) {
		amfInstance.NAS.HandleServiceRequest(ctx, ueConn, msg.NASPDU)
	} else {
		resumeExistingContext(ctx, amfInstance, ueConn, msg)

		if ueConn.UeContext() != nil {
			amfInstance.StopIdleTimers(ueConn.UeContext())
		}

		amfInstance.NAS.HandleNAS(ctx, ueConn, msg.NASPDU)
	}

	// A NAS message that never established a UE context (undecodable, no usable mobile
	// identity, not a registration request, or a service request the AMF has no context
	// for) leaves a bare RAN connection; release it so an unauthenticated peer cannot
	// exhaust RAN-UE-NGAP-IDs (mirrors the MME's ReleaseBareConn). A message that bound a
	// context is torn down on its own registration path.
	if ueConn.UeContext() == nil {
		if rerr := amfInstance.RemoveUeConn(ctx, ueConn); rerr != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("failed to release bare RAN UE", zap.Error(rerr))
		}
	}
}

// resumeExistingContext optimistically binds an existing, integrity-verified context to a
// fresh connection when a non-service-request initial NAS message cites a known 5G-S-TMSI,
// so the NAS layer need not re-resolve it. It binds nothing when the message cannot be
// authenticated for the cited context (TS 24.501), leaving the NAS layer to register on a
// fresh context. Mirrors the MME's S1AP S-TMSI resume.
func resumeExistingContext(ctx context.Context, amfInstance *amf.AMF, ueConn *amf.UeConn, msg decode.InitialUEMessage) {
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
	tmpReginID, _, _ := ngapConvert.AmfIdToNgap(operatorInfo.Guami.AmfID)
	amfID := ngapConvert.AmfIdToModels(tmpReginID, msg.FiveGSTMSI.AMFSetID, msg.FiveGSTMSI.AMFPointer)

	tmsi, err := etsi.NewTMSI(binary.BigEndian.Uint32(msg.FiveGSTMSI.FiveGTMSI))
	if err != nil {
		logger.WithTrace(ctx, ueConn.Log).Warn("invalid tmsi", zap.Error(err))
	}

	guti, err := etsi.NewGUTI5G(operatorInfo.Guami.PlmnID.Mcc, operatorInfo.Guami.PlmnID.Mnc, amfID, tmsi)
	if err != nil {
		logger.WithTrace(ctx, ueConn.Log).Warn("invalid guti", zap.Error(err))
	}

	amfUe, ok := amfInstance.LookupUeByGuti(guti)
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
		logger.WithTrace(ctx, ueConn.Log).Debug("Implicit Deregistration", zap.Int64("RanUeNgapID", ueConn.RanUeNgapID))
	}

	logger.WithTrace(ctx, ueConn.Log).Debug("UeContext Attach UeConn", zap.Int64("RanUeNgapID", ueConn.RanUeNgapID))
	amfInstance.AttachUeConn(amfUe, ueConn)
}
