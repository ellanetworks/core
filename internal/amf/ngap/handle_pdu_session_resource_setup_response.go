package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/consumer"
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceSetupResponse(ctx ctxt.Context, ran *context.AmfRan, message *ngapType.NGAPPDU) {
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

	pDUSessionResourceSetupResponse := successfulOutcome.Value.PDUSessionResourceSetupResponse
	if pDUSessionResourceSetupResponse == nil {
		ran.Log.Error("PDUSessionResourceSetupResponse is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var pDUSessionResourceSetupResponseList *ngapType.PDUSessionResourceSetupListSURes
	var pDUSessionResourceFailedToSetupList *ngapType.PDUSessionResourceFailedToSetupListSURes

	for _, ie := range pDUSessionResourceSetupResponse.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID: // ignore
			aMFUENGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID: // ignore
			rANUENGAPID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListSURes: // ignore
			pDUSessionResourceSetupResponseList = ie.Value.PDUSessionResourceSetupListSURes
		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListSURes: // ignore
			pDUSessionResourceFailedToSetupList = ie.Value.PDUSessionResourceFailedToSetupListSURes
		}
	}

	var ranUe *context.RanUe

	if rANUENGAPID != nil {
		ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
		if ranUe == nil {
			ran.Log.Warn("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		}
	}

	if aMFUENGAPID != nil {
		ranUe = context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
		if ranUe == nil {
			ran.Log.Warn("UE Context not found", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
			return
		}
	}

	if ranUe != nil {
		ranUe.Ran = ran

		amfUe := ranUe.AmfUe
		if amfUe == nil {
			ranUe.Log.Error("amfUe is nil")
			return
		}

		if pDUSessionResourceSetupResponseList != nil {
			ranUe.Log.Debug("Send PDUSessionResourceSetupResponseTransfer to SMF")

			for _, item := range pDUSessionResourceSetupResponseList.List {
				pduSessionID := uint8(item.PDUSessionID.Value)
				transfer := item.PDUSessionResourceSetupResponseTransfer
				smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
				if !ok {
					ranUe.Log.Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
					continue
				}
				response, err := consumer.SendUpdateSmContextN2Info(ctx, amfUe, smContext,
					models.N2SmInfoTypePduResSetupRsp, transfer)
				if err != nil {
					ranUe.Log.Error("SendUpdateSmContextN2Info[PDUSessionResourceSetupResponseTransfer] Error", zap.Error(err))
				}
				// RAN initiated QoS Flow Mobility in subclause 5.2.2.3.7
				if response != nil && response.BinaryDataN2SmInformation != nil {
				} else if response == nil {
					ranUe.Log.Error("SendUpdateSmContextN2Info[PDUSessionResourceSetupResponseTransfer] Error: received error response from SMF")
				}
			}
		}

		if pDUSessionResourceFailedToSetupList != nil {
			ranUe.Log.Debug("Send PDUSessionResourceSetupUnsuccessfulTransfer to SMF")

			for _, item := range pDUSessionResourceFailedToSetupList.List {
				pduSessionID := uint8(item.PDUSessionID.Value)
				transfer := item.PDUSessionResourceSetupUnsuccessfulTransfer
				smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
				if !ok {
					ranUe.Log.Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				}
				_, err := consumer.SendUpdateSmContextN2Info(ctx, amfUe, smContext, models.N2SmInfoTypePduResSetupFail, transfer)
				if err != nil {
					ranUe.Log.Error("SendUpdateSmContextN2Info[PDUSessionResourceSetupUnsuccessfulTransfer] Error", zap.Error(err))
				}
			}
		}
	}
}
