// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ErrUENotReachable is returned when the UE is in CM-IDLE state and the
// requested signaling cannot be delivered. Per TS 23.502 §4.2.3.3 step 3b,
// the AMF may ignore the N2 SM information when the UE is not reachable and
// the caller should defer delivery until the UE transitions to CM-CONNECTED.
var ErrUENotReachable = errors.New("UE is in CM-IDLE state")

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
		return storeN1N2AndPage(ctx, amfInstance, ue, req)
	}

	nasPdu, err := message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, req.BinaryDataN1Message, req.PduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	ue.Log.Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")

	if ranUe.ICS != amf.ICSNotStarted {
		list := ngapType.PDUSessionResourceSetupListSUReq{}

		send.AppendPDUSessionResourceSetupListSUReq(&list, req.PduSessionID, req.SNssai, nasPdu, req.BinaryDataN2Information)

		err := ranUe.SendPDUSessionResourceSetupRequest(ctx, ue.Current().Ambr.Uplink, ue.Current().Ambr.Downlink, nil, list)
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
		ue.Current().Ambr.Uplink,
		ue.Current().Ambr.Downlink,
		ue.Current().AllowedNssai,
		ue.Current().Kgnb,
		ue.PlmnID,
		ue.Current().UeRadioCapability,
		ue.Current().UeRadioCapabilityForPaging,
		ue.Current().UESecurityCapability,
		nil,
		&list,
		operatorInfo.Guami,
	)
	if err != nil {
		return fmt.Errorf("send initial context setup request error: %v", err)
	}

	ue.Log.Info("Sent NGAP initial context setup request to UE")

	ranUe.ICS = amf.ICSPending

	return nil
}

func storeN1N2AndPage(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe, req models.N1N2MessageTransferRequest) error {
	nasConn := ue.NasConn()
	if nasConn == nil {
		return fmt.Errorf("ue has no active NAS connection")
	}

	if nasConn.Procedures.Active(procedure.Paging) {
		return fmt.Errorf("higher priority request ongoing")
	}

	if nasConn.Procedures.Active(procedure.Registration) {
		return fmt.Errorf("temporary reject registration ongoing")
	}

	if nasConn.Procedures.Active(procedure.N2Handover) {
		return fmt.Errorf("temporary reject handover ongoing")
	}

	if ue.GetState() != amf.Registered {
		return fmt.Errorf("ue is not in registered state")
	}

	nasConn.N1N2Message = &req

	_, beginErr := nasConn.Procedures.Begin(nasConn.Ctx(), procedure.Procedure{Type: procedure.Paging})
	if beginErr != nil {
		return fmt.Errorf("begin paging procedure: %w", beginErr)
	}

	pkg, err := send.BuildPaging(
		ue.Guti,
		ue.Current().RegistrationArea,
		ue.Current().UeRadioCapabilityForPaging,
		nil,
	)
	if err != nil {
		return fmt.Errorf("build paging error: %v", err)
	}

	if err := amfInstance.SendPaging(ctx, ue, pkg); err != nil {
		return fmt.Errorf("send paging error: %v", err)
	}

	return nil
}

// ModifyN1N2Message sends a PDUSessionResourceModifyRequest to the gNB,
// carrying the N1 NAS PDU (PDU Session Modification Command) piggybacked
// in the per-session modify item, and the N2 transfer (PDU Session Resource
// Modify Request Transfer) as the opaque transfer blob.
// This implements TS 23.502 §4.3.3.2 step 3 (AMF→gNB).
func ModifyN1N2Message(ctx context.Context, amfInstance *amf.AMF, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Msg []byte) error {
	ctx, span := tracer.Start(
		ctx,
		"AMF PDUSessionResourceModifyRequest",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
			attribute.Int("pdu_session_id", int(pduSessionID)),
		),
	)
	defer span.End()

	ue, ok := amfInstance.FindAMFUEBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		// Per TS 23.502 §4.2.3.3 step 3b: when the UE is in CM-IDLE, the AMF
		// may ignore the N2 SM information. Since the gNB has released all
		// radio resources for this session, there is nothing to "modify".
		// The caller should commit the policy change; when the UE transitions
		// back to CM-CONNECTED, ActivateSmContext will build a fresh
		// PDUSessionResourceSetupRequestTransfer with the updated QoS.
		return ErrUENotReachable
	}

	// Build the DL NAS Transport wrapping the N1 SM message.
	nasPdu, err := message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	// Construct the modify list with a single session item.
	list := ngapType.PDUSessionResourceModifyListModReq{
		List: []ngapType.PDUSessionResourceModifyItemModReq{
			{
				PDUSessionID: ngapType.PDUSessionID{Value: int64(pduSessionID)},
				NASPDU: &ngapType.NASPDU{
					Value: nasPdu,
				},
				PDUSessionResourceModifyRequestTransfer: n2Msg,
			},
		},
	}

	if err := ranUe.SendPDUSessionResourceModifyRequest(ctx, list); err != nil {
		return fmt.Errorf("send pdu session resource modify request error: %v", err)
	}

	ue.Log.Info("Sent NGAP PDU Session Resource Modify Request to gNB")

	return nil
}

