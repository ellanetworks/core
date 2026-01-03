package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverNotify(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.HandoverNotify) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID             *ngapType.AMFUENGAPID
		rANUENGAPID             *ngapType.RANUENGAPID
		userLocationInformation *ngapType.UserLocationInformation
	)

	for i := 0; i < len(msg.ProtocolIEs.List); i++ {
		ie := msg.ProtocolIEs.List[i]
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

	targetUe := ran.FindUEByRanUeNgapID(rANUENGAPID.Value)

	if targetUe == nil {
		ran.Log.Error("No RanUe Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value), zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}

		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}

		ran.Log.Info("sent error indication", zap.Int64("AMFUENGAPID", aMFUENGAPID.Value))

		return
	}

	if userLocationInformation != nil {
		targetUe.UpdateLocation(ctx, amf, userLocationInformation)
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

	sourceUe.ReleaseAction = amfContext.UeContextReleaseHandover

	err := sourceUe.Radio.NGAPSender.SendUEContextReleaseCommand(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		ran.Log.Error("error sending ue context release command", zap.Error(err))
		return
	}
}
