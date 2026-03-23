package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleUEContextModificationResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngapType.UEContextModificationResponse) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID             *ngapType.AMFUENGAPID
		rANUENGAPID             *ngapType.RANUENGAPID
		rRCState                *ngapType.RRCState
		userLocationInformation *ngapType.UserLocationInformation
	)

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID: // ignore
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Warn("AmfUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDRANUENGAPID: // ignore
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Warn("RanUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDRRCState: // optional, ignore
			rRCState = ie.Value.RRCState
		case ngapType.ProtocolIEIDUserLocationInformation: // optional, ignore
			userLocationInformation = ie.Value.UserLocationInformation
		}
	}

	var ranUe *amf.RanUe

	if rANUENGAPID != nil {
		if aMFUENGAPID != nil {
			ranUe = ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
			if ranUe == nil {
				logger.WithTrace(ctx, ran.Log).Warn("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value), zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
			}
		} else {
			ranUe = ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
			if ranUe == nil {
				logger.WithTrace(ctx, ran.Log).Warn("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
			}
		}
	}

	if aMFUENGAPID != nil {
		ranUe = amfInstance.FindRanUeByAmfUeNgapID(aMFUENGAPID.Value)
		if ranUe == nil {
			logger.WithTrace(ctx, ran.Log).Warn("UE Context not found", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
			return
		}
	}

	if ranUe != nil {
		ranUe.Radio = ran
		ranUe.TouchLastSeen()
		logger.WithTrace(ctx, ranUe.Log).Debug("Handle UE Context Modification Response", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

		if rRCState != nil {
			switch rRCState.Value {
			case ngapType.RRCStatePresentInactive:
				logger.WithTrace(ctx, ranUe.Log).Debug("UE RRC State: Inactive")
			case ngapType.RRCStatePresentConnected:
				logger.WithTrace(ctx, ranUe.Log).Debug("UE RRC State: Connected")
			}
		}

		if userLocationInformation != nil {
			ranUe.UpdateLocation(ctx, amfInstance, userLocationInformation)
		}
	}
}
