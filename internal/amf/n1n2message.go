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
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
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

	nasPdu, err := BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, req.BinaryDataN1Message, req.PduSessionID, nil, nil)
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

	err = ueConn.SendInitialContextSetup(
		ctx,
		ue.Ambr.Uplink,
		ue.Ambr.Downlink,
		ue.AllowedNssai,
		ue.kgnb,
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

// storeN1N2AndPage buffers an SMF-pushed N1N2 message on the idle UE's persistent
// context and pages it (TS 23.502 §4.2.3.3). The buffer is delivered on the new
// connection the UE establishes when it answers the page.
func (amf *AMF) storeN1N2AndPage(ctx context.Context, ue *UeContext, req models.N1N2MessageTransferRequest) error {
	if ue.PagingActive() {
		return fmt.Errorf("higher priority request ongoing")
	}

	if ue.State() == RegistrationInitiated {
		return fmt.Errorf("temporary reject registration ongoing")
	}

	if ue.Procedures().Active(procedure.N2Handover) {
		return fmt.Errorf("temporary reject handover ongoing")
	}

	if ue.State() != Registered {
		return fmt.Errorf("ue is not in registered state")
	}

	ue.SetN1N2Message(&req)

	operatorInfo, err := amf.OperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("get operator info error: %v", err)
	}

	guti, err := amf.PagingGuti(operatorInfo.Guami, ue)
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

	nasPdu, err := BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	if n2Msg == nil {
		// N1-only delivery (e.g. DNS update via Extended PCO): the Modification
		// Command rides Downlink NAS Transport and no radio resources change
		// (TS 23.502).
		if err := ueConn.SendDownlinkNASTransport(ctx, nasPdu); err != nil {
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

	nasPdu, err := BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil, nil)
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

	ueConn := ue.Conn()
	if ueConn == nil {
		// UE is CM-IDLE: buffer the N2 message and page it (TS 23.502 §4.2.3.3).
		return amf.storeN1N2AndPage(ctx, ue, req)
	}

	if ue.PagingActive() {
		return fmt.Errorf("higher priority request ongoing")
	}

	if ue.State() == RegistrationInitiated {
		return fmt.Errorf("temporary reject registration ongoing")
	}

	if ueConn.Parent().Procedures().Active(procedure.N2Handover) {
		return fmt.Errorf("temporary reject handover ongoing")
	}

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

	err = ueConn.SendInitialContextSetup(
		ctx,
		ue.Ambr.Uplink,
		ue.Ambr.Downlink,
		ue.AllowedNssai,
		ue.kgnb,
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

	nasPdu, err := BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	err = ueConn.SendDownlinkNASTransport(ctx, nasPdu)
	if err != nil {
		return fmt.Errorf("send downlink nas transport error: %v", err)
	}

	logger.From(ctx, logger.AmfLog).Info("sent downlink nas transport to UE", logger.SUPI(supi.String()))

	return nil
}

// TransferN1LPPMsg wraps an LPP payload in a DL NAS Transport message and
// sends it to the UE via the RAN. Per TS 24.501 §5.4.5.3.1 case c), LPP
// payloads are carried in DL NAS Transport with PayloadContainerTypeLPP.
//
// pduSessionID must be 0 for LPP messages — LPP is not PDU-session-scoped.
func (amf *AMF) TransferN1LPPMsg(ctx context.Context, supi etsi.SUPI, lppMsg []byte) error {
	ctx, span := tracer.Start(
		ctx,
		"AMF N1 LPP Transfer",
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

	// TS 24.501 §5.4.5.3.2 case c): the Additional information IE carries an LCS
	// correlation identifier, which the UE hands to its location services
	// application (§5.4.5.3.3 case c) and echoes back on the uplink
	// (§5.4.5.2.1 case c). NOTE 2 of §5.4.5.3.2 has the AMF assign it for
	// on-demand transfers, and distinguishes AMF- from LMF-assigned identifiers
	// by octet count, so this one is 4 octets.
	correlationID := amf.nextLCSCorrelationID()

	// TS 24.501 §9.11.3.1: the UE reports whether it speaks LPP in N1 mode at
	// registration. A UE that reports "not supported" is under no obligation to
	// answer anything sent here, so the bit is reported alongside the PDU.
	lppSupported := "unknown"

	if reg := ueConn.RegistrationRequest; reg != nil && reg.Capability5GMM != nil {
		lppSupported = strconv.Itoa(int(reg.Capability5GMM.GetLPP()))
	}

	nasPdu, err := BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeLPP, lppMsg, 0, nil, correlationID)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport (LPP) error: %v", err)
	}

	if err := ueConn.SendDownlinkNASTransport(ctx, nasPdu); err != nil {
		return fmt.Errorf("send downlink nas transport (LPP): %w", err)
	}

	logger.From(ctx, logger.AmfLog).Info("sent DL NAS Transport (LPP) to UE",
		logger.SUPI(supi.String()),
		zap.Uint8("payload_container_type", nasMessage.PayloadContainerTypeLPP),
		zap.Int("lpp_len", len(lppMsg)),
		zap.String("lpp_hex", hex.EncodeToString(lppMsg)),
		zap.String("lcs_correlation_id", hex.EncodeToString(correlationID)),
		zap.String("ue_lpp_n1_capability", lppSupported),
	)

	return nil
}

// nextLCSCorrelationID returns the next AMF-assigned LCS correlation identifier
// for an LPP transfer, as a 4-octet value (TS 24.501 §5.4.5.3.2 NOTE 2).
func (amf *AMF) nextLCSCorrelationID() []byte {
	id := make([]byte, 4)
	binary.BigEndian.PutUint32(id, amf.lcsCorrelationSeq.Add(1))

	return id
}
