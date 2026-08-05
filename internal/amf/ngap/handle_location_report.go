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
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// HandleLocationReport records the UE's serving cell from an NG-RAN node
// LOCATION REPORT (TS 38.413 §8.12.3).
func HandleLocationReport(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.LocationReport) {
	ueConn, ok := resolveUEIDs(ctx, amfInstance, ran, &msg.AMFUENGAPID, &msg.RANUENGAPID)
	if !ok {
		return
	}

	ueConn.TouchLastSeen()

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	// LocationReportingRequestType is ignore criticality, so §10.3.5 delivers a
	// report without it; the location above is recorded either way.
	if msg.LocationReportingRequestType == nil {
		logger.WithTrace(ctx, ueConn.Log).Warn("Location Report carries no LocationReportingRequestType")
		return
	}

	logger.WithTrace(ctx, ueConn.Log).Debug("Handle Location Report",
		zap.Uint32("ran-ue-id", uint32(ueConn.RanUeNgapID)),
		zap.Uint64("amf-ue-id", uint64(ueConn.AmfUeNgapID)),
		zap.Int("report-area", int(msg.LocationReportingRequestType.ReportArea)))

	switch msg.LocationReportingRequestType.EventType {
	case ngap.EventTypeDirect:
		logger.WithTrace(ctx, ueConn.Log).Debug("To report directly")

	case ngap.EventTypeChangeOfServeCell:
		logger.WithTrace(ctx, ueConn.Log).Debug("To report upon change of serving cell")

	case ngap.EventTypeUEPresenceInAreaOfInterest:
		// This AMF never requests area-of-interest reporting, so the library
		// refuses an areaOfInterestList and there is nothing to match against;
		// the presences are reported for the record only.
		for _, item := range msg.UEPresenceInAreaOfInterestList {
			logger.WithTrace(ctx, ueConn.Log).Debug("UE presence in an area this AMF did not request",
				zap.Int("reference-id", int(item.LocationReportingReferenceID)),
				zap.Int("ue-presence", int(item.UEPresence)))
		}

	case ngap.EventTypeStopChangeOfServeCell:
		if err := ueConn.SendLocationReportingControl(ctx, msg.LocationReportingRequestType.EventType); err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("error sending location reporting control", zap.Error(err))
		}

	case ngap.EventTypeStopUEPresenceInAreaOfInterest:
		if msg.LocationReportingRequestType.LocationReportingReferenceIDToBeCancelled == nil {
			logger.WithTrace(ctx, ueConn.Log).Warn("stop-ue-presence-in-area-of-interest with no reference id to cancel")
			break
		}

		logger.WithTrace(ctx, ueConn.Log).Debug("To stop reporting UE presence in the area of interest",
			zap.Int("reference-id", int(*msg.LocationReportingRequestType.LocationReportingReferenceIDToBeCancelled)))

	case ngap.EventTypeCancelLocationReportingForTheUE:
		logger.WithTrace(ctx, ueConn.Log).Debug("To cancel location reporting for the UE")
	}
}
