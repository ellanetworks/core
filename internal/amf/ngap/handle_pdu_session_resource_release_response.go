package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceReleaseResponse(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.PDUSessionResourceReleaseResponse) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID                    *ngapType.AMFUENGAPID
		rANUENGAPID                    *ngapType.RANUENGAPID
		pDUSessionResourceReleasedList *ngapType.PDUSessionResourceReleasedListRelRes
		userLocationInformation        *ngapType.UserLocationInformation
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
		case ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes:
			pDUSessionResourceReleasedList = ie.Value.PDUSessionResourceReleasedListRelRes
			if pDUSessionResourceReleasedList == nil {
				logger.WithTrace(ctx, ran.Log).Error("PDUSessionResourceReleasedList is nil")
				return
			}
		case ngapType.ProtocolIEIDUserLocationInformation:
			userLocationInformation = ie.Value.UserLocationInformation
		}
	}

	if aMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMFUENGAPID IE (mandatory) is missing in PDUSessionResourceReleaseResponse")
		return
	}

	if rANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID IE (mandatory) is missing in PDUSessionResourceReleaseResponse")
		return
	}

	ranUe := ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value), zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		return
	}

	if userLocationInformation != nil {
		ranUe.UpdateLocation(ctx, amf, userLocationInformation)
	}

	ranUe.TouchLastSeen()

	amfUe := ranUe.AmfUe
	if amfUe == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("amfUe is nil")
		return
	}

	if pDUSessionResourceReleasedList != nil {
		logger.WithTrace(ctx, ranUe.Log).Debug("Send PDUSessionResourceReleaseResponseTransfer to SMF")

		for _, item := range pDUSessionResourceReleasedList.List {
			if item.PDUSessionID.Value < 1 || item.PDUSessionID.Value > 15 {
				logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
				continue
			}

			pduSessionID := uint8(item.PDUSessionID.Value)

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				continue
			}

			err := amf.Smf.UpdateSmContextN2InfoPduResRelRsp(ctx, smContext.Ref)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Error("SendUpdateSmContextN2InfoPduResRelRsp failed", zap.Error(err))
				continue
			}

			smContext.PduSessionInactive = true
		}
	}
}
