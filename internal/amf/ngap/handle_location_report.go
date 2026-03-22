package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleLocationReport(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.LocationReport) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID                    *ngapType.AMFUENGAPID
		rANUENGAPID                    *ngapType.RANUENGAPID
		userLocationInformation        *ngapType.UserLocationInformation
		uEPresenceInAreaOfInterestList *ngapType.UEPresenceInAreaOfInterestList
		locationReportingRequestType   *ngapType.LocationReportingRequestType
	)

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID: // reject
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("AmfUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDRANUENGAPID: // reject
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("RanUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDUserLocationInformation: // ignore
			userLocationInformation = ie.Value.UserLocationInformation
			if userLocationInformation == nil {
				logger.WithTrace(ctx, ran.Log).Warn("userLocationInformation is nil")
			}
		case ngapType.ProtocolIEIDUEPresenceInAreaOfInterestList: // optional, ignore
			uEPresenceInAreaOfInterestList = ie.Value.UEPresenceInAreaOfInterestList
			if uEPresenceInAreaOfInterestList == nil {
				logger.WithTrace(ctx, ran.Log).Warn("uEPresenceInAreaOfInterestList is nil [optional]")
			}
		case ngapType.ProtocolIEIDLocationReportingRequestType: // ignore
			locationReportingRequestType = ie.Value.LocationReportingRequestType
			if locationReportingRequestType == nil {
				logger.WithTrace(ctx, ran.Log).Warn("LocationReportingRequestType is nil")
			}
		}
	}

	if rANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID IE (mandatory) is missing in LocationReport")
		return
	}

	if locationReportingRequestType == nil {
		logger.WithTrace(ctx, ran.Log).Error("LocationReportingRequestType IE (mandatory) is missing in LocationReport")
		return
	}

	ranUe := ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		return
	}

	ranUe.UpdateLocation(ctx, amf, userLocationInformation)
	ranUe.TouchLastSeen()

	// logger.WithTrace(ctx, ranUe.Log).Debugf("Report Area[%d]", locationReportingRequestType.ReportArea.Value)
	logger.WithTrace(ctx, ranUe.Log).Debug("Handle Location Report", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID), zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Any("ReportArea", locationReportingRequestType.ReportArea))

	switch locationReportingRequestType.EventType.Value {
	case ngapType.EventTypePresentDirect:
		logger.WithTrace(ctx, ranUe.Log).Debug("To report directly")

	case ngapType.EventTypePresentChangeOfServeCell:
		logger.WithTrace(ctx, ranUe.Log).Debug("To report upon change of serving cell")

	case ngapType.EventTypePresentUePresenceInAreaOfInterest:
		logger.WithTrace(ctx, ranUe.Log).Debug("To report UE presence in the area of interest")

		if uEPresenceInAreaOfInterestList == nil {
			logger.WithTrace(ctx, ranUe.Log).Warn("UEPresenceInAreaOfInterestList is nil, skipping area of interest processing")
			break
		}

		if locationReportingRequestType.AreaOfInterestList == nil {
			logger.WithTrace(ctx, ranUe.Log).Warn("AreaOfInterestList is nil, skipping area matching")
			break
		}

		for _, uEPresenceInAreaOfInterestItem := range uEPresenceInAreaOfInterestList.List {
			uEPresence := uEPresenceInAreaOfInterestItem.UEPresence.Value
			referenceID := uEPresenceInAreaOfInterestItem.LocationReportingReferenceID.Value

			for _, AOIitem := range locationReportingRequestType.AreaOfInterestList.List {
				if referenceID == AOIitem.LocationReportingReferenceID.Value {
					logger.WithTrace(ctx, ranUe.Log).Debug("To report UE presence in the area of interest", zap.Int("uEPresence", int(uEPresence)), zap.Int("AOI ReferenceID", int(referenceID)))
				}
			}
		}

	case ngapType.EventTypePresentStopChangeOfServeCell:
		err := ranUe.Radio.NGAPSender.SendLocationReportingControl(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, locationReportingRequestType.EventType)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("error sending location reporting control", zap.Error(err))
		}

		logger.WithTrace(ctx, ranUe.Log).Info("sent location reporting control ngap message")
	case ngapType.EventTypePresentStopUePresenceInAreaOfInterest:
		if locationReportingRequestType.LocationReportingReferenceIDToBeCancelled == nil {
			logger.WithTrace(ctx, ranUe.Log).Warn("LocationReportingReferenceIDToBeCancelled is nil, skipping")
			break
		}

		logger.WithTrace(ctx, ranUe.Log).Debug("To stop reporting UE presence in the area of interest", zap.Int64("ReferenceID", locationReportingRequestType.LocationReportingReferenceIDToBeCancelled.Value))

	case ngapType.EventTypePresentCancelLocationReportingForTheUe:
		logger.WithTrace(ctx, ranUe.Log).Debug("To cancel location reporting for the UE")
	}
}
