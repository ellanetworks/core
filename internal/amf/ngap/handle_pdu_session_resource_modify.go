package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/consumer"
	"github.com/ellanetworks/core/internal/amf/context"
	gmm_message "github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceNotify(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	initiatingMessage := msg.InitiatingMessage
	if initiatingMessage == nil {
		ran.Log.Error("InitiatingMessage is nil")
		return
	}

	PDUSessionResourceNotify := initiatingMessage.Value.PDUSessionResourceNotify
	if PDUSessionResourceNotify == nil {
		ran.Log.Error("PDUSessionResourceNotify is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var pDUSessionResourceNotifyList *ngapType.PDUSessionResourceNotifyList
	var pDUSessionResourceReleasedListNot *ngapType.PDUSessionResourceReleasedListNot
	var userLocationInformation *ngapType.UserLocationInformation

	for _, ie := range PDUSessionResourceNotify.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID // reject
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID // reject
		case ngapType.ProtocolIEIDPDUSessionResourceNotifyList: // reject
			pDUSessionResourceNotifyList = ie.Value.PDUSessionResourceNotifyList
			if pDUSessionResourceNotifyList == nil {
				ran.Log.Error("pDUSessionResourceNotifyList is nil")
			}
		case ngapType.ProtocolIEIDPDUSessionResourceReleasedListNot: // ignore
			pDUSessionResourceReleasedListNot = ie.Value.PDUSessionResourceReleasedListNot
			if pDUSessionResourceReleasedListNot == nil {
				ran.Log.Error("PDUSessionResourceReleasedListNot is nil")
			}
		case ngapType.ProtocolIEIDUserLocationInformation: // optional, ignore
			userLocationInformation = ie.Value.UserLocationInformation
			if userLocationInformation == nil {
				ran.Log.Warn("userLocationInformation is nil [optional]")
			}
		}
	}

	var ranUe *context.RanUe

	ranUe = ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Warn("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
	}

	ranUe = context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Warn("UE Context not found", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
		return
	}

	ranUe.Ran = ran
	ranUe.Log.Debug("Handle PDUSessionResourceNotify", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))
	amfUe := ranUe.AmfUe
	if amfUe == nil {
		ranUe.Log.Error("amfUe is nil")
		return
	}

	if userLocationInformation != nil {
		ranUe.UpdateLocation(ctx, userLocationInformation)
	}

	ranUe.Log.Debug("Send PDUSessionResourceNotifyTransfer to SMF")

	for _, item := range pDUSessionResourceNotifyList.List {
		pduSessionID := int32(item.PDUSessionID.Value)
		transfer := item.PDUSessionResourceNotifyTransfer
		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			ranUe.Log.Error("SmContext not found", zap.Int32("PduSessionID", pduSessionID))
		}
		response, err := consumer.SendUpdateSmContextN2Info(ctx, amfUe, smContext,
			models.N2SmInfoTypePduResNty, transfer)
		if err != nil {
			ranUe.Log.Error("SendUpdateSmContextN2Info[PDUSessionResourceNotifyTransfer] Error", zap.Error(err))
		}

		if response != nil {
			responseData := response.JSONData
			n2Info := response.BinaryDataN1SmMessage
			n1Msg := response.BinaryDataN2SmInformation
			if n2Info != nil {
				switch responseData.N2SmInfoType {
				case models.N2SmInfoTypePduResModReq:
					ranUe.Log.Debug("AMF Transfer NGAP PDU Resource Modify Req from SMF")
					var nasPdu []byte
					if n1Msg != nil {
						pduSessionIDUint8 := uint8(pduSessionID)
						nasPdu, err = gmm_message.BuildDLNASTransport(amfUe, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionIDUint8, nil)
						if err != nil {
							ranUe.Log.Warn("error building NAS transport message", zap.Error(err))
						}
					}
					list := ngapType.PDUSessionResourceModifyListModReq{}
					message.AppendPDUSessionResourceModifyListModReq(&list, pduSessionID, nasPdu, n2Info)
					err := message.SendPDUSessionResourceModifyRequest(ctx, ranUe, list)
					if err != nil {
						ranUe.Log.Error("error sending pdu session resource modify request", zap.Error(err))
						return
					}
					ranUe.Log.Info("sent pdu session resource modify request")
				}
			}
		} else if err != nil {
			return
		} else {
			ranUe.Log.Error("Failed to Update smContext", zap.Int32("PduSessionID", pduSessionID), zap.Error(err))
			return
		}
	}

	if pDUSessionResourceReleasedListNot != nil {
		ranUe.Log.Debug("Send PDUSessionResourceNotifyReleasedTransfer to SMF")
		for _, item := range pDUSessionResourceReleasedListNot.List {
			pduSessionID := int32(item.PDUSessionID.Value)
			transfer := item.PDUSessionResourceNotifyReleasedTransfer
			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				ranUe.Log.Error("SmContext not found", zap.Int32("PduSessionID", pduSessionID))
			}
			response, err := consumer.SendUpdateSmContextN2Info(ctx, amfUe, smContext, models.N2SmInfoTypePduResNtyRel, transfer)
			if err != nil {
				ranUe.Log.Error("SendUpdateSmContextN2Info[PDUSessionResourceNotifyReleasedTransfer] Error", zap.Error(err))
			}
			if response != nil {
				responseData := response.JSONData
				n2Info := response.BinaryDataN1SmMessage
				n1Msg := response.BinaryDataN2SmInformation
				buildAndSendN1N2Msg(ctx, ranUe, n1Msg, n2Info, responseData.N2SmInfoType, pduSessionID)
			} else if err != nil {
				return
			} else {
				ranUe.Log.Error("Failed to Update smContext", zap.Int32("PduSessionID", pduSessionID), zap.Error(err))
				return
			}
		}
	}
}

func buildAndSendN1N2Msg(ctx ctxt.Context, ranUe *context.RanUe, n1Msg, n2Info []byte, N2SmInfoType models.N2SmInfoType, pduSessID int32) {
	if n2Info != nil {
		switch N2SmInfoType {
		case models.N2SmInfoTypePduResRelCmd:
			ranUe.Log.Debug("AMF Transfer NGAP PDU Session Resource Rel Co from SMF")
			var nasPdu []byte
			if n1Msg != nil {
				pduSessionID := uint8(pduSessID)
				var err error
				nasPdu, err = gmm_message.BuildDLNASTransport(ranUe.AmfUe, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
				if err != nil {
					ranUe.Log.Warn("error building NAS transport message", zap.Error(err))
				}
			}
			list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
			message.AppendPDUSessionResourceToReleaseListRelCmd(&list, pduSessID, n2Info)
			err := message.SendPDUSessionResourceReleaseCommand(ctx, ranUe, nasPdu, list)
			if err != nil {
				ranUe.Log.Error("error sending pdu session resource release command", zap.Error(err))
				return
			}
			ranUe.Log.Info("sent pdu session resource release command")
		}
	}
}
