package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceNotify(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.PDUSessionResourceNotify) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID                       *ngapType.AMFUENGAPID
		rANUENGAPID                       *ngapType.RANUENGAPID
		pDUSessionResourceNotifyList      *ngapType.PDUSessionResourceNotifyList
		pDUSessionResourceReleasedListNot *ngapType.PDUSessionResourceReleasedListNot
		userLocationInformation           *ngapType.UserLocationInformation
	)

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

	if rANUENGAPID == nil {
		ran.Log.Error("RANUENGAPID IE (mandatory) is missing in PDUSessionResourceNotify")
		return
	}

	if aMFUENGAPID == nil {
		ran.Log.Error("AMFUENGAPID IE (mandatory) is missing in PDUSessionResourceNotify")
		return
	}

	var ranUe *amfContext.RanUe

	ranUe = ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Warn("No UE Context", zap.Int64("ran_ue_ngap_id", rANUENGAPID.Value))
	}

	ranUe = amf.FindRanUeByAmfUeNgapID(aMFUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Warn("UE Context not found", zap.Int64("amf_ue_ngap_id", aMFUENGAPID.Value))
		return
	}

	ranUe.Radio = ran
	ranUe.TouchLastSeen()
	ranUe.Log.Debug("Handle PDUSessionResourceNotify", zap.Int64("amf_ue_ngap_id", ranUe.AmfUeNgapID))

	if userLocationInformation != nil {
		ranUe.UpdateLocation(ctx, amf, userLocationInformation)
	}

	ranUe.Log.Debug("Send PDUSessionResourceNotifyTransfer to SMF")
}
