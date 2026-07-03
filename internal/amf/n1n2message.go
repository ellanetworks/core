// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ErrUENotReachable is returned when the UE is in CM-IDLE state and the
// requested signaling cannot be delivered. Per TS 23.502,
// the AMF may ignore the N2 SM information when the UE is not reachable and
// the caller should defer delivery until the UE transitions to CM-CONNECTED.
var ErrUENotReachable = errors.New("UE is in CM-IDLE state")

func (amf *AMF) TransferN1N2Message(ctx context.Context, supi etsi.SUPI, req models.N1N2MessageTransferRequest) error {
	ctx, span := tracer.Start(
		ctx,
		"AMF N1N2 MessageTransfer",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
		),
	)
	defer span.End()

	ue, ok := amf.FindUeContextBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return amf.storeN1N2AndPage(ctx, ue, req)
	}

	nasPdu, err := BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, req.BinaryDataN1Message, req.PduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	ue.Log.Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")

	if ranUe.ICS != ICSNotStarted {
		list := ngapType.PDUSessionResourceSetupListSUReq{}

		send.AppendPDUSessionResourceSetupListSUReq(&list, req.PduSessionID, req.SNssai, nasPdu, req.BinaryDataN2Information)

		err := ranUe.SendPDUSessionResourceSetupRequest(ctx, ue.Ambr.Uplink, ue.Ambr.Downlink, nil, list)
		if err != nil {
			return fmt.Errorf("send pdu session resource setup request error: %v", err)
		}

		ue.Log.Info("Sent NGAP pdu session resource setup request to UE")

		return nil
	}

	operatorInfo, err := amf.OperatorInfo(ctx)
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
		ue.kgnb,
		ue.PlmnID,
		ue.UeRadioCapability,
		ue.UeRadioCapabilityForPaging,
		ue.ueSecurityCapability,
		nil,
		&list,
		operatorInfo.Guami,
	)
	if err != nil {
		return fmt.Errorf("send initial context setup request error: %v", err)
	}

	ue.Log.Info("Sent NGAP initial context setup request to UE")

	ranUe.ICS = ICSPending

	return nil
}

func (amf *AMF) storeN1N2AndPage(ctx context.Context, ue *UeContext, req models.N1N2MessageTransferRequest) error {
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

	if ue.State() != Registered {
		return fmt.Errorf("ue is not in registered state")
	}

	nasConn.N1N2Message = &req

	_, beginErr := nasConn.Procedures.Begin(nasConn.Ctx(), procedure.Procedure{Type: procedure.Paging})
	if beginErr != nil {
		return fmt.Errorf("begin paging procedure: %w", beginErr)
	}

	pkg, err := send.BuildPaging(
		ue.guti,
		ue.RegistrationArea,
		ue.UeRadioCapabilityForPaging,
		nil,
	)
	if err != nil {
		return fmt.Errorf("build paging error: %v", err)
	}

	if err := amf.SendPaging(ctx, ue, pkg); err != nil {
		return fmt.Errorf("send paging error: %v", err)
	}

	return nil
}

// ModifyN1N2Message delivers a PDU Session Modification Command (N1) to the
// UE, optionally accompanied by a PDU Session Resource Modify Request (N2) to
// the gNB when radio-resource changes are needed.
//
// When n2Msg is nil (e.g. DNS-only change carried in Extended PCO), the NAS
// message is delivered transparently via Downlink NAS Transport (TS 38.413)
// — no gNB resource modification is required.
//
// When n2Msg is present (AMBR/QoS changes), the AMF sends a
// PDUSessionResourceModifyRequest (TS 38.413) which carries both the
// N1 NAS PDU and the mandatory N2 transfer IE for the gNB.
//
// This implements TS 23.502.
func (amf *AMF) ModifyN1N2Message(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Msg []byte) error {
	ctx, span := tracer.Start(
		ctx,
		"AMF PDUSessionModification",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
			attribute.Int("pdu_session_id", int(pduSessionID)),
		),
	)
	defer span.End()

	ue, ok := amf.FindUeContextBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		// Per TS 23.502: in CM-IDLE the AMF may ignore the N2
		// SM information. The gNB has released the session's radio resources,
		// so there is nothing to modify; the updated QoS applies on the next
		// CM-CONNECTED setup.
		return ErrUENotReachable
	}

	nasPdu, err := BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	if n2Msg == nil {
		// N1-only delivery (e.g. DNS update via Extended PCO). Per TS 23.502,
		// when the Modification Command is sent transparently through
		// NG-RAN the N2 SM information is omitted and no radio resources change.
		if err := ranUe.SendDownlinkNasTransport(ctx, nasPdu, nil); err != nil {
			return fmt.Errorf("send downlink NAS transport: %w", err)
		}

		ue.Log.Info("Sent DL NAS Transport (N1-only session modification) to gNB")

		return nil
	}

	// N1+N2 delivery: gNB resource modification required (AMBR/QoS change).
	// The PDUSessionResourceModifyRequestTransfer IE is mandatory per
	// TS 38.413, so this path must only be taken when n2Msg is set.
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
// This implements the network-initiated PDU Session Release (TS 23.502).
func (amf *AMF) ReleaseSessionMessage(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Transfer []byte) error {
	ctx, span := tracer.Start(
		ctx,
		"AMF PDUSessionResourceReleaseCommand",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
			attribute.Int("pdu_session_id", int(pduSessionID)),
		),
	)
	defer span.End()

	ue, ok := amf.FindUeContextBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return ErrUENotReachable
	}

	nasPdu, err := BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

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

func (amf *AMF) N2MessageTransferOrPage(ctx context.Context, supi etsi.SUPI, req models.N1N2MessageTransferRequest) error {
	ctx, span := tracer.Start(
		ctx,
		"AMF N1N2 MessageTransfer",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
		),
	)
	defer span.End()

	ue, ok := amf.FindUeContextBySupi(supi)
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

		if ranUe.ICS != ICSNotStarted {
			list := ngapType.PDUSessionResourceSetupListSUReq{}
			send.AppendPDUSessionResourceSetupListSUReq(&list, req.PduSessionID, req.SNssai, nil, req.BinaryDataN2Information)

			err := ranUe.SendPDUSessionResourceSetupRequest(ctx, ue.Ambr.Uplink, ue.Ambr.Downlink, nil, list)
			if err != nil {
				return fmt.Errorf("send pdu session resource setup request error: %v", err)
			}

			ue.Log.Info("Sent NGAP pdu session resource setup request to UE")

			return nil
		}

		operatorInfo, err := amf.OperatorInfo(ctx)
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
			ue.kgnb,
			ue.PlmnID,
			ue.UeRadioCapability,
			ue.UeRadioCapabilityForPaging,
			ue.ueSecurityCapability,
			nil,
			&list,
			operatorInfo.Guami,
		)
		if err != nil {
			return fmt.Errorf("send initial context setup request error: %v", err)
		}

		ue.Log.Info("Sent NGAP initial context setup request to UE")

		ranUe.ICS = ICSPending

		return nil
	}

	// 504: the UE in MICO mode or the UE is only registered over Non-3GPP access and its state is CM-IDLE
	return amf.storeN1N2AndPage(ctx, ue, req)
}

func (amf *AMF) TransferN1Msg(ctx context.Context, supi etsi.SUPI, n1Msg []byte, pduSessionID uint8) error {
	ctx, span := tracer.Start(
		ctx,
		"AMF N1N2 MessageTransfer",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
		),
	)
	defer span.End()

	ue, ok := amf.FindUeContextBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	nasPdu, err := BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
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
