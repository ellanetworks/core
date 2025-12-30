package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceModifyResponse(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.PDUSessionResourceModifyResponse) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var userLocationInformation *ngapType.UserLocationInformation

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

	var ranUe *amfContext.RanUe

	if rANUENGAPID != nil {
		ranUe = ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
		if ranUe == nil {
			ran.Log.Warn("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		}
	}

	if aMFUENGAPID != nil {
		ranUe = amf.FindRanUeByAmfUeNgapID(aMFUENGAPID.Value)
		if ranUe == nil {
			ran.Log.Warn("No UE Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
			return
		}
	}

	if ranUe != nil {
		ranUe.Radio = ran
		ranUe.Log.Debug("Handle PDUSessionResourceModifyResponse", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))

		if userLocationInformation != nil {
			ranUe.UpdateLocation(ctx, amf, userLocationInformation)
		}
	}
}
