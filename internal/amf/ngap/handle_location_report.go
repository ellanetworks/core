package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleLocationReport(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var userLocationInformation *ngapType.UserLocationInformation
	var uEPresenceInAreaOfInterestList *ngapType.UEPresenceInAreaOfInterestList
	var locationReportingRequestType *ngapType.LocationReportingRequestType

	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	initiatingMessage := msg.InitiatingMessage
	if initiatingMessage == nil {
		ran.Log.Error("InitiatingMessage is nil")
		return
	}

	locationReport := initiatingMessage.Value.LocationReport
	if locationReport == nil {
		ran.Log.Error("LocationReport is nil")
		return
	}

	for _, ie := range locationReport.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID: // reject
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				ran.Log.Error("AmfUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDRANUENGAPID: // reject
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				ran.Log.Error("RanUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDUserLocationInformation: // ignore
			userLocationInformation = ie.Value.UserLocationInformation
			if userLocationInformation == nil {
				ran.Log.Warn("userLocationInformation is nil")
			}
		case ngapType.ProtocolIEIDUEPresenceInAreaOfInterestList: // optional, ignore
			uEPresenceInAreaOfInterestList = ie.Value.UEPresenceInAreaOfInterestList
			if uEPresenceInAreaOfInterestList == nil {
				ran.Log.Warn("uEPresenceInAreaOfInterestList is nil [optional]")
			}
		case ngapType.ProtocolIEIDLocationReportingRequestType: // ignore
			locationReportingRequestType = ie.Value.LocationReportingRequestType
			if locationReportingRequestType == nil {
				ran.Log.Warn("LocationReportingRequestType is nil")
			}
		}
	}

	ranUe := ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Error("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		return
	}

	ranUe.UpdateLocation(ctx, userLocationInformation)

	// ranUe.Log.Debugf("Report Area[%d]", locationReportingRequestType.ReportArea.Value)
	ranUe.Log.Debug("Handle Location Report", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID), zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Any("ReportArea", locationReportingRequestType.ReportArea))

	switch locationReportingRequestType.EventType.Value {
	case ngapType.EventTypePresentDirect:
		ranUe.Log.Debug("To report directly")

	case ngapType.EventTypePresentChangeOfServeCell:
		ranUe.Log.Debug("To report upon change of serving cell")

	case ngapType.EventTypePresentUePresenceInAreaOfInterest:
		ranUe.Log.Debug("To report UE presence in the area of interest")
		for _, uEPresenceInAreaOfInterestItem := range uEPresenceInAreaOfInterestList.List {
			uEPresence := uEPresenceInAreaOfInterestItem.UEPresence.Value
			referenceID := uEPresenceInAreaOfInterestItem.LocationReportingReferenceID.Value

			for _, AOIitem := range locationReportingRequestType.AreaOfInterestList.List {
				if referenceID == AOIitem.LocationReportingReferenceID.Value {
					ranUe.Log.Debug("To report UE presence in the area of interest", zap.Int("uEPresence", int(uEPresence)), zap.Int("AOI ReferenceID", int(referenceID)))
				}
			}
		}

	case ngapType.EventTypePresentStopChangeOfServeCell:
		err := message.SendLocationReportingControl(ctx, ranUe, nil, 0, locationReportingRequestType.EventType)
		if err != nil {
			ranUe.Log.Error("error sending location reporting control", zap.Error(err))
		}
		ranUe.Log.Info("sent location reporting control ngap message")
	case ngapType.EventTypePresentStopUePresenceInAreaOfInterest:
		ranUe.Log.Debug("To stop reporting UE presence in the area of interest", zap.Int64("ReferenceID", locationReportingRequestType.LocationReportingReferenceIDToBeCancelled.Value))

	case ngapType.EventTypePresentCancelLocationReportingForTheUe:
		ranUe.Log.Debug("To cancel location reporting for the UE")
	}
}
