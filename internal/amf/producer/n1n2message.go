// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/amf/context"
	gmm_message "github.com/ellanetworks/core/internal/amf/gmm/message"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/aper"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/ngap/ngapType"
	"github.com/omec-project/openapi/models"
)

func CreateN1N2MessageTransfer(ueContextId string, n1n2MessageTransferRequest models.N1N2MessageTransferRequest, reqUri string) (*models.N1N2MessageTransferRspData, error) {
	logger.AmfLog.Infof("Handle N1N2 Message Transfer Request")
	amfSelf := context.AMF_Self()
	if _, ok := amfSelf.AmfUeFindByUeContextID(ueContextId); !ok {
		return nil, fmt.Errorf("UE context not found")
	}
	respData, problemDetails, transferErr := N1N2MessageTransferProcedure(ueContextId, reqUri, n1n2MessageTransferRequest)
	if problemDetails != nil {
		return nil, fmt.Errorf("problem Details: %v", problemDetails)
	} else if transferErr != nil {
		return nil, fmt.Errorf("transfer Error: %v", transferErr)
	} else if respData != nil {
		switch respData.Cause {
		case models.N1N2MessageTransferCause_N1_MSG_NOT_TRANSFERRED:
			fallthrough
		case models.N1N2MessageTransferCause_N1_N2_TRANSFER_INITIATED:
			return respData, nil
		case models.N1N2MessageTransferCause_ATTEMPTING_TO_REACH_UE:
			return respData, nil
		}
	}

	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return nil, fmt.Errorf("problem Details: %v", problemDetails)
}

// There are 4 possible return value for this function:
//   - n1n2MessageTransferRspData: if AMF handle N1N2MessageTransfer Request successfully.
//   - problemDetails: if AMF reject the request due to application error, e.g. UE context not found.
//   - TransferErr: if AMF reject the request due to procedure error, e.g. UE has an ongoing procedure.

