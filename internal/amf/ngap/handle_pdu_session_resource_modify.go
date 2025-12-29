package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceNotify(ctx context.Context, ran *amfContext.AmfRan, msg *ngapType.PDUSessionResourceNotify) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var pDUSessionResourceNotifyList *ngapType.PDUSessionResourceNotifyList
	var pDUSessionResourceReleasedListNot *ngapType.PDUSessionResourceReleasedListNot
	var userLocationInformation *ngapType.UserLocationInformation

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID // reject
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID // reject
		case ngapType.ProtocolIEIDPDUSessionResourceNotifyList: // reject
			pDUSessionResourceNotifyList = ie.Value.PDUSessionResourceNotifyList
			if pDUSessionResourceNotifyList == nil {
				ran.Log.Error("pDUSessionResourceNotifyList is nil")
			}
		case ngapType.ProtocolIEIDPDUSessionResourceReleasedListNot: // ignore
			pDUSessionResourceReleasedListNot = ie.Value.PDUSessionResourceReleasedListNot
			if pDUSessionResourceReleasedListNot == nil {
				ran.Log.Error("PDUSessionResourceReleasedListNot is nil")
			}
		case ngapType.ProtocolIEIDUserLocationInformation: // optional, ignore
			userLocationInformation = ie.Value.UserLocationInformation
			if userLocationInformation == nil {
				ran.Log.Warn("userLocationInformation is nil [optional]")
			}
		}
	}

	var ranUe *amfContext.RanUe

	ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Warn("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
	}

	ranUe = amfContext.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Warn("UE Context not found", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
		return
	}

	ranUe.Ran = ran
	ranUe.Log.Debug("Handle PDUSessionResourceNotify", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))

	if userLocationInformation != nil {
		ranUe.UpdateLocation(ctx, userLocationInformation)
	}

	ranUe.Log.Debug("Send PDUSessionResourceNotifyTransfer to SMF")
}
