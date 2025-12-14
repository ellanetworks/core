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
	gmm_message "github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer("ella-core/amf/producer")

// see TS 29.518 6.1.3.5.3.1 for more details.
func N1N2MessageTransferProcedure(ctx ctxt.Context, supi string, n1n2MessageTransferRequest models.N1N2MessageTransferRequest) error {
	ctx, span := tracer.Start(ctx, "AMF N1N2 MessageTransfer")
	defer span.End()

	span.SetAttributes(
		attribute.String("supi", supi),
	)

	amfSelf := context.AMFSelf()

	ue, ok := amfSelf.AmfUeFindBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	requestData := n1n2MessageTransferRequest.JSONData

	_, ok = ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
	if !ok {
		return fmt.Errorf("sm context not found")
	}

	onGoing := ue.GetOnGoing()
	// 4xx response cases
	switch onGoing.Procedure {
	case context.OnGoingProcedurePaging:
		return fmt.Errorf("higher priority request ongoing")
	case context.OnGoingProcedureRegistration:
		return fmt.Errorf("temporary reject registration ongoing")
	case context.OnGoingProcedureN2Handover:
		return fmt.Errorf("temporary reject handover ongoing")
	}

	n1Msg := n1n2MessageTransferRequest.BinaryDataN1Message
	n2Info := n1n2MessageTransferRequest.BinaryDataN2Information
	// var n1n2MessageTransferRspData *models.N1N2MessageTransferRspData
	// UE is CM-Connected
	if ue.CmConnect() {
		var (
			nasPdu []byte
			err    error
		)
		if n1Msg != nil {
			nasPdu, err = gmm_message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, uint8(requestData.PduSessionID), nil)
			if err != nil {
				return fmt.Errorf("build DL NAS Transport error: %v", err)
			}
			if n2Info == nil {
				ue.Log.Debug("Forward N1 Message to UE")
				err := message.SendDownlinkNasTransport(ctx, ue.RanUe, nasPdu, nil)
				if err != nil {
					return fmt.Errorf("send downlink nas transport error: %v", err)
				}
				ue.Log.Info("sent downlink nas transport to UE")
				return nil
			}
		}

		if n2Info != nil {
			switch requestData.NgapIeType {
			case models.N2SmInfoTypePduResSetupReq:
				ue.Log.Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")
				if ue.RanUe.SentInitialContextSetupRequest {
					list := ngapType.PDUSessionResourceSetupListSUReq{}
					message.AppendPDUSessionResourceSetupListSUReq(&list, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
					err := message.SendPDUSessionResourceSetupRequest(ctx, ue.RanUe, nil, list)
					if err != nil {
						return fmt.Errorf("send pdu session resource setup request error: %v", err)
					}
					ue.Log.Info("Sent NGAP pdu session resource setup request to UE")
				} else {
					operatorInfo, err := context.GetOperatorInfo(ctx)
					if err != nil {
						return fmt.Errorf("error getting operator info: %v", err)
					}
					list := ngapType.PDUSessionResourceSetupListCxtReq{}
					message.AppendPDUSessionResourceSetupListCxtReq(&list, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
					err = message.SendInitialContextSetupRequest(ctx, ue, nil, &list, nil, nil, nil, operatorInfo.Guami)
					if err != nil {
						return fmt.Errorf("send initial context setup request error: %v", err)
					}
					ue.Log.Info("Sent NGAP initial context setup request to UE")
					ue.RanUe.SentInitialContextSetupRequest = true
				}
				// context.StoreContextInDB(ue)
				return nil
			case models.N2SmInfoTypePduResModReq:
				ue.Log.Debug("AMF Transfer NGAP PDU Session Resource Modify Request from SMF")
				list := ngapType.PDUSessionResourceModifyListModReq{}
				message.AppendPDUSessionResourceModifyListModReq(&list, requestData.PduSessionID, nasPdu, n2Info)
				err := message.SendPDUSessionResourceModifyRequest(ctx, ue.RanUe, list)
				if err != nil {
					return fmt.Errorf("send pdu session resource modify request error: %v", err)
				}
				ue.Log.Info("sent pdu session resource modify request to UE")
				// context.StoreContextInDB(ue)
				return nil
			case models.N2SmInfoTypePduResRelCmd:
				ue.Log.Debug("AMF Transfer NGAP PDU Session Resource Release Command from SMF")
				list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
				message.AppendPDUSessionResourceToReleaseListRelCmd(&list, requestData.PduSessionID, n2Info)
				err := message.SendPDUSessionResourceReleaseCommand(ctx, ue.RanUe, nasPdu, list)
				if err != nil {
					return fmt.Errorf("send pdu session resource release command error: %v", err)
				}
				ue.Log.Info("sent pdu session resource release command to UE")
				return nil
			default:
				return fmt.Errorf("ngap ie type [%s] is not supported for SmInfo", requestData.NgapIeType)
			}
		}
	}

	// UE is CM-IDLE

	// 409: transfer a N2 PDU Session Resource Release Command to a 5G-AN and if the UE is in CM-IDLE
	if n2Info != nil && requestData.NgapIeType == models.N2SmInfoTypePduResRelCmd {
		return fmt.Errorf("ue in cm idle state")
	}
	// 504: the UE in MICO mode or the UE is only registered over Non-3GPP access and its state is CM-IDLE
	if !ue.State.Is(context.Registered) {
		return fmt.Errorf("ue not reachable")
	}

	var pagingPriority *ngapType.PagingPriority

	// Case A (UE is CM-IDLE in 3GPP access and the associated access type is 3GPP access)
	// in subclause 5.2.2.3.1.2 of TS29518

	ue.N1N2Message = &n1n2MessageTransferRequest
	ue.SetOnGoing(&context.OnGoingProcedureWithPrio{
		Procedure: context.OnGoingProcedurePaging,
	})

	pkg, err := message.BuildPaging(ue, pagingPriority, false)
	if err != nil {
		return fmt.Errorf("build paging error: %v", err)
	}

	err = message.SendPaging(ctx, ue, pkg)
	if err != nil {
		return fmt.Errorf("send paging error: %v", err)
	}

	return nil
}