// see TS 29.518 6.1.3.5.3.1 for more details.
func N1N2MessageTransferProcedure(ueContextID string, reqUri string,
	n1n2MessageTransferRequest models.N1N2MessageTransferRequest) (
	n1n2MessageTransferRspData *models.N1N2MessageTransferRspData,
	problemDetails *models.ProblemDetails,
	transferErr *models.N1N2MessageTransferError,
) {
	var (
		requestData *models.N1N2MessageTransferReqData = n1n2MessageTransferRequest.JsonData
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
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return nil, problemDetails, nil
	}

	if requestData.N1MessageContainer != nil {
		switch requestData.N1MessageContainer.N1MessageClass {
		case models.N1MessageClass_SM:
			ue.ProducerLog.Debugf("Receive N1 SM Message (PDU Session ID: %d)", requestData.PduSessionId)
			n1MsgType = nasMessage.PayloadContainerTypeN1SMInfo
			if smContext, ok = ue.SmContextFindByPDUSessionID(requestData.PduSessionId); !ok {
				problemDetails = &models.ProblemDetails{
					Status: http.StatusNotFound,
					Cause:  "CONTEXT_NOT_FOUND",
				}
				return nil, problemDetails, nil
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
		case models.N2InformationClass_SM:
			ue.ProducerLog.Debugf("Receive N2 SM Message (PDU Session ID: %d)", requestData.PduSessionId)
			if smContext == nil {
				if smContext, ok = ue.SmContextFindByPDUSessionID(requestData.PduSessionId); !ok {
					problemDetails = &models.ProblemDetails{
						Status: http.StatusNotFound,
						Cause:  "CONTEXT_NOT_FOUND",
					}
					return nil, problemDetails, nil
				} else {
					anType = smContext.AccessType()
				}
			}
		default:
			ue.ProducerLog.Warnf("N2 Information type [%s] is not supported", requestData.N2InfoContainer.N2InformationClass)
			problemDetails = &models.ProblemDetails{
				Status: http.StatusNotImplemented,
				Cause:  "NOT_IMPLEMENTED",
			}
			return nil, problemDetails, nil
		}
	}

	onGoing := ue.GetOnGoing(anType)
	// 4xx response cases
	switch onGoing.Procedure {
	case context.OnGoingProcedurePaging:
		if requestData.Ppi == 0 || (onGoing.Ppi != 0 && onGoing.Ppi <= requestData.Ppi) {
			transferErr = new(models.N1N2MessageTransferError)
			transferErr.Error = &models.ProblemDetails{
				Status: http.StatusConflict,
				Cause:  "HIGHER_PRIORITY_REQUEST_ONGOING",
			}
			return nil, nil, transferErr
		}
		ue.T3513.Stop()
	case context.OnGoingProcedureRegistration:
		transferErr = new(models.N1N2MessageTransferError)
		transferErr.Error = &models.ProblemDetails{
			Status: http.StatusConflict,
			Cause:  "TEMPORARY_REJECT_REGISTRATION_ONGOING",
		}
		return nil, nil, transferErr
	case context.OnGoingProcedureN2Handover:
		transferErr = new(models.N1N2MessageTransferError)
		transferErr.Error = &models.ProblemDetails{
			Status: http.StatusConflict,
			Cause:  "TEMPORARY_REJECT_HANDOVER_ONGOING",
		}
		return nil, nil, transferErr
	}

	// UE is CM-Connected
	if ue.CmConnect(anType) {
		var (
			nasPdu []byte
			err    error
		)
		if n1Msg != nil {
			nasPdu, err = gmm_message.BuildDLNASTransport(ue, n1MsgType, n1Msg, uint8(requestData.PduSessionId), nil, nil, 0)
			if err != nil {
				ue.ProducerLog.Errorf("Build DL NAS Transport error: %+v", err)
				problemDetails = &models.ProblemDetails{
					Title:  "System failure",
					Status: http.StatusInternalServerError,
					Detail: err.Error(),
					Cause:  "SYSTEM_FAILURE",
				}
				return nil, problemDetails, nil
			}
			if n2Info == nil {
				ue.ProducerLog.Debug("Forward N1 Message to UE")
				ngap_message.SendDownlinkNasTransport(ue.RanUe[anType], nasPdu, nil)
				n1n2MessageTransferRspData = new(models.N1N2MessageTransferRspData)
				n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCause_N1_N2_TRANSFER_INITIATED
				return n1n2MessageTransferRspData, nil, nil
			}
		}

		if n2Info != nil {
			smInfo := requestData.N2InfoContainer.SmInfo
			switch smInfo.N2InfoContent.NgapIeType {
			case models.NgapIeType_PDU_RES_SETUP_REQ:
				ue.ProducerLog.Debugln("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")
				if ue.RanUe[anType].SentInitialContextSetupRequest {
					list := ngapType.PDUSessionResourceSetupListSUReq{}
					ngap_message.AppendPDUSessionResourceSetupListSUReq(&list, smInfo.PduSessionId, *smInfo.SNssai, nasPdu, n2Info)
					ngap_message.SendPDUSessionResourceSetupRequest(ue.RanUe[anType], nil, list)
				} else {
					list := ngapType.PDUSessionResourceSetupListCxtReq{}
					ngap_message.AppendPDUSessionResourceSetupListCxtReq(&list, smInfo.PduSessionId, *smInfo.SNssai, nasPdu, n2Info)
					ngap_message.SendInitialContextSetupRequest(ue, anType, nil, &list, nil, nil, nil)
					ue.RanUe[anType].SentInitialContextSetupRequest = true
				}
				n1n2MessageTransferRspData = new(models.N1N2MessageTransferRspData)
				n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCause_N1_N2_TRANSFER_INITIATED
				// context.StoreContextInDB(ue)
				return n1n2MessageTransferRspData, nil, nil
			case models.NgapIeType_PDU_RES_MOD_REQ:
				ue.ProducerLog.Debugln("AMF Transfer NGAP PDU Session Resource Modify Request from SMF")
				list := ngapType.PDUSessionResourceModifyListModReq{}
				ngap_message.AppendPDUSessionResourceModifyListModReq(&list, smInfo.PduSessionId, nasPdu, n2Info)
				ngap_message.SendPDUSessionResourceModifyRequest(ue.RanUe[anType], list)
				n1n2MessageTransferRspData = new(models.N1N2MessageTransferRspData)
				n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCause_N1_N2_TRANSFER_INITIATED
				// context.StoreContextInDB(ue)
				return n1n2MessageTransferRspData, nil, nil
			case models.NgapIeType_PDU_RES_REL_CMD:
				ue.ProducerLog.Debugln("AMF Transfer NGAP PDU Session Resource Release Command from SMF")
				list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
				ngap_message.AppendPDUSessionResourceToReleaseListRelCmd(&list, smInfo.PduSessionId, n2Info)
				ngap_message.SendPDUSessionResourceReleaseCommand(ue.RanUe[anType], nasPdu, list)
				n1n2MessageTransferRspData = new(models.N1N2MessageTransferRspData)
				n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCause_N1_N2_TRANSFER_INITIATED
				// context.StoreContextInDB(ue)
				return n1n2MessageTransferRspData, nil, nil
			default:
				ue.ProducerLog.Errorf("NGAP IE Type[%s] is not supported for SmInfo", smInfo.N2InfoContent.NgapIeType)
				problemDetails = &models.ProblemDetails{
					Status: http.StatusForbidden,
					Cause:  "UNSPECIFIED",
				}
				return nil, problemDetails, nil
			}
		}
	}

	// UE is CM-IDLE

	// 409: transfer a N2 PDU Session Resource Release Command to a 5G-AN and if the UE is in CM-IDLE
	if n2Info != nil && requestData.N2InfoContainer.SmInfo.N2InfoContent.NgapIeType == models.NgapIeType_PDU_RES_REL_CMD {
		transferErr = new(models.N1N2MessageTransferError)
		transferErr.Error = &models.ProblemDetails{
			Status: http.StatusConflict,
			Cause:  "UE_IN_CM_IDLE_STATE",
		}
		return nil, nil, transferErr
	}
	// 504: the UE in MICO mode or the UE is only registered over Non-3GPP access and its state is CM-IDLE
	if !ue.State[models.AccessType__3_GPP_ACCESS].Is(context.Registered) {
		transferErr = new(models.N1N2MessageTransferError)
		transferErr.Error = &models.ProblemDetails{
			Status: http.StatusGatewayTimeout,
			Cause:  "UE_NOT_REACHABLE",
		}
		return nil, nil, transferErr
	}

	n1n2MessageTransferRspData = new(models.N1N2MessageTransferRspData)

	var pagingPriority *ngapType.PagingPriority

	if _, err := ue.N1N2MessageIDGenerator.Allocate(); err != nil {
		ue.ProducerLog.Errorf("Allocate n1n2MessageID error: %+v", err)
		problemDetails = &models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		return n1n2MessageTransferRspData, problemDetails, transferErr
	}

	// Case A (UE is CM-IDLE in 3GPP access and the associated access type is 3GPP access)
	// in subclause 5.2.2.3.1.2 of TS29518
	if anType == models.AccessType__3_GPP_ACCESS {
		if requestData.SkipInd && n2Info == nil {
			n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCause_N1_MSG_NOT_TRANSFERRED
		} else {
			n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCause_ATTEMPTING_TO_REACH_UE
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
				logger.AmfLog.Errorf("Build Paging failed : %s", err.Error())
				return n1n2MessageTransferRspData, problemDetails, transferErr
			}
			ngap_message.SendPaging(ue, pkg)
		}
		return n1n2MessageTransferRspData, nil, nil
	} else {
		// Case B (UE is CM-IDLE in Non-3GPP access but CM-CONNECTED in 3GPP access and the associated
		// access type is Non-3GPP access)in subclause 5.2.2.3.1.2 of TS29518
		if ue.CmConnect(models.AccessType__3_GPP_ACCESS) {
			if n2Info == nil {
				n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCause_N1_N2_TRANSFER_INITIATED
				gmm_message.SendDLNASTransport(ue.RanUe[models.AccessType__3_GPP_ACCESS],
					nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionId, 0, nil, 0)
			} else {
				n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCause_ATTEMPTING_TO_REACH_UE
				message := context.N1N2Message{
					Request: n1n2MessageTransferRequest,
					Status:  n1n2MessageTransferRspData.Cause,
				}
				ue.N1N2Message = &message
				nasMsg, err := gmm_message.BuildNotification(ue, models.AccessType_NON_3_GPP_ACCESS)
				if err != nil {
					logger.AmfLog.Errorf("Build Notification failed : %s", err.Error())
					return n1n2MessageTransferRspData, problemDetails, transferErr
				}
				gmm_message.SendNotification(ue.RanUe[models.AccessType__3_GPP_ACCESS], nasMsg)
			}
			return n1n2MessageTransferRspData, nil, nil
		} else {
			// Case C ( UE is CM-IDLE in both Non-3GPP access and 3GPP access and the associated access ype is Non-3GPP access)
			// in subclause 5.2.2.3.1.2 of TS29518
			n1n2MessageTransferRspData.Cause = models.N1N2MessageTransferCause_ATTEMPTING_TO_REACH_UE
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
			return n1n2MessageTransferRspData, nil, nil
		}
	}
}

