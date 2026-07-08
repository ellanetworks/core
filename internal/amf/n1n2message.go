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
// requested signaling cannot be delivered. Per TS 23.502 the AMF may ignore
// the N2 SM information when the UE is not reachable; delivery is deferred
// until the UE transitions to CM-CONNECTED.
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

	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		return amf.storeN1N2AndPage(ctx, ue, req)
	}

	nasPdu, err := BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, req.BinaryDataN1Message, req.PduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	logger.From(ctx, logger.AmfLog).Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")

	if !ueConn.ClaimICS() {
		// Context already set up (or in progress): deliver the PDU session standalone.
		list := ngapType.PDUSessionResourceSetupListSUReq{}

		send.AppendPDUSessionResourceSetupListSUReq(&list, req.PduSessionID, req.SNssai, nasPdu, req.BinaryDataN2Information)

		err := ueConn.SendPDUSessionResourceSetupRequest(ctx, ue.Ambr.Uplink, ue.Ambr.Downlink, nil, list)
		if err != nil {
			return fmt.Errorf("send pdu session resource setup request error: %v", err)
		}

		logger.From(ctx, logger.AmfLog).Info("Sent NGAP pdu session resource setup request to UE")

		return nil
	}

	// Claimed the Initial Context Setup: bundle the PDU session into it.
	operatorInfo, err := amf.OperatorInfo(ctx)
	if err != nil {
		ueConn.ResetICS()
		return fmt.Errorf("error getting operator info: %v", err)
	}

	list := ngapType.PDUSessionResourceSetupListCxtReq{}

	send.AppendPDUSessionResourceSetupListCxtReq(&list, req.PduSessionID, req.SNssai, nasPdu, req.BinaryDataN2Information)

	err = ueConn.SendInitialContextSetupRequest(
		ctx,
		ue.Ambr.Uplink,
		ue.Ambr.Downlink,
		ue.AllowedNssai,
		ue.kgnb,
		ue.PlmnID,
		ue.RadioCapability,
		ue.RadioCapabilityForPaging,
		ue.ueSecurityCapability,
		nil,
		&list,
		operatorInfo.Guami,
	)
	if err != nil {
		ueConn.ResetICS()
		return fmt.Errorf("send initial context setup request error: %v", err)
	}

	logger.From(ctx, logger.AmfLog).Info("Sent NGAP initial context setup request to UE")

	return nil
}

