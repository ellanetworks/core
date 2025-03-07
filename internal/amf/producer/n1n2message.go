// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	gmm_message "github.com/ellanetworks/core/internal/amf/gmm/message"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/aper"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/ngap/ngapType"
)

func CreateN1N2MessageTransfer(ueContextId string, n1n2MessageTransferRequest models.N1N2MessageTransferRequest, reqUri string) (*models.N1N2MessageTransferRspData, error) {
	amfSelf := context.AMF_Self()
	if _, ok := amfSelf.AmfUeFindByUeContextID(ueContextId); !ok {
		return nil, fmt.Errorf("UE context not found")
	}
	respData, err := N1N2MessageTransferProcedure(ueContextId, reqUri, n1n2MessageTransferRequest)
	if err != nil {
		return nil, fmt.Errorf("n1 n2 message transfer error: %v", err)
	}
	if respData == nil {
		return nil, fmt.Errorf("unspecified error")
	}

	switch respData.Cause {
	case models.N1N2MessageTransferCauseN1MsgNotTransferred:
		fallthrough
	case models.N1N2MessageTransferCauseN1N2TransferInitiated:
		return respData, nil
	case models.N1N2MessageTransferCauseAttemptingToReachUE:
		return respData, nil
	default:
		return nil, fmt.Errorf("unsupported cause: %v", respData.Cause)
	}
}

// There are 4 possible return value for this function:
//   - n1n2MessageTransferRspData: if AMF handle N1N2MessageTransfer Request successfully.
//   - TransferErr: if AMF reject the request due to procedure error, e.g. UE has an ongoing procedure.
//   - error: if AMF reject the request due to application error, e.g. UE context not found.

