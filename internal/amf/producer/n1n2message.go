// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	gmm_message "github.com/ellanetworks/core/internal/amf/gmm/message"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/producer")

func CreateN1N2MessageTransfer(ctx ctxt.Context, ueContextID string, n1n2MessageTransferRequest models.N1N2MessageTransferRequest) (models.N1N2MessageTransferCause, error) {
	ctx, span := tracer.Start(ctx, "AMF N1N2 MessageTransfer")
	defer span.End()

	span.SetAttributes(
		attribute.String("amf.ue_context_id", ueContextID),
	)

	amfSelf := context.AMFSelf()
	if _, ok := amfSelf.AmfUeFindByUeContextID(ueContextID); !ok {
		return "", fmt.Errorf("ue context not found")
	}

	cause, err := N1N2MessageTransferProcedure(ctx, ueContextID, n1n2MessageTransferRequest)
	if err != nil {
		return "", fmt.Errorf("n1 n2 message transfer error: %v", err)
	}

	if cause == "" {
		return "", fmt.Errorf("unspecified error")
	}

	switch cause {
	case models.N1N2MessageTransferCauseN1MsgNotTransferred:
		fallthrough
	case models.N1N2MessageTransferCauseN1N2TransferInitiated:
		return cause, nil
	case models.N1N2MessageTransferCauseAttemptingToReachUE:
		return cause, nil
	default:
		return cause, fmt.Errorf("unsupported cause: %v", cause)
	}
}

// There are 4 possible return value for this function:
//   - n1n2MessageTransferRspData: if AMF handle N1N2MessageTransfer Request successfully.
//   - TransferErr: if AMF reject the request due to procedure error, e.g. UE has an ongoing procedure.
//   - error: if AMF reject the request due to application error, e.g. UE context not found.

