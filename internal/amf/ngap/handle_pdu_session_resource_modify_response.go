package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceModifyResponse(ctx ctxt.Context, ran *context.AmfRan, message *ngapType.NGAPPDU) {
	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if message == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}
	successfulOutcome := message.SuccessfulOutcome
	if successfulOutcome == nil {
		ran.Log.Error("SuccessfulOutcome is nil")
		return
	}
	pDUSessionResourceModifyResponse := successfulOutcome.Value.PDUSessionResourceModifyResponse
	if pDUSessionResourceModifyResponse == nil {
		ran.Log.Error("PDUSessionResourceModifyResponse is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var userLocationInformation *ngapType.UserLocationInformation

	for _, ie := range pDUSessionResourceModifyResponse.ProtocolIEs.List {
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

	var ranUe *context.RanUe

	if rANUENGAPID != nil {
		ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
		if ranUe == nil {
			ran.Log.Warn("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		}
	}

	if aMFUENGAPID != nil {
		ranUe = context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
		if ranUe == nil {
			ran.Log.Warn("No UE Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
			return
		}
	}

	if ranUe != nil {
		ranUe.Ran = ran
		ranUe.Log.Debug("Handle PDUSessionResourceModifyResponse", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))

		if userLocationInformation != nil {
			ranUe.UpdateLocation(ctx, userLocationInformation)
		}
	}
}
