package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleUEContextModificationResponse(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.UEContextModificationResponse) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var rRCState *ngapType.RRCState
	var userLocationInformation *ngapType.UserLocationInformation

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID: // ignore
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				ran.Log.Warn("AmfUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDRANUENGAPID: // ignore
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				ran.Log.Warn("RanUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDRRCState: // optional, ignore
			rRCState = ie.Value.RRCState
		case ngapType.ProtocolIEIDUserLocationInformation: // optional, ignore
			userLocationInformation = ie.Value.UserLocationInformation
		}
	}

	var ranUe *amfContext.RanUe
	if rANUENGAPID != nil {
		ranUe = ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
		if ranUe == nil {
			ran.Log.Warn("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value), zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
		}
	}

	if aMFUENGAPID != nil {
		ranUe = amf.FindRanUeByAmfUeNgapID(aMFUENGAPID.Value)
		if ranUe == nil {
			ran.Log.Warn("UE Context not found", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
			return
		}
	}

	if ranUe != nil {
		ranUe.Radio = ran
		ranUe.Log.Debug("Handle UE Context Modification Response", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

		if rRCState != nil {
			switch rRCState.Value {
			case ngapType.RRCStatePresentInactive:
				ranUe.Log.Debug("UE RRC State: Inactive")
			case ngapType.RRCStatePresentConnected:
				ranUe.Log.Debug("UE RRC State: Connected")
			}
		}

		if userLocationInformation != nil {
			ranUe.UpdateLocation(ctx, amf, userLocationInformation)
		}
	}
}
