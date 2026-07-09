// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleLocationReport(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.LocationReport) {
	if msg.LocationReportingRequestType == nil {
		logger.WithTrace(ctx, ran.Log).Error("LocationReportingRequestType IE (mandatory) is missing in LocationReport")
		return
	}

	ueConn, ok := resolveUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	ueConn.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
	ueConn.TouchLastSeen()

	// UserLocationInformationNR carries NRCGI/TAI only; RSRP/RSRQ/TA measurements
	// arrive separately via NRPPa UEPositioningInformation.
	if msg.UserLocationInformation != nil {
		if nr := msg.UserLocationInformation.UserLocationInformationNR; nr != nil {
			_ = nr
		}
	}

	logger.WithTrace(ctx, ueConn.Log).Debug("Handle Location Report", zap.Int64("RanUeNgapID", int64(ueConn.RanUeNgapID)), zap.Int64("AmfUeNgapID", int64(ueConn.AmfUeNgapID)), zap.Any("ReportArea", msg.LocationReportingRequestType.ReportArea))

	switch msg.LocationReportingRequestType.EventType.Value {
	case ngapType.EventTypePresentDirect:
		logger.WithTrace(ctx, ueConn.Log).Debug("To report directly")

	case ngapType.EventTypePresentChangeOfServeCell:
		logger.WithTrace(ctx, ueConn.Log).Debug("To report upon change of serving cell")

	case ngapType.EventTypePresentUePresenceInAreaOfInterest:
		logger.WithTrace(ctx, ueConn.Log).Debug("To report UE presence in the area of interest")

		if msg.UEPresenceInAreaOfInterestList == nil {
			logger.WithTrace(ctx, ueConn.Log).Warn("UEPresenceInAreaOfInterestList is nil, skipping area of interest processing")
			break
		}

		if msg.LocationReportingRequestType.AreaOfInterestList == nil {
			logger.WithTrace(ctx, ueConn.Log).Warn("AreaOfInterestList is nil, skipping area matching")
			break
		}

		for _, uEPresenceInAreaOfInterestItem := range msg.UEPresenceInAreaOfInterestList.List {
			uEPresence := uEPresenceInAreaOfInterestItem.UEPresence.Value
			referenceID := uEPresenceInAreaOfInterestItem.LocationReportingReferenceID.Value

			for _, AOIitem := range msg.LocationReportingRequestType.AreaOfInterestList.List {
				if referenceID == AOIitem.LocationReportingReferenceID.Value {
					logger.WithTrace(ctx, ueConn.Log).Debug("To report UE presence in the area of interest", zap.Int("uEPresence", int(uEPresence)), zap.Int("AOI ReferenceID", int(referenceID)))
				}
			}
		}

	case ngapType.EventTypePresentStopChangeOfServeCell:
		pkt, err := send.BuildLocationReportingControl(int64(ueConn.AmfUeNgapID), int64(ueConn.RanUeNgapID), msg.LocationReportingRequestType.EventType)
		if err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("error building location reporting control", zap.Error(err))
		} else if err := ueConn.SendNGAP(ctx, send.NGAPProcedureLocationReportingControl, pkt); err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("error sending location reporting control", zap.Error(err))
		}
	case ngapType.EventTypePresentStopUePresenceInAreaOfInterest:
		if msg.LocationReportingRequestType.LocationReportingReferenceIDToBeCancelled == nil {
			logger.WithTrace(ctx, ueConn.Log).Warn("LocationReportingReferenceIDToBeCancelled is nil, skipping")
			break
		}

		logger.WithTrace(ctx, ueConn.Log).Debug("To stop reporting UE presence in the area of interest", zap.Int64("ReferenceID", msg.LocationReportingRequestType.LocationReportingReferenceIDToBeCancelled.Value))

	case ngapType.EventTypePresentCancelLocationReportingForTheUe:
		logger.WithTrace(ctx, ueConn.Log).Debug("To cancel location reporting for the UE")
	}
}
