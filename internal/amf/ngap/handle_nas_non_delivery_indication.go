package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleNasNonDeliveryIndication(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.NASNonDeliveryIndication) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
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
				ran.Log.Error("AmfUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				ran.Log.Error("RanUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDNASPDU:
			nASPDU = ie.Value.NASPDU
			if nASPDU == nil {
				ran.Log.Error("NasPdu is nil")
				return
			}
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
			if cause == nil {
				ran.Log.Error("Cause is nil")
				return
			}
		}
	}

	if rANUENGAPID == nil {
		ran.Log.Error("RANUENGAPID IE (mandatory) is missing in NASNonDeliveryIndication")
		return
	}

	if aMFUENGAPID == nil {
		ran.Log.Error("AMFUENGAPID IE (mandatory) is missing in NASNonDeliveryIndication")
		return
	}

	if nASPDU == nil {
		ran.Log.Error("NASPDU IE (mandatory) is missing in NASNonDeliveryIndication")
		return
	}

	if cause == nil {
		ran.Log.Error("Cause IE (mandatory) is missing in NASNonDeliveryIndication")
		return
	}

	ranUe := ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Error("No UE Context", zap.Int64("ran_ue_ngap_id", rANUENGAPID.Value))
		return
	}

	ran.Log.Debug("Handle NAS Non Delivery Indication", zap.Int64("ran_ue_ngap_id", ranUe.RanUeNgapID), zap.Int64("amf_ue_ngap_id", ranUe.AmfUeNgapID), zap.String("cause", causeToString(*cause)))
	ranUe.TouchLastSeen()

	err := nas.HandleNAS(ctx, amf, ranUe, nASPDU.Value)
	if err != nil {
		ranUe.Log.Error("error handling NAS", zap.Error(err))
	}
}
