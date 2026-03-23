package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleUplinkNasTransport(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngapType.UplinkNASTransport) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID             *ngapType.AMFUENGAPID
		rANUENGAPID             *ngapType.RANUENGAPID
		nASPDU                  *ngapType.NASPDU
		userLocationInformation *ngapType.UserLocationInformation
	)

	for i := 0; i < len(msg.ProtocolIEs.List); i++ {
		ie := msg.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("AmfUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("RanUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDNASPDU:
			nASPDU = ie.Value.NASPDU
			if nASPDU == nil {
				logger.WithTrace(ctx, ran.Log).Error("nASPDU is nil")
				return
			}
		case ngapType.ProtocolIEIDUserLocationInformation:
			userLocationInformation = ie.Value.UserLocationInformation
			if userLocationInformation == nil {
				logger.WithTrace(ctx, ran.Log).Error("UserLocationInformation is nil")
				return
			}
		}
	}

	if aMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMFUENGAPID IE (mandatory) is missing in UplinkNASTransport")
		return
	}

	if rANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID IE (mandatory) is missing in UplinkNASTransport")
		return
	}

	if nASPDU == nil {
		logger.WithTrace(ctx, ran.Log).Error("NASPDU IE (mandatory) is missing in UplinkNASTransport")
		return
	}

	ranUe := ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("ran ue is nil", zap.Int64("ranUeNgapID", rANUENGAPID.Value))
		return
	}

	ranUe.Radio = ran
	ranUe.TouchLastSeen()

	amfUe := ranUe.AmfUe
	if amfUe == nil {
		err := ranUe.Remove()
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error removing ran ue context", zap.Error(err))
		}

		logger.WithTrace(ctx, ran.Log).Error("No UE Context of RanUe", zap.Int64("ranUeNgapID", rANUENGAPID.Value), zap.Int64("amfUeNgapID", aMFUENGAPID.Value))

		return
	}

	if userLocationInformation != nil {
		ranUe.UpdateLocation(ctx, amfInstance, userLocationInformation)
	}

	err := nas.HandleNAS(ctx, amfInstance, ranUe, nASPDU.Value)
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error handling NAS message", zap.Error(err))
	}
}
