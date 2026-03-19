package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceSetupResponse(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.PDUSessionResourceSetupResponse) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID                         *ngapType.AMFUENGAPID
		rANUENGAPID                         *ngapType.RANUENGAPID
		pDUSessionResourceSetupResponseList *ngapType.PDUSessionResourceSetupListSURes
		pDUSessionResourceFailedToSetupList *ngapType.PDUSessionResourceFailedToSetupListSURes
	)

	for _, ie := range msg.ProtocolIEs.List {
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

	var ranUe *amfContext.RanUe

	if rANUENGAPID != nil {
		ranUe = ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
		if ranUe == nil {
			ran.Log.Warn("No UE Context", zap.Int64("ran_ue_ngap_id", rANUENGAPID.Value))
		}
	}

	if aMFUENGAPID != nil {
		ranUe = amf.FindRanUeByAmfUeNgapID(aMFUENGAPID.Value)
		if ranUe == nil {
			ran.Log.Warn("UE Context not found", zap.Int64("amf_ue_ngap_id", aMFUENGAPID.Value))
			return
		}
	}

	if ranUe != nil {
		ranUe.Radio = ran
		ranUe.TouchLastSeen()

		amfUe := ranUe.AmfUe
		if amfUe == nil {
			ranUe.Log.Error("amfUe is nil")
			return
		}

		if pDUSessionResourceSetupResponseList != nil {
			ranUe.Log.Debug("Send PDUSessionResourceSetupResponseTransfer to SMF")

			for _, item := range pDUSessionResourceSetupResponseList.List {
				if item.PDUSessionID.Value < 1 || item.PDUSessionID.Value > 15 {
					ranUe.Log.Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
					continue
				}

				pduSessionID := uint8(item.PDUSessionID.Value)
				transfer := item.PDUSessionResourceSetupResponseTransfer

				smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
				if !ok {
					ranUe.Log.Error("SmContext not found", zap.Uint8("pdu_session_id", pduSessionID))
					continue
				}

				err := pdusession.UpdateSmContextN2InfoPduResSetupRsp(ctx, smContext.Ref, transfer)
				if err != nil {
					ranUe.Log.Error("SendUpdateSmContextN2Info[PDUSessionResourceSetupResponseTransfer] Error", zap.Error(err))
				}
			}
		}

		if pDUSessionResourceFailedToSetupList != nil {
			ranUe.Log.Debug("Send PDUSessionResourceSetupUnsuccessfulTransfer to SMF")

			for _, item := range pDUSessionResourceFailedToSetupList.List {
				if item.PDUSessionID.Value < 1 || item.PDUSessionID.Value > 15 {
					ranUe.Log.Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
					continue
				}

				pduSessionID := uint8(item.PDUSessionID.Value)
				transfer := item.PDUSessionResourceSetupUnsuccessfulTransfer

				smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
				if !ok {
					ranUe.Log.Error("SmContext not found", zap.Uint8("pdu_session_id", pduSessionID))
					continue
				}

				err := pdusession.UpdateSmContextN2InfoPduResSetupFail(smContext.Ref, transfer)
				if err != nil {
					ranUe.Log.Error("SendUpdateSmContextN2Info[PDUSessionResourceSetupUnsuccessfulTransfer] Error", zap.Error(err))
				}
			}
		}
	}
}
