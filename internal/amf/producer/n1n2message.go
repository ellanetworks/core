// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/amf/producer")

func TransferN1N2Message(ctx context.Context, amfInstance *amf.AMF, supi etsi.SUPI, req models.N1N2MessageTransferRequest) error {
	ctx, span := tracer.Start(
		ctx,
		"AMF N1N2 MessageTransfer",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
		),
	)
	defer span.End()

	ue, ok := amfInstance.FindAMFUEBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	nasPdu, err := message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, req.BinaryDataN1Message, req.PduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	ue.Log.Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")

	if ranUe.SentInitialContextSetupRequest {
		list := ngapType.PDUSessionResourceSetupListSUReq{}

		send.AppendPDUSessionResourceSetupListSUReq(&list, req.PduSessionID, req.SNssai, nasPdu, req.BinaryDataN2Information)

		err := ranUe.SendPDUSessionResourceSetupRequest(ctx, ue.Ambr.Uplink, ue.Ambr.Downlink, nil, list)
		if err != nil {
			return fmt.Errorf("send pdu session resource setup request error: %v", err)
		}

		ue.Log.Info("Sent NGAP pdu session resource setup request to UE")

		return nil
	}

	operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	list := ngapType.PDUSessionResourceSetupListCxtReq{}

	send.AppendPDUSessionResourceSetupListCxtReq(&list, req.PduSessionID, req.SNssai, nasPdu, req.BinaryDataN2Information)

	err = ranUe.SendInitialContextSetupRequest(
		ctx,
		ue.Ambr.Uplink,
		ue.Ambr.Downlink,
		ue.AllowedNssai,
		ue.Kgnb,
		ue.PlmnID,
		ue.UeRadioCapability,
		ue.UeRadioCapabilityForPaging,
		ue.UESecurityCapability,
		nil,
		&list,
		operatorInfo.Guami,
	)
	if err != nil {
		return fmt.Errorf("send initial context setup request error: %v", err)
	}

	ue.Log.Info("Sent NGAP initial context setup request to UE")

	ranUe.SentInitialContextSetupRequest = true

	return nil
}

func N2MessageTransferOrPage(ctx context.Context, amfInstance *amf.AMF, supi etsi.SUPI, req models.N1N2MessageTransferRequest) error {
	ctx, span := tracer.Start(
		ctx,
		"AMF N1N2 MessageTransfer",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
		),
	)
	defer span.End()

	ue, ok := amfInstance.FindAMFUEBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	onGoing := ue.GetOnGoing()
	switch onGoing {
	case amf.OnGoingProcedurePaging:
		return fmt.Errorf("higher priority request ongoing")
	case amf.OnGoingProcedureRegistration:
		return fmt.Errorf("temporary reject registration ongoing")
	case amf.OnGoingProcedureN2Handover:
		return fmt.Errorf("temporary reject handover ongoing")
	}

	ranUe := ue.RanUe()
	if ranUe != nil {
		ue.Log.Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")

		if ranUe.SentInitialContextSetupRequest {
			list := ngapType.PDUSessionResourceSetupListSUReq{}
			send.AppendPDUSessionResourceSetupListSUReq(&list, req.PduSessionID, req.SNssai, nil, req.BinaryDataN2Information)

			err := ranUe.SendPDUSessionResourceSetupRequest(ctx, ue.Ambr.Uplink, ue.Ambr.Downlink, nil, list)
			if err != nil {
				return fmt.Errorf("send pdu session resource setup request error: %v", err)
			}

			ue.Log.Info("Sent NGAP pdu session resource setup request to UE")

			return nil
		}

		operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
		if err != nil {
			return fmt.Errorf("error getting operator info: %v", err)
		}

		list := ngapType.PDUSessionResourceSetupListCxtReq{}
		send.AppendPDUSessionResourceSetupListCxtReq(&list, req.PduSessionID, req.SNssai, nil, req.BinaryDataN2Information)

		err = ranUe.SendInitialContextSetupRequest(
			ctx,
			ue.Ambr.Uplink,
			ue.Ambr.Downlink,
			ue.AllowedNssai,
			ue.Kgnb,
			ue.PlmnID,
			ue.UeRadioCapability,
			ue.UeRadioCapabilityForPaging,
			ue.UESecurityCapability,
			nil,
			&list,
			operatorInfo.Guami,
		)
		if err != nil {
			return fmt.Errorf("send initial context setup request error: %v", err)
		}

		ue.Log.Info("Sent NGAP initial context setup request to UE")

		ranUe.SentInitialContextSetupRequest = true

		return nil
	}

	// 504: the UE in MICO mode or the UE is only registered over Non-3GPP access and its state is CM-IDLE
	if ue.GetState() != amf.Registered {
		return fmt.Errorf("ue is not in registered state")
	}

	var pagingPriority *ngapType.PagingPriority

	ue.N1N2Message = &req
	ue.SetOnGoing(amf.OnGoingProcedurePaging)

	pkg, err := send.BuildPaging(
		ue.Guti,
		ue.RegistrationArea,
		ue.UeRadioCapabilityForPaging,
		pagingPriority,
	)
	if err != nil {
		return fmt.Errorf("build paging error: %v", err)
	}

	err = amfInstance.SendPaging(ctx, ue, pkg)
	if err != nil {
		return fmt.Errorf("send paging error: %v", err)
	}

	return nil
}

func TransferN1Msg(ctx context.Context, amfInstance *amf.AMF, supi etsi.SUPI, n1Msg []byte, pduSessionID uint8) error {
	ctx, span := tracer.Start(
		ctx,
		"AMF N1N2 MessageTransfer",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
		),
	)
	defer span.End()

	ue, ok := amfInstance.FindAMFUEBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	nasPdu, err := message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	err = ranUe.SendDownlinkNasTransport(ctx, nasPdu, nil)
	if err != nil {
		return fmt.Errorf("send downlink nas transport error: %v", err)
	}

	ue.Log.Info("sent downlink nas transport to UE", logger.SUPI(supi.String()))

	return nil
}
