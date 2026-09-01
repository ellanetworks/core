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
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
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

	logger.From(ctx, logger.AmfLog).Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")

	plain, err := BuildDLNASTransport(fgs.PayloadContainerTypeN1SMInfo, req.BinaryDataN1Message, new(fgs.PDUSessionID(req.PduSessionID)), nil, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	sht := uint8(fgs.SHTIntegrityProtectedCiphered)

	if !ueConn.ClaimICS() {
		// Context already set up (or in progress): deliver the PDU session standalone.
		return ue.SendDownlinkNAS(plain, sht, func(wire []byte) error {
			if !ueConn.ClaimN2SetupSession(N2SetupPDUSession, req.PduSessionID) {
				logger.From(ctx, logger.AmfLog).Debug("delivering N1 without a duplicate PDU session setup",
					zap.Uint8("pdu_session_id", req.PduSessionID))

				return ueConn.SendDownlinkNASTransport(ctx, wire)
			}

			item, err := PDUSessionSetupItemSUReq(req.PduSessionID, req.SNssai, wire, req.BinaryDataN2Information)
			if err != nil {
				ueConn.EndN2Setup(N2SetupPDUSession)

				return fmt.Errorf("could not build PDU session setup item: %w", err)
			}

			list := ngap.PDUSessionResourceSetupListSUReq{item}

			if err := ueConn.SendPDUSessionResourceSetupRequest(ctx, ue.Ambr.Uplink, ue.Ambr.Downlink, nil, list); err != nil {
				ueConn.EndN2Setup(N2SetupPDUSession)

				return fmt.Errorf("send pdu session resource setup request error: %v", err)
			}

			ueConn.ArmN2Setup(N2SetupPDUSession, amf.N2SetupGuardCfg)

			logger.From(ctx, logger.AmfLog).Info("Sent NGAP pdu session resource setup request to UE")

			return nil
		})
	}

	// Claimed the Initial Context Setup: bundle the PDU session into it.
	operatorInfo, err := amf.OperatorInfo(ctx)
	if err != nil {
		ueConn.ResetICS()
		return fmt.Errorf("error getting operator info: %v", err)
	}

	kgnb, ueSecCap := ue.Kgnb(), ue.UESecCap()

	err = ue.SendDownlinkNAS(plain, sht, func(wire []byte) error {
		if !ueConn.ClaimN2SetupSession(N2SetupInitialContext, req.PduSessionID) {
			ueConn.ResetICS()

			logger.From(ctx, logger.AmfLog).Debug("delivering N1 without a duplicate PDU session setup",
				zap.Uint8("pdu_session_id", req.PduSessionID))

			return ueConn.SendDownlinkNASTransport(ctx, wire)
		}

		item, err := PDUSessionSetupItem(req.PduSessionID, req.SNssai, wire, req.BinaryDataN2Information)
		if err != nil {
			ueConn.EndN2Setup(N2SetupInitialContext)

			return fmt.Errorf("could not build PDU session setup item: %w", err)
		}

		list := ngap.PDUSessionResourceSetupListCxtReq{item}

		if err := ueConn.SendInitialContextSetup(
			ctx,
			ue.Ambr.Uplink,
			ue.Ambr.Downlink,
			ue.AllowedNssai,
			kgnb,
			ue.RadioCapability,
			ue.RadioCapabilityForPaging,
			ueSecCap,
			nil,
			list,
			operatorInfo.Guami,
		); err != nil {
			ueConn.EndN2Setup(N2SetupInitialContext)

			return fmt.Errorf("send initial context setup request error: %v", err)
		}

		ueConn.ArmN2Setup(N2SetupInitialContext, amf.N2SetupGuardCfg)

		logger.From(ctx, logger.AmfLog).Info("Sent NGAP initial context setup request to UE")

		return nil
	})
	if err != nil {
		ueConn.AbortICS()

		return err
	}

	return nil
}