// ReleaseSessionMessage sends a PDUSessionResourceReleaseCommand to the gNB,
// carrying the N1 NAS PDU (PDU Session Release Command) and the N2 transfer
// (PDU Session Resource Release Command Transfer).
// This implements the network-initiated PDU Session Release (TS 23.502 §4.3.4.2).
func ReleaseSessionMessage(ctx context.Context, amfInstance *amf.AMF, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Transfer []byte) error {
	ctx, span := tracer.Start(
		ctx,
		"AMF PDUSessionResourceReleaseCommand",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
			attribute.Int("pdu_session_id", int(pduSessionID)),
		),
	)
	defer span.End()

	ue, ok := amfInstance.FindAMFUEBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return ErrUENotReachable
	}

	// Build the DL NAS Transport wrapping the N1 SM message.
	nasPdu, err := message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	// Construct the release list with a single session item.
	list := ngapType.PDUSessionResourceToReleaseListRelCmd{
		List: []ngapType.PDUSessionResourceToReleaseItemRelCmd{
			{
				PDUSessionID:                             ngapType.PDUSessionID{Value: int64(pduSessionID)},
				PDUSessionResourceReleaseCommandTransfer: n2Transfer,
			},
		},
	}

	if err := ranUe.SendPDUSessionResourceReleaseCommand(ctx, nasPdu, list); err != nil {
		return fmt.Errorf("send pdu session resource release command error: %v", err)
	}

	ue.Log.Info("Sent NGAP PDU Session Resource Release Command to gNB",
		logger.PDUSessionID(pduSessionID),
	)

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

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("ue has no active NAS connection")
	}

	if conn.Procedures.Active(procedure.Paging) {
		return fmt.Errorf("higher priority request ongoing")
	}

	if conn.Procedures.Active(procedure.Registration) {
		return fmt.Errorf("temporary reject registration ongoing")
	}

	if conn.Procedures.Active(procedure.N2Handover) {
		return fmt.Errorf("temporary reject handover ongoing")
	}

	ranUe := ue.RanUe()
	if ranUe != nil {
		ue.Log.Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")

		if ranUe.ICS != amf.ICSNotStarted {
			list := ngapType.PDUSessionResourceSetupListSUReq{}
			send.AppendPDUSessionResourceSetupListSUReq(&list, req.PduSessionID, req.SNssai, nil, req.BinaryDataN2Information)

			err := ranUe.SendPDUSessionResourceSetupRequest(ctx, ue.Current().Ambr.Uplink, ue.Current().Ambr.Downlink, nil, list)
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
			ue.Current().Ambr.Uplink,
			ue.Current().Ambr.Downlink,
			ue.Current().AllowedNssai,
			ue.Current().Kgnb,
			ue.PlmnID,
			ue.Current().UeRadioCapability,
			ue.Current().UeRadioCapabilityForPaging,
			ue.Current().UESecurityCapability,
			nil,
			&list,
			operatorInfo.Guami,
		)
		if err != nil {
			return fmt.Errorf("send initial context setup request error: %v", err)
		}

		ue.Log.Info("Sent NGAP initial context setup request to UE")

		ranUe.ICS = amf.ICSPending

		return nil
	}

	// 504: the UE in MICO mode or the UE is only registered over Non-3GPP access and its state is CM-IDLE
	return storeN1N2AndPage(ctx, amfInstance, ue, req)
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
