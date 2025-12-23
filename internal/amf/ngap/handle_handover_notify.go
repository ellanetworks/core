package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverNotify(ctx ctxt.Context, ran *context.AmfRan, message *ngapType.NGAPPDU) {
	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if message == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	initiatingMessage := message.InitiatingMessage
	if initiatingMessage == nil {
		ran.Log.Error("Initiating Message is nil")
		return
	}

	handoverNotify := initiatingMessage.Value.HandoverNotify
	if handoverNotify == nil {
		ran.Log.Error("HandoverNotify is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var userLocationInformation *ngapType.UserLocationInformation

	for i := 0; i < len(handoverNotify.ProtocolIEs.List); i++ {
		ie := handoverNotify.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				ran.Log.Error("AMFUENGAPID is nil")
				return
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				ran.Log.Error("RANUENGAPID is nil")
				return
			}
		case ngapType.ProtocolIEIDUserLocationInformation:
			userLocationInformation = ie.Value.UserLocationInformation
			if userLocationInformation == nil {
				ran.Log.Error("userLocationInformation is nil")
				return
			}
		}
	}

	targetUe := ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)

	if targetUe == nil {
		ran.Log.Error("No RanUe Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value), zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}
		err := ran.NGAPSender.SendErrorIndication(ctx, nil, nil, &cause, nil)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}
		ran.Log.Info("sent error indication", zap.Int64("AMFUENGAPID", aMFUENGAPID.Value))
		return
	}

	if userLocationInformation != nil {
		targetUe.UpdateLocation(ctx, userLocationInformation)
	}

	amfUe := targetUe.AmfUe
	if amfUe == nil {
		ran.Log.Error("AmfUe is nil")
		return
	}

	sourceUe := targetUe.SourceUe
	if sourceUe == nil {
		ran.Log.Error("N2 Handover between AMF has not been implemented yet")
		return
	}

	ran.Log.Info("Handle Handover notification Finshed ")

	amfUe.AttachRanUe(targetUe)
	err := ngap_message.SendUEContextReleaseCommand(ctx, sourceUe, context.UeContextReleaseHandover, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		ran.Log.Error("error sending ue context release command", zap.Error(err))
		return
	}
}
