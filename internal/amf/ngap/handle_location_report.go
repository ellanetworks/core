package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleLocationReport(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.LocationReport) {
	if msg.LocationReportingRequestType == nil {
		logger.WithTrace(ctx, ran.Log).Error("LocationReportingRequestType IE (mandatory) is missing in LocationReport")
		return
	}

	ranUe := ran.FindUEByRanUeNgapID(msg.RANUENGAPID)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("RanUeNgapID", msg.RANUENGAPID))
		return
	}

	ranUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
	ranUe.TouchLastSeen()

	logger.WithTrace(ctx, ranUe.Log).Debug("Handle Location Report", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID), zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Any("ReportArea", msg.LocationReportingRequestType.ReportArea))

	switch msg.LocationReportingRequestType.EventType.Value {
	case ngapType.EventTypePresentDirect:
		logger.WithTrace(ctx, ranUe.Log).Debug("To report directly")

	case ngapType.EventTypePresentChangeOfServeCell:
		logger.WithTrace(ctx, ranUe.Log).Debug("To report upon change of serving cell")

	case ngapType.EventTypePresentUePresenceInAreaOfInterest:
		logger.WithTrace(ctx, ranUe.Log).Debug("To report UE presence in the area of interest")

		if msg.UEPresenceInAreaOfInterestList == nil {
			logger.WithTrace(ctx, ranUe.Log).Warn("UEPresenceInAreaOfInterestList is nil, skipping area of interest processing")
			break
		}

		if msg.LocationReportingRequestType.AreaOfInterestList == nil {
			logger.WithTrace(ctx, ranUe.Log).Warn("AreaOfInterestList is nil, skipping area matching")
			break
		}

		for _, uEPresenceInAreaOfInterestItem := range msg.UEPresenceInAreaOfInterestList.List {
			uEPresence := uEPresenceInAreaOfInterestItem.UEPresence.Value
			referenceID := uEPresenceInAreaOfInterestItem.LocationReportingReferenceID.Value

			for _, AOIitem := range msg.LocationReportingRequestType.AreaOfInterestList.List {
				if referenceID == AOIitem.LocationReportingReferenceID.Value {
					logger.WithTrace(ctx, ranUe.Log).Debug("To report UE presence in the area of interest", zap.Int("uEPresence", int(uEPresence)), zap.Int("AOI ReferenceID", int(referenceID)))
				}
			}
		}

	case ngapType.EventTypePresentStopChangeOfServeCell:
		err := ranUe.Radio.NGAPSender.SendLocationReportingControl(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, msg.LocationReportingRequestType.EventType)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("error sending location reporting control", zap.Error(err))
		}

		logger.WithTrace(ctx, ranUe.Log).Info("sent location reporting control ngap message")
	case ngapType.EventTypePresentStopUePresenceInAreaOfInterest:
		if msg.LocationReportingRequestType.LocationReportingReferenceIDToBeCancelled == nil {
			logger.WithTrace(ctx, ranUe.Log).Warn("LocationReportingReferenceIDToBeCancelled is nil, skipping")
			break
		}

		logger.WithTrace(ctx, ranUe.Log).Debug("To stop reporting UE presence in the area of interest", zap.Int64("ReferenceID", msg.LocationReportingRequestType.LocationReportingReferenceIDToBeCancelled.Value))

	case ngapType.EventTypePresentCancelLocationReportingForTheUe:
		logger.WithTrace(ctx, ranUe.Log).Debug("To cancel location reporting for the UE")
	}
}