// storeN1N2AndPage buffers a downlink request and pages the idle UE
// (TS 23.502 §4.2.3.3). An earlier request already being paged for is not displaced; the
// caller is told to retry (HIGHER_PRIORITY_REQUEST_ONGOING, TS 29.518 §6.1.7.3).
func (amf *AMF) storeN1N2AndPage(ctx context.Context, ue *UeContext, req models.N1N2MessageTransferRequest) error {
	if ue.PagingActive() {
		return errPagingActive
	}

	if err := guardIdlePaging(ue); err != nil {
		return err
	}

	return amf.pageIdleUE(ctx, ue, &req)
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
	if conn := ue.Conn(); conn != nil && ue.Procedures().Active(procedure.N2Handover) {
		return fmt.Errorf("temporary reject: PDU session modification during handover")
	}

	plain, err := BuildDLNASTransport(fgs.PayloadContainerTypeN1SMInfo, n1Msg, new(fgs.PDUSessionID(pduSessionID)), nil, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	return ue.SendDownlinkNAS(plain, uint8(fgs.SHTIntegrityProtectedCiphered), func(wire []byte) error {
		if n2Msg == nil {
			if err := ueConn.SendDownlinkNASTransport(ctx, wire); err != nil {
				return fmt.Errorf("send downlink NAS transport: %w", err)
			}

			logger.From(ctx, logger.AmfLog).Info("Sent DL NAS Transport (N1-only session modification) to gNB")

			return nil
		}

		list := ngap.PDUSessionResourceModifyListModReq{
			{
				PDUSessionID: ngap.PDUSessionID(pduSessionID),
				NASPDU:       ngap.Ptr(ngap.NASPDU(wire)),
				Transfer:     ngap.TransferContainer(n2Msg),
			},
		}

		if err := ueConn.SendPDUSessionResourceModifyRequest(ctx, list); err != nil {
			return fmt.Errorf("send pdu session resource modify request error: %v", err)
		}

		logger.From(ctx, logger.AmfLog).Info("Sent NGAP PDU Session Resource Modify Request to gNB")

		return nil
	})
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

	plain, err := BuildDLNASTransport(fgs.PayloadContainerTypeN1SMInfo, n1Msg, new(fgs.PDUSessionID(pduSessionID)), nil, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	return ue.SendDownlinkNAS(plain, uint8(fgs.SHTIntegrityProtectedCiphered), func(wire []byte) error {
		list := ngap.PDUSessionResourceToReleaseListRelCmd{
			{PDUSessionID: ngap.PDUSessionID(pduSessionID), Transfer: ngap.TransferContainer(n2Transfer)},
		}

		if err := ueConn.SendPDUSessionResourceReleaseCommand(ctx, wire, list); err != nil {
			return fmt.Errorf("send pdu session resource release command error: %v", err)
		}

		logger.From(ctx, logger.AmfLog).Info("Sent NGAP PDU Session Resource Release Command to gNB",
			logger.PDUSessionID(pduSessionID),
		)

		return nil
	})
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

	if ue.State() == RegistrationInitiated {
		return fmt.Errorf("temporary reject registration ongoing")
	}

	if ue.Procedures().Active(procedure.N2Handover) {
		return fmt.Errorf("temporary reject handover ongoing")
	}

	logger.From(ctx, logger.AmfLog).Debug("AMF Transfer NGAP PDU Session Resource Setup Request from SMF")

	if !ueConn.ClaimICS() {
		// Context already set up (or in progress): deliver the PDU session standalone.
		if !ueConn.ClaimN2SetupSession(N2SetupPDUSession, req.PduSessionID) {
			logger.From(ctx, logger.AmfLog).Warn("PDU session already set up on the NG-RAN node; dropping the duplicate N2 transfer",
				zap.Uint8("pdu_session_id", req.PduSessionID))

			return nil
		}

		item, err := PDUSessionSetupItemSUReq(req.PduSessionID, req.SNssai, nil, req.BinaryDataN2Information)
		if err != nil {
			ueConn.EndN2Setup(N2SetupPDUSession)

			return fmt.Errorf("could not build PDU session setup item: %w", err)
		}

		list := ngap.PDUSessionResourceSetupListSUReq{item}

		err = ueConn.SendPDUSessionResourceSetupRequest(ctx, ue.Ambr.Uplink, ue.Ambr.Downlink, nil, list)
		if err != nil {
			ueConn.EndN2Setup(N2SetupPDUSession)

			return fmt.Errorf("send pdu session resource setup request error: %v", err)
		}

		ueConn.ArmN2Setup(N2SetupPDUSession, amf.N2SetupGuardCfg)

		logger.From(ctx, logger.AmfLog).Info("Sent NGAP pdu session resource setup request to UE")

		return nil
	}

	// Claimed the Initial Context Setup: bundle the PDU session into it.
	operatorInfo, err := amf.OperatorInfo(ctx)
	if err != nil {
		ueConn.ResetICS()
		return fmt.Errorf("error getting operator info: %v", err)
	}

	if !ueConn.ClaimN2SetupSession(N2SetupInitialContext, req.PduSessionID) {
		ueConn.ResetICS()

		logger.From(ctx, logger.AmfLog).Warn("PDU session already set up on the NG-RAN node; dropping the duplicate N2 transfer",
			zap.Uint8("pdu_session_id", req.PduSessionID))

		return nil
	}

	item, err := PDUSessionSetupItem(req.PduSessionID, req.SNssai, nil, req.BinaryDataN2Information)
	if err != nil {
		ueConn.AbortICS()

		return fmt.Errorf("could not build PDU session setup item: %w", err)
	}

	list := ngap.PDUSessionResourceSetupListCxtReq{item}

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
		list,
		operatorInfo.Guami,
	)
	if err != nil {
		ueConn.AbortICS()

		return fmt.Errorf("send initial context setup request error: %v", err)
	}

	ueConn.ArmN2Setup(N2SetupInitialContext, amf.N2SetupGuardCfg)

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

	plain, err := BuildDLNASTransport(fgs.PayloadContainerTypeN1SMInfo, n1Msg, new(fgs.PDUSessionID(pduSessionID)), nil, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport error: %v", err)
	}

	return ue.SendDownlinkNAS(plain, uint8(fgs.SHTIntegrityProtectedCiphered), func(wire []byte) error {
		if err := ueConn.SendDownlinkNASTransport(ctx, wire); err != nil {
			return fmt.Errorf("send downlink nas transport error: %v", err)
		}

		logger.From(ctx, logger.AmfLog).Info("sent downlink nas transport to UE", logger.SUPI(supi.String()))

		return nil
	})
}

// TransferN1LPPMsg transfers a DL Positioning message to the UE (TS 23.273 §6.11.1
// step 1), paging it first when CM-IDLE (step 2).
func (amf *AMF) TransferN1LPPMsg(ctx context.Context, supi etsi.SUPI, correlationID, lppMsg []byte) error {
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

	// A transfer with no correlation identifier is assigned a fresh 4-octet one
	// (TS 24.501 §5.4.5.3.1 NOTE 2).
	if len(correlationID) == 0 {
		correlationID = amf.nextLCSCorrelationID()
	}

	req := models.N1N2MessageTransferRequest{
		N1Class:             models.N1ClassLPP,
		BinaryDataN1Message: lppMsg,
		LCSCorrelationID:    correlationID,
	}

	return amf.transferOrPageStandalone(ctx, ue, req)
}

// TransferN2NRPPaMsg transfers a Network Positioning message to the serving NG-RAN node
// (TS 23.273 §6.11.2 step 1), paging the UE first when CM-IDLE (step 2).
func (amf *AMF) TransferN2NRPPaMsg(ctx context.Context, supi etsi.SUPI, routingID int64, nrppaPdu []byte) error {
	ctx, span := tracer.Start(
		ctx,
		"AMF N2 NRPPa Transfer",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
		),
	)
	defer span.End()

	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("ue context not found")
	}

	req := models.N1N2MessageTransferRequest{
		N2Class:                 models.N2ClassNRPPa,
		BinaryDataN2Information: nrppaPdu,
		RoutingID:               routingID,
	}

	return amf.transferOrPageStandalone(ctx, ue, req)
}

