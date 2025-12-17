package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceNotify(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
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

	PDUSessionResourceNotify := initiatingMessage.Value.PDUSessionResourceNotify
	if PDUSessionResourceNotify == nil {
		ran.Log.Error("PDUSessionResourceNotify is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var pDUSessionResourceNotifyList *ngapType.PDUSessionResourceNotifyList
	var pDUSessionResourceReleasedListNot *ngapType.PDUSessionResourceReleasedListNot
	var userLocationInformation *ngapType.UserLocationInformation

	for _, ie := range PDUSessionResourceNotify.ProtocolIEs.List {
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

	var ranUe *context.RanUe

	ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Warn("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
	}

	ranUe = context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
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
