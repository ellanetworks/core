package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceReleaseResponse(ctx ctxt.Context, ran *context.AmfRan, message *ngapType.NGAPPDU) {
	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if message == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	successfulOutcome := message.SuccessfulOutcome
	if successfulOutcome == nil {
		ran.Log.Error("SuccessfulOutcome is nil")
		return
	}

	pDUSessionResourceReleaseResponse := successfulOutcome.Value.PDUSessionResourceReleaseResponse
	if pDUSessionResourceReleaseResponse == nil {
		ran.Log.Error("PDUSessionResourceReleaseResponse is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var pDUSessionResourceReleasedList *ngapType.PDUSessionResourceReleasedListRelRes
	var userLocationInformation *ngapType.UserLocationInformation

	for _, ie := range pDUSessionResourceReleaseResponse.ProtocolIEs.List {
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
		case ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes:
			pDUSessionResourceReleasedList = ie.Value.PDUSessionResourceReleasedListRelRes
			if pDUSessionResourceReleasedList == nil {
				ran.Log.Error("PDUSessionResourceReleasedList is nil")
				return
			}
		case ngapType.ProtocolIEIDUserLocationInformation:
			userLocationInformation = ie.Value.UserLocationInformation
		}
	}

	ranUe := ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Error("No UE Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value), zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		return
	}

	if userLocationInformation != nil {
		ranUe.UpdateLocation(ctx, userLocationInformation)
	}

	amfUe := ranUe.AmfUe
	if amfUe == nil {
		ranUe.Log.Error("amfUe is nil")
		return
	}

	if pDUSessionResourceReleasedList != nil {
		ranUe.Log.Debug("Send PDUSessionResourceReleaseResponseTransfer to SMF")

		for _, item := range pDUSessionResourceReleasedList.List {
			pduSessionID := uint8(item.PDUSessionID.Value)
			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				ranUe.Log.Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				continue
			}
			err := pdusession.UpdateSmContextN2InfoPduResRelRsp(ctx, smContext.SmContextRef())
			if err != nil {
				ranUe.Log.Error("SendUpdateSmContextN2InfoPduResRelRsp failed", zap.Error(err))
				continue
			}
			smContext.SetPduSessionInActive(true)
		}
	}
}
