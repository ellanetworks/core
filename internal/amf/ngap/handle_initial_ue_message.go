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
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapConvert"
	"go.uber.org/zap"
)

func HandleInitialUEMessage(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.InitialUEMessage) {
	ranUe := ran.FindUEByRanUeNgapID(msg.RANUENGAPID)
	if ranUe != nil {
		// gNB reused a RAN UE NGAP ID before completing the previous
		// UEContextRelease. Drop the stale ranUe so a deferred
		// UEContextReleaseComplete carrying the old AMF UE NGAP ID
		// cannot remove the freshly created context below.
		logger.WithTrace(ctx, ranUe.Log).Debug("RAN UE NGAP ID reused in InitialUEMessage, removing stale RanUe",
			zap.Int64("RanUeNgapID", ranUe.RanUeNgapID),
			zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))

		err := ranUe.Remove(ctx)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
		}

		ranUe = nil
	}

	if ranUe == nil {
		var err error

		ranUe, err = amfInstance.NewRanUe(ran, msg.RANUENGAPID)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("Failed to add Ran UE to the pool", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ranUe.Log).Debug("Added Ran UE to the pool", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

		if msg.FiveGSTMSI != nil {
			logger.WithTrace(ctx, ranUe.Log).Debug("Receive 5G-S-TMSI")

			operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Error("Could not get operator info", zap.Error(err))
				return
			}

			// <5G-S-TMSI> := <AMF Set ID><AMF Pointer><5G-TMSI>
			// GUAMI := <MCC><MNC><AMF Region ID><AMF Set ID><AMF Pointer>
			// 5G-GUTI := <GUAMI><5G-TMSI>
			tmpReginID, _, _ := ngapConvert.AmfIdToNgap(operatorInfo.Guami.AmfID)
			amfID := ngapConvert.AmfIdToModels(tmpReginID, msg.FiveGSTMSI.AMFSetID, msg.FiveGSTMSI.AMFPointer)

			tmsi, err := etsi.NewTMSI(binary.BigEndian.Uint32(msg.FiveGSTMSI.FiveGTMSI))
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Warn("invalid tmsi", zap.Error(err))
			}

			guti, err := etsi.NewGUTI(operatorInfo.Guami.PlmnID.Mcc, operatorInfo.Guami.PlmnID.Mnc, amfID, tmsi)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Warn("invalid guti", zap.Error(err))
			}

			if amfUe, ok := amfInstance.FindUeContextByGuti(guti); !ok {
				logger.WithTrace(ctx, ranUe.Log).Warn("Unknown UE", logger.GUTI(guti.String()))
			} else if !amfUe.ReuseForInboundNAS(msg.NASPDU) {
				// TS 24.501 §4.4.4.3: this message cites an existing context but
				// is not authenticated for it. Do not bind to or mutate the live
				// context; the NAS layer registers it on a fresh context pending
				// authentication.
				logger.WithTrace(ctx, ranUe.Log).Info("Initial UE Message cites a known GUTI but is not authenticated for that context; registering on a fresh context", logger.GUTI(guti.String()))
			} else {
				logger.WithTrace(ctx, ranUe.Log).Debug("find UeContext", logger.GUTI(guti.String()))

				if amfUe.RanUe() != nil {
					logger.WithTrace(ctx, ranUe.Log).Debug("Implicit Deregistration", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))
				}

				logger.WithTrace(ctx, ranUe.Log).Debug("UeContext Attach RanUe", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))
				amfUe.AttachRanUe(ranUe)
			}
		}
	}

	ranUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation.Raw())

	ranUe.UeContextRequest = msg.UEContextRequest

	if ranUe.UeContext() != nil {
		ranUe.UeContext().StopImplicitDeregistrationTimer()
		ranUe.UeContext().StopMobileReachableTimer()
	}

	if amfInstance.NAS == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("NAS handler not set")
		return
	}

	if err := amfInstance.NAS.HandleNAS(ctx, ranUe, msg.NASPDU); err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error handling NAS Message", zap.Error(err))
		sendStatus5GMM(ctx, ranUe, nasMessage.Cause5GMMProtocolErrorUnspecified)
	}
}