func (amf *AMF) storeN1N2AndPage(ctx context.Context, ue *UeContext, req models.N1N2MessageTransferRequest) error {
	nasConn := ue.Conn()
	if nasConn == nil {
		return fmt.Errorf("ue has no active NAS connection")
	}

	if ue.PagingActive() {
		return fmt.Errorf("higher priority request ongoing")
	}

	if ue.State() == RegistrationInitiated {
		return fmt.Errorf("temporary reject registration ongoing")
	}

	if nasConn.Parent().Procedures().Active(procedure.N2Handover) {
		return fmt.Errorf("temporary reject handover ongoing")
	}

	if ue.State() != Registered {
		return fmt.Errorf("ue is not in registered state")
	}

	nasConn.SetN1N2Message(&req)

	operatorInfo, err := amf.OperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("get operator info error: %v", err)
	}

	guti, err := amf.Guti(operatorInfo.Guami, ue)
	if err != nil {
		return fmt.Errorf("build 5G-GUTI error: %v", err)
	}

	// Paging supervision is armed per-UE by SendPaging; there is no per-session
	// paging to track in the procedure registry.
	pkg, err := send.BuildPaging(
		guti,
		ue.RegistrationArea,
		ue.RadioCapabilityForPaging,
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
// UE, optionally with a PDU Session Resource Modify Request (N2) to the gNB.
//
// With n2Msg nil (e.g. DNS-only change carried in Extended PCO) the NAS
// message is delivered transparently via Downlink NAS Transport and no gNB
// resources change. With n2Msg present (AMBR/QoS change) the
// PDUSessionResourceModifyRequest carries both the N1 NAS PDU and the
// mandatory N2 transfer IE (TS 38.413, TS 23.502).
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

	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		// Per TS 23.502, in CM-IDLE the AMF may ignore the N2 SM information.
		// The gNB has released the session's radio resources, so there is
		// nothing to modify; the updated QoS applies on the next CM-CONNECTED
		// setup.
		return ErrUENotReachable
	}

	// A network-requested modification during an N2 handover races the
	// handover's own resource signalling on the source gNB (TS 38.413 §8.4).
	// Defer it; the reconcile backstop re-applies it once the handover completes.
	if conn := ue.Conn(); conn != nil && conn.Parent().Procedures().Active(procedure.N2Handover) {
		return fmt.Errorf("temporary reject: PDU session modification during handover")
	}

	nasPdu, err := BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	if n2Msg == nil {
		// N1-only delivery (e.g. DNS update via Extended PCO). Per TS 23.502,
		// when the Modification Command is sent transparently through NG-RAN
		// the N2 SM information is omitted and no radio resources change.
		if err := ueConn.SendDownlinkNASTransport(ctx, nasPdu, nil); err != nil {
			return fmt.Errorf("send downlink NAS transport: %w", err)
		}

		logger.From(ctx, logger.AmfLog).Info("Sent DL NAS Transport (N1-only session modification) to gNB")

		return nil
	}

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

	if err := ueConn.SendPDUSessionResourceModifyRequest(ctx, list); err != nil {
		return fmt.Errorf("send pdu session resource modify request error: %v", err)
	}

	logger.From(ctx, logger.AmfLog).Info("Sent NGAP PDU Session Resource Modify Request to gNB")

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

	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	ueConn := ue.Conn()
	if ueConn == nil {
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

	if err := ueConn.SendPDUSessionResourceReleaseCommand(ctx, nasPdu, list); err != nil {
		return fmt.Errorf("send pdu session resource release command error: %v", err)
	}

	logger.From(ctx, logger.AmfLog).Info("Sent NGAP PDU Session Resource Release Command to gNB",
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

	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	conn := ue.Conn()
	if conn == nil {
		return fmt.Errorf("ue has no active NAS connection")
	}

	if ue.PagingActive() {
		return fmt.Errorf("higher priority request ongoing")
	}

	if ue.State() == RegistrationInitiated {
		return fmt.Errorf("temporary reject registration ongoing")
	}

	if conn.Parent().Procedures().Active(procedure.N2Handover) {
		return fmt.Errorf("temporary reject handover ongoing")
	}

	ueConn := ue.Conn()
	if ueConn != nil {
		logger.From(ctx, logger.AmfLog).Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")

		if !ueConn.ClaimICS() {
			// Context already set up (or in progress): deliver the PDU session standalone.
			list := ngapType.PDUSessionResourceSetupListSUReq{}
			send.AppendPDUSessionResourceSetupListSUReq(&list, req.PduSessionID, req.SNssai, nil, req.BinaryDataN2Information)

			err := ueConn.SendPDUSessionResourceSetupRequest(ctx, ue.Ambr.Uplink, ue.Ambr.Downlink, nil, list)
			if err != nil {
				return fmt.Errorf("send pdu session resource setup request error: %v", err)
			}

			logger.From(ctx, logger.AmfLog).Info("Sent NGAP pdu session resource setup request to UE")

			return nil
		}

		// Claimed the Initial Context Setup: bundle the PDU session into it.
		operatorInfo, err := amf.OperatorInfo(ctx)
		if err != nil {
			ueConn.ResetICS()
			return fmt.Errorf("error getting operator info: %v", err)
		}

		list := ngapType.PDUSessionResourceSetupListCxtReq{}
		send.AppendPDUSessionResourceSetupListCxtReq(&list, req.PduSessionID, req.SNssai, nil, req.BinaryDataN2Information)

		err = ueConn.SendInitialContextSetupRequest(
			ctx,
			ue.Ambr.Uplink,
			ue.Ambr.Downlink,
			ue.AllowedNssai,
			ue.kgnb,
			ue.PlmnID,
			ue.RadioCapability,
			ue.RadioCapabilityForPaging,
			ue.ueSecurityCapability,
			nil,
			&list,
			operatorInfo.Guami,
		)
		if err != nil {
			ueConn.ResetICS()
			return fmt.Errorf("send initial context setup request error: %v", err)
		}

		logger.From(ctx, logger.AmfLog).Info("Sent NGAP initial context setup request to UE")

		return nil
	}

	// UE is CM-IDLE (MICO mode or non-3GPP-only registration): page it.
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

	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	nasPdu, err := BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	err = ueConn.SendDownlinkNASTransport(ctx, nasPdu, nil)
	if err != nil {
		return fmt.Errorf("send downlink nas transport error: %v", err)
	}

	logger.From(ctx, logger.AmfLog).Info("sent downlink nas transport to UE", logger.SUPI(supi.String()))

	return nil
}