// transferOrPageStandalone delivers a request that is not PDU-session scoped, buffering it
// and paging the UE when it is CM-IDLE (TS 23.502 §4.2.3.3).
func (amf *AMF) transferOrPageStandalone(ctx context.Context, ue *UeContext, req models.N1N2MessageTransferRequest) error {
	conn := ue.Conn()
	if conn == nil {
		return amf.storeN1N2AndPage(ctx, ue, req)
	}

	return DeliverStandaloneN1N2(ctx, ue, conn, &req)
}

// AllocateLCSCorrelationID assigns the LCS correlation identifier for a
// positioning session. TS 23.273 §6.11.1: one identifier, assigned by the AMF
// and passed to the LMF, is used for every downlink and uplink message of the
// session so responses route to the correct LMF and session (NOTE 11).
func (amf *AMF) AllocateLCSCorrelationID() []byte {
	return amf.nextLCSCorrelationID()
}

// nextLCSCorrelationID returns the next AMF-assigned LCS correlation identifier
// for an LPP transfer, as a 4-octet value (TS 24.501 §5.4.5.3.1 NOTE 2).
func (amf *AMF) nextLCSCorrelationID() []byte {
	id := make([]byte, 4)
	binary.BigEndian.PutUint32(id, amf.lcsCorrelationSeq.Add(1))

	return id
}

func (amf *AMF) SessionDropped(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, ref string, n2Transfer []byte) {
	ctx, span := tracer.Start(
		ctx,
		"AMF SessionDropped",
		trace.WithAttributes(
			attribute.String("supi", supi.String()),
			attribute.Int("pdu_session_id", int(pduSessionID)),
		),
	)
	defer span.End()

	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		return
	}

	if !ue.DeleteSmContextRef(pduSessionID, ref) {
		logger.From(ctx, logger.AmfLog).Debug("ignoring a transfer report for a session this AMF no longer routes",
			logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID), zap.String("ref", ref))

		return
	}

	logger.From(ctx, logger.AmfLog).Info("PDU session moved to EPS; dropping the 5GS routing context",
		logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))

	if n2Transfer == nil {
		return
	}

	if amf.HandoverToEPSInProgress(ue) {
		return
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		return
	}

	list := ngap.PDUSessionResourceToReleaseListRelCmd{
		{PDUSessionID: ngap.PDUSessionID(pduSessionID), Transfer: ngap.TransferContainer(n2Transfer)},
	}

	if err := ueConn.SendPDUSessionResourceReleaseCommand(ctx, nil, list); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to release the NG-RAN resources of a moved PDU session",
			zap.Error(err), logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))
	}
}
