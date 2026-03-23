package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceModifyResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngapType.PDUSessionResourceModifyResponse) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID             *ngapType.AMFUENGAPID
		rANUENGAPID             *ngapType.RANUENGAPID
		userLocationInformation *ngapType.UserLocationInformation
	)

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID: // ignore
			aMFUENGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID: // ignore
			rANUENGAPID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDPDUSessionResourceModifyListModRes: // ignore
		case ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModRes: // ignore
		case ngapType.ProtocolIEIDUserLocationInformation: // optional, ignore
			userLocationInformation = ie.Value.UserLocationInformation
		}
	}

	var ranUe *amf.RanUe

	if rANUENGAPID != nil {
		ranUe = ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
		if ranUe == nil {
			logger.WithTrace(ctx, ran.Log).Warn("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		}
	}

	if aMFUENGAPID != nil {
		ranUe = amfInstance.FindRanUeByAmfUeNgapID(aMFUENGAPID.Value)
		if ranUe == nil {
			logger.WithTrace(ctx, ran.Log).Warn("No UE Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
			return
		}
	}

	if ranUe != nil {
		ranUe.Radio = ran
		ranUe.TouchLastSeen()
		logger.WithTrace(ctx, ranUe.Log).Debug("Handle PDUSessionResourceModifyResponse", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))

		if userLocationInformation != nil {
			ranUe.UpdateLocation(ctx, amfInstance, userLocationInformation)
		}
	}
}
