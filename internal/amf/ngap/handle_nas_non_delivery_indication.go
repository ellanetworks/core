package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleNasNonDeliveryIndication(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.NASNonDeliveryIndication) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID *ngapType.AMFUENGAPID
		rANUENGAPID *ngapType.RANUENGAPID
		nASPDU      *ngapType.NASPDU
		cause       *ngapType.Cause
	)

	for _, ie := range msg.ProtocolIEs.List {
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
				logger.WithTrace(ctx, ran.Log).Error("NasPdu is nil")
				return
			}
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
			if cause == nil {
				logger.WithTrace(ctx, ran.Log).Error("Cause is nil")
				return
			}
		}
	}

	if rANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID IE (mandatory) is missing in NASNonDeliveryIndication")
		return
	}

	if aMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMFUENGAPID IE (mandatory) is missing in NASNonDeliveryIndication")
		return
	}

	if nASPDU == nil {
		logger.WithTrace(ctx, ran.Log).Error("NASPDU IE (mandatory) is missing in NASNonDeliveryIndication")
		return
	}

	if cause == nil {
		logger.WithTrace(ctx, ran.Log).Error("Cause IE (mandatory) is missing in NASNonDeliveryIndication")
		return
	}

	ranUe := ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		return
	}

	logger.WithTrace(ctx, ran.Log).Debug("Handle NAS Non Delivery Indication", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID), zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), logger.Cause(causeToString(*cause)))
	ranUe.TouchLastSeen()

	err := nas.HandleNAS(ctx, amf, ranUe, nASPDU.Value)
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error handling NAS", zap.Error(err))
	}
}