// see TS 29.518 6.1.3.5.3.1 for more details.
func N1N2MessageTransferProcedure(ueContextID string, reqUri string, n1n2MessageTransferRequest models.N1N2MessageTransferRequest) (*models.N1N2MessageTransferRspData, error) {
	var (
		requestData *models.N1N2MessageTransferReqData = n1n2MessageTransferRequest.JSONData
		n2Info      []byte                             = n1n2MessageTransferRequest.BinaryDataN2Information
		n1Msg       []byte                             = n1n2MessageTransferRequest.BinaryDataN1Message

		ue        *context.AmfUe
		ok        bool
		smContext *context.SmContext
		n1MsgType uint8
		anType    models.AccessType = models.AccessType__3_GPP_ACCESS
	)

	amfSelf := context.AMF_Self()

	if ue, ok = amfSelf.AmfUeFindByUeContextID(ueContextID); !ok {
		return nil, fmt.Errorf("ue context not found")
	}

	if requestData.N1MessageContainer != nil {
		switch requestData.N1MessageContainer.N1MessageClass {
		case models.N1MessageClass_SM:
			ue.ProducerLog.Debugf("Receive N1 SM Message (PDU Session ID: %d)", requestData.PduSessionID)
			n1MsgType = nasMessage.PayloadContainerTypeN1SMInfo
			if smContext, ok = ue.SmContextFindByPDUSessionID(requestData.PduSessionID); !ok {
				return nil, fmt.Errorf("sm context not found")
			} else {
				anType = smContext.AccessType()
			}
		case models.N1MessageClass_SMS:
			n1MsgType = nasMessage.PayloadContainerTypeSMS
		case models.N1MessageClass_LPP:
			n1MsgType = nasMessage.PayloadContainerTypeLPP
		case models.N1MessageClass_UPDP:
			n1MsgType = nasMessage.PayloadContainerTypeUEPolicy
		default:
		}
	}

	if requestData.N2InfoContainer != nil {
		switch requestData.N2InfoContainer.N2InformationClass {
		case models.N2InformationClassSM:
			ue.ProducerLog.Debugf("Receive N2 SM Message (PDU Session ID: %d)", requestData.PduSessionID)
			if smContext == nil {
				if smContext, ok = ue.SmContextFindByPDUSessionID(requestData.PduSessionID); !ok {
					return nil, fmt.Errorf("sm context not found")
				} else {
					anType = smContext.AccessType()
				}
			}
		default:
			return nil, fmt.Errorf("n2 information type [%s] is not supported", requestData.N2InfoContainer.N2InformationClass)
		}
	}

	onGoing := ue.GetOnGoing(anType)
	// 4xx response cases
	switch onGoing.Procedure {
	case context.OnGoingProcedurePaging:
		if requestData.Ppi == 0 || (onGoing.Ppi != 0 && onGoing.Ppi <= requestData.Ppi) {
			return nil, fmt.Errorf("higher priority request ongoing")
		}
		ue.T3513.Stop()
	case context.OnGoingProcedureRegistration:
		return nil, fmt.Errorf("temporary reject registration ongoing")
	case context.OnGoingProcedureN2Handover:
		return nil, fmt.Errorf("temporary reject handover ongoing")
	}

	var n1n2MessageTransferRspData *models.N1N2MessageTransferRspData
	// UE is CM-Connected
	if ue.CmConnect(anType) {
		var (
			nasPdu []byte
			err    error
		)
		if n1Msg != nil {
			nasPdu, err = gmm_message.BuildDLNASTransport(ue, n1MsgType, n1Msg, uint8(requestData.PduSessionID), nil, nil, 0)
			if err != nil {
				return nil, fmt.Errorf("build DL NAS Transport error: %v", err)
			}
			if n2Info == nil {
				ngap_message.SendDownlinkNasTransport(ue.RanUe[anType], nasPdu, nil)
				n1n2MessageTransferRspData = new(models.N1N2MessageTransferRspData)
				n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCauseN1N2TransferInitiated
				return n1n2MessageTransferRspData, nil
			}
		}

		if n2Info != nil {
			smInfo := requestData.N2InfoContainer.SmInfo
			switch smInfo.N2InfoContent.NgapIeType {
			case models.NgapIeType_PDU_RES_SETUP_REQ:
				ue.ProducerLog.Debugln("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")
				omecSnssai := models.Snssai{
					Sst: smInfo.SNssai.Sst,
					Sd:  smInfo.SNssai.Sd,
				}
				if ue.RanUe[anType].SentInitialContextSetupRequest {
					list := ngapType.PDUSessionResourceSetupListSUReq{}

					ngap_message.AppendPDUSessionResourceSetupListSUReq(&list, smInfo.PduSessionID, omecSnssai, nasPdu, n2Info)
					ngap_message.SendPDUSessionResourceSetupRequest(ue.RanUe[anType], nil, list)
				} else {
					list := ngapType.PDUSessionResourceSetupListCxtReq{}
					ngap_message.AppendPDUSessionResourceSetupListCxtReq(&list, smInfo.PduSessionID, omecSnssai, nasPdu, n2Info)
					ngap_message.SendInitialContextSetupRequest(ue, anType, nil, &list, nil, nil, nil)
					ue.RanUe[anType].SentInitialContextSetupRequest = true
				}
				n1n2MessageTransferRspData = new(models.N1N2MessageTransferRspData)
				n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCauseN1N2TransferInitiated
				// context.StoreContextInDB(ue)
				return n1n2MessageTransferRspData, nil
			case models.NgapIeType_PDU_RES_MOD_REQ:
				ue.ProducerLog.Debugln("AMF Transfer NGAP PDU Session Resource Modify Request from SMF")
				list := ngapType.PDUSessionResourceModifyListModReq{}
				ngap_message.AppendPDUSessionResourceModifyListModReq(&list, smInfo.PduSessionID, nasPdu, n2Info)
				ngap_message.SendPDUSessionResourceModifyRequest(ue.RanUe[anType], list)
				n1n2MessageTransferRspData = new(models.N1N2MessageTransferRspData)
				n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCauseN1N2TransferInitiated
				// context.StoreContextInDB(ue)
				return n1n2MessageTransferRspData, nil
			case models.NgapIeType_PDU_RES_REL_CMD:
				ue.ProducerLog.Debugln("AMF Transfer NGAP PDU Session Resource Release Command from SMF")
				list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
				ngap_message.AppendPDUSessionResourceToReleaseListRelCmd(&list, smInfo.PduSessionID, n2Info)
				ngap_message.SendPDUSessionResourceReleaseCommand(ue.RanUe[anType], nasPdu, list)
				n1n2MessageTransferRspData = new(models.N1N2MessageTransferRspData)
				n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCauseN1N2TransferInitiated
				// context.StoreContextInDB(ue)
				return n1n2MessageTransferRspData, nil
			default:
				return nil, fmt.Errorf("ngap ie type [%s] is not supported for SmInfo", smInfo.N2InfoContent.NgapIeType)
			}
		}
	}

	// UE is CM-IDLE

	// 409: transfer a N2 PDU Session Resource Release Command to a 5G-AN and if the UE is in CM-IDLE
	if n2Info != nil && requestData.N2InfoContainer.SmInfo.N2InfoContent.NgapIeType == models.NgapIeType_PDU_RES_REL_CMD {
		return nil, fmt.Errorf("ue in cm idle state")
	}
	// 504: the UE in MICO mode or the UE is only registered over Non-3GPP access and its state is CM-IDLE
	if !ue.State[models.AccessType__3_GPP_ACCESS].Is(context.Registered) {
		return nil, fmt.Errorf("ue not reachable")
	}

	n1n2MessageTransferRspData = new(models.N1N2MessageTransferRspData)

	var pagingPriority *ngapType.PagingPriority

	if _, err := ue.N1N2MessageIDGenerator.Allocate(); err != nil {
		return n1n2MessageTransferRspData, fmt.Errorf("allocate n1n2MessageID error: %v", err)
	}

	// Case A (UE is CM-IDLE in 3GPP access and the associated access type is 3GPP access)
	// in subclause 5.2.2.3.1.2 of TS29518
	if anType == models.AccessType__3_GPP_ACCESS {
		if requestData.SkipInd && n2Info == nil {
			n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCauseN1MsgNotTransferred
		} else {
			n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCauseAttemptingToReachUE
			message := context.N1N2Message{
				Request: n1n2MessageTransferRequest,
				Status:  n1n2MessageTransferRspData.Cause,
			}
			ue.N1N2Message = &message
			ue.SetOnGoing(anType, &context.OnGoingProcedureWithPrio{
				Procedure: context.OnGoingProcedurePaging,
				Ppi:       requestData.Ppi,
			})

			if onGoing.Ppi != 0 {
				pagingPriority = new(ngapType.PagingPriority)
				pagingPriority.Value = aper.Enumerated(onGoing.Ppi)
			}
			pkg, err := ngap_message.BuildPaging(ue, pagingPriority, false)
			if err != nil {
				return n1n2MessageTransferRspData, fmt.Errorf("build paging error: %v", err)
			}
			ngap_message.SendPaging(ue, pkg)
		}
		return n1n2MessageTransferRspData, nil
	} else {
		// Case B (UE is CM-IDLE in Non-3GPP access but CM-CONNECTED in 3GPP access and the associated
		// access type is Non-3GPP access)in subclause 5.2.2.3.1.2 of TS29518
		if ue.CmConnect(models.AccessType__3_GPP_ACCESS) {
			if n2Info == nil {
				n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCauseN1N2TransferInitiated
				gmm_message.SendDLNASTransport(ue.RanUe[models.AccessType__3_GPP_ACCESS],
					nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0, nil, 0)
			} else {
				n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCauseAttemptingToReachUE
				message := context.N1N2Message{
					Request: n1n2MessageTransferRequest,
					Status:  n1n2MessageTransferRspData.Cause,
				}
				ue.N1N2Message = &message
				nasMsg, err := gmm_message.BuildNotification(ue, models.AccessType_NON_3_GPP_ACCESS)
				if err != nil {
					return n1n2MessageTransferRspData, fmt.Errorf("build notification error: %v", err)
				}
				gmm_message.SendNotification(ue.RanUe[models.AccessType__3_GPP_ACCESS], nasMsg)
			}
			return n1n2MessageTransferRspData, nil
		} else {
			// Case C ( UE is CM-IDLE in both Non-3GPP access and 3GPP access and the associated access ype is Non-3GPP access)
			// in subclause 5.2.2.3.1.2 of TS29518
			n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCauseAttemptingToReachUE
			message := context.N1N2Message{
				Request: n1n2MessageTransferRequest,
				Status:  n1n2MessageTransferRspData.Cause,
			}
			ue.N1N2Message = &message

			ue.SetOnGoing(anType, &context.OnGoingProcedureWithPrio{
				Procedure: context.OnGoingProcedurePaging,
				Ppi:       requestData.Ppi,
			})
			if onGoing.Ppi != 0 {
				pagingPriority = new(ngapType.PagingPriority)
				pagingPriority.Value = aper.Enumerated(onGoing.Ppi)
			}
			pkg, err := ngap_message.BuildPaging(ue, pagingPriority, true)
			if err != nil {
				logger.AmfLog.Errorf("Build Paging failed : %s", err.Error())
			}
			ngap_message.SendPaging(ue, pkg)
			return n1n2MessageTransferRspData, nil
		}
	}
}