// see TS 29.518 6.1.3.5.3.1 for more details.
func N1N2MessageTransferProcedure(ctx ctxt.Context, ueContextID string, n1n2MessageTransferRequest models.N1N2MessageTransferRequest) (models.N1N2MessageTransferCause, error) {
	var (
		requestData = n1n2MessageTransferRequest.JSONData
		n2Info      = n1n2MessageTransferRequest.BinaryDataN2Information
		n1Msg       = n1n2MessageTransferRequest.BinaryDataN1Message
		ue          *context.AmfUe
		ok          bool
		smContext   *context.SmContext
		n1MsgType   uint8
	)

	amfSelf := context.AMFSelf()

	if ue, ok = amfSelf.AmfUeFindByUeContextID(ueContextID); !ok {
		return "", fmt.Errorf("ue context not found")
	}

	switch requestData.N1MessageClass {
	case models.N1MessageClassSM:
		n1MsgType = nasMessage.PayloadContainerTypeN1SMInfo
		smContext, ok = ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
		if !ok {
			return "", fmt.Errorf("sm context not found")
		}
	case models.N1MessageClassSMS:
		n1MsgType = nasMessage.PayloadContainerTypeSMS
	case models.N1MessageClassLPP:
		n1MsgType = nasMessage.PayloadContainerTypeLPP
	case models.N1MessageClassUPDP:
		n1MsgType = nasMessage.PayloadContainerTypeUEPolicy
	default:
	}

	if requestData.N2InfoContainer != nil {
		switch requestData.N2InfoContainer.N2InformationClass {
		case models.N2InformationClassSM:
			ue.ProducerLog.Debug("Receive N2 SM Message", zap.Int32("PDUSessionID", requestData.PduSessionID))
			if smContext == nil {
				_, ok = ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
				if !ok {
					return "", fmt.Errorf("sm context not found")
				}
			}
		default:
			return "", fmt.Errorf("n2 information type [%s] is not supported", requestData.N2InfoContainer.N2InformationClass)
		}
	}

	onGoing := ue.GetOnGoing()
	// 4xx response cases
	switch onGoing.Procedure {
	case context.OnGoingProcedurePaging:
		if requestData.Ppi == 0 || (onGoing.Ppi != 0 && onGoing.Ppi <= requestData.Ppi) {
			return "", fmt.Errorf("higher priority request ongoing")
		}
		ue.T3513.Stop()
	case context.OnGoingProcedureRegistration:
		return "", fmt.Errorf("temporary reject registration ongoing")
	case context.OnGoingProcedureN2Handover:
		return "", fmt.Errorf("temporary reject handover ongoing")
	}

	// var n1n2MessageTransferRspData *models.N1N2MessageTransferRspData
	// UE is CM-Connected
	if ue.CmConnect() {
		var (
			nasPdu []byte
			err    error
		)
		if n1Msg != nil {
			nasPdu, err = gmm_message.BuildDLNASTransport(ue, n1MsgType, n1Msg, uint8(requestData.PduSessionID), nil)
			if err != nil {
				return "", fmt.Errorf("build DL NAS Transport error: %v", err)
			}
			if n2Info == nil {
				ue.ProducerLog.Debug("Forward N1 Message to UE")
				err := ngap_message.SendDownlinkNasTransport(ctx, ue.RanUe, nasPdu, nil)
				if err != nil {
					return "", fmt.Errorf("send downlink nas transport error: %v", err)
				}
				ue.ProducerLog.Info("sent downlink nas transport to UE")
				return models.N1N2MessageTransferCauseN1N2TransferInitiated, nil
			}
		}

		if n2Info != nil {
			smInfo := requestData.N2InfoContainer.SmInfo
			switch smInfo.NgapIeType {
			case models.NgapIeTypePduResSetupReq:
				ue.ProducerLog.Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")
				omecSnssai := models.Snssai{
					Sst: smInfo.SNssai.Sst,
					Sd:  smInfo.SNssai.Sd,
				}
				if ue.RanUe.SentInitialContextSetupRequest {
					list := ngapType.PDUSessionResourceSetupListSUReq{}
					ngap_message.AppendPDUSessionResourceSetupListSUReq(&list, smInfo.PduSessionID, omecSnssai, nasPdu, n2Info)
					err := ngap_message.SendPDUSessionResourceSetupRequest(ctx, ue.RanUe, nil, list)
					if err != nil {
						return "", fmt.Errorf("send pdu session resource setup request error: %v", err)
					}
					ue.ProducerLog.Info("Sent NGAP pdu session resource setup request to UE")
				} else {
					operatorInfo, err := context.GetOperatorInfo(ctx)
					if err != nil {
						return "", fmt.Errorf("error getting operator info: %v", err)
					}
					list := ngapType.PDUSessionResourceSetupListCxtReq{}
					ngap_message.AppendPDUSessionResourceSetupListCxtReq(&list, smInfo.PduSessionID, omecSnssai, nasPdu, n2Info)
					err = ngap_message.SendInitialContextSetupRequest(ctx, ue, nil, &list, nil, nil, nil, operatorInfo.Guami)
					if err != nil {
						return "", fmt.Errorf("send initial context setup request error: %v", err)
					}
					ue.ProducerLog.Info("Sent NGAP initial context setup request to UE")
					ue.RanUe.SentInitialContextSetupRequest = true
				}
				// context.StoreContextInDB(ue)
				return models.N1N2MessageTransferCauseN1N2TransferInitiated, nil
			case models.NgapIeTypePduResModReq:
				ue.ProducerLog.Debug("AMF Transfer NGAP PDU Session Resource Modify Request from SMF")
				list := ngapType.PDUSessionResourceModifyListModReq{}
				ngap_message.AppendPDUSessionResourceModifyListModReq(&list, smInfo.PduSessionID, nasPdu, n2Info)
				err := ngap_message.SendPDUSessionResourceModifyRequest(ctx, ue.RanUe, list)
				if err != nil {
					return "", fmt.Errorf("send pdu session resource modify request error: %v", err)
				}
				ue.ProducerLog.Info("sent pdu session resource modify request to UE")
				// context.StoreContextInDB(ue)
				return models.N1N2MessageTransferCauseN1N2TransferInitiated, nil
			case models.NgapIeTypePduResRelCmd:
				ue.ProducerLog.Debug("AMF Transfer NGAP PDU Session Resource Release Command from SMF")
				list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
				ngap_message.AppendPDUSessionResourceToReleaseListRelCmd(&list, smInfo.PduSessionID, n2Info)
				err := ngap_message.SendPDUSessionResourceReleaseCommand(ctx, ue.RanUe, nasPdu, list)
				if err != nil {
					return "", fmt.Errorf("send pdu session resource release command error: %v", err)
				}
				ue.ProducerLog.Info("sent pdu session resource release command to UE")
				// context.StoreContextInDB(ue)
				return models.N1N2MessageTransferCauseN1N2TransferInitiated, nil
			default:
				return "", fmt.Errorf("ngap ie type [%s] is not supported for SmInfo", smInfo.NgapIeType)
			}
		}
	}

	// UE is CM-IDLE

	// 409: transfer a N2 PDU Session Resource Release Command to a 5G-AN and if the UE is in CM-IDLE
	if n2Info != nil && requestData.N2InfoContainer.SmInfo.NgapIeType == models.NgapIeTypePduResRelCmd {
		return "", fmt.Errorf("ue in cm idle state")
	}
	// 504: the UE in MICO mode or the UE is only registered over Non-3GPP access and its state is CM-IDLE
	if !ue.State.Is(context.Registered) {
		return "", fmt.Errorf("ue not reachable")
	}

	// n1n2MessageTransferRspData = new(models.N1N2MessageTransferRspData)
	var cause models.N1N2MessageTransferCause

	var pagingPriority *ngapType.PagingPriority

	// Case A (UE is CM-IDLE in 3GPP access and the associated access type is 3GPP access)
	// in subclause 5.2.2.3.1.2 of TS29518
	cause = models.N1N2MessageTransferCauseAttemptingToReachUE
	ue.N1N2Message = &n1n2MessageTransferRequest
	ue.SetOnGoing(&context.OnGoingProcedureWithPrio{
		Procedure: context.OnGoingProcedurePaging,
		Ppi:       requestData.Ppi,
	})

	if onGoing.Ppi != 0 {
		pagingPriority = new(ngapType.PagingPriority)
		pagingPriority.Value = aper.Enumerated(onGoing.Ppi)
	}
	pkg, err := ngap_message.BuildPaging(ue, pagingPriority, false)
	if err != nil {
		return cause, fmt.Errorf("build paging error: %v", err)
	}
	err = ngap_message.SendPaging(ctx, ue, pkg)
	if err != nil {
		return cause, fmt.Errorf("send paging error: %v", err)
	}
	return cause, nil
}