func N1N2MessageTransferStatusProcedure(ueContextID string, reqUri string) (models.N1N2MessageTransferCause,
	*models.ProblemDetails,
) {
	amfSelf := context.AMF_Self()

	ue, ok := amfSelf.AmfUeFindByUeContextID(ueContextID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return "", problemDetails
	}

	n1n2Message := ue.N1N2Message
	if n1n2Message == nil {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return "", problemDetails
	}

	return n1n2Message.Status, nil
}

func N1N2MessageSubscribeProcedure(ueContextID string,
	ueN1N2InfoSubscriptionCreateData models.UeN1N2InfoSubscriptionCreateData) (
	*models.UeN1N2InfoSubscriptionCreatedData, *models.ProblemDetails,
) {
	amfSelf := context.AMF_Self()

	ue, ok := amfSelf.AmfUeFindByUeContextID(ueContextID)
	if !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "CONTEXT_NOT_FOUND",
		}
		return nil, problemDetails
	}

	ueN1N2InfoSubscriptionCreatedData := new(models.UeN1N2InfoSubscriptionCreatedData)

	if newSubscriptionID, err := ue.N1N2MessageSubscribeIDGenerator.Allocate(); err != nil {
		logger.AmfLog.Errorf("Create subscriptionID Error: %+v", err)
		problemDetails := &models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "SYSTEM_FAILURE",
		}
		return nil, problemDetails
	} else {
		ueN1N2InfoSubscriptionCreatedData.N1n2NotifySubscriptionId = strconv.Itoa(int(newSubscriptionID))
		ue.N1N2MessageSubscription.Store(newSubscriptionID, ueN1N2InfoSubscriptionCreateData)
	}
	return ueN1N2InfoSubscriptionCreatedData, nil
}
