// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func forward5GSMMessageToSMF(
	ctx context.Context,
	amfInstance *amf.AMF,
	ue *amf.UeContext,
	pduSessionID uint8,
	smContextRef string,
	smMessage []byte,
) {
	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Warn("RAN UE context is nil, cannot forward 5GSM message to SMF")
		return
	}

	response, err := amfInstance.Session.UpdateSmContextN1Msg(ctx, smContextRef, smMessage)
	if err != nil {
		// Kept, it survives the UE's whole registration, keeps the PDU session ID
		// looking occupied, and leaves the reconciler working a dead reference.
		if errors.Is(err, smf.ErrSMContextNotFound) {
			ue.DeleteSmContext(pduSessionID)
		}

		logger.From(ctx, logger.AmfLog).Warn("couldn't send update sm context request", zap.Error(err))

		return
	}

	if response == nil {
		logger.From(ctx, logger.AmfLog).Warn("SMF did not return any N1/N2 message", zap.Uint8("pdu_session_id", pduSessionID))
		return
	}

	// No later signal would drop the routing context.
	if response.SessionRemoved {
		ue.DeleteSmContext(pduSessionID)
	}

	var plain []byte

	if response.N1Msg != nil {
		logger.From(ctx, logger.AmfLog).Debug("Receive N1 SM Message from SMF")

		var err error

		plain, err = amf.BuildDLNASTransport(fgs.PayloadContainerTypeN1SMInfo, response.N1Msg, new(fgs.PDUSessionID(pduSessionID)), nil, nil)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("could not deliver the SMF's 5GSM message", zap.Error(err))

			return
		}
	}

	if response.N2Msg != nil {
		logger.From(ctx, logger.AmfLog).Debug("Receive N2 SM Information from SMF")

		if !response.ReleaseN2 {
			logger.From(ctx, logger.AmfLog).Debug("amf.AMF forward N2 SM Information to UE")

			return
		}
	}

	send := func(wire []byte) error {
		if response.N2Msg != nil {
			list := ngap.PDUSessionResourceToReleaseListRelCmd{
				{PDUSessionID: ngap.PDUSessionID(pduSessionID), Transfer: ngap.TransferContainer(response.N2Msg)},
			}

			if err := ueConn.SendPDUSessionResourceReleaseCommand(ctx, wire, list); err != nil {
				return fmt.Errorf("error sending pdu session resource release command: %w", err)
			}

			logger.From(ctx, logger.AmfLog).Info("sent pdu session resource release command to UE")

			return nil
		}

		if wire != nil {
			if err := ueConn.SendDownlinkNASTransport(ctx, wire); err != nil {
				return fmt.Errorf("error sending downlink nas transport: %w", err)
			}

			logger.From(ctx, logger.AmfLog).Info("sent downlink nas transport to UE")
		}

		return nil
	}

	deliver := func() error {
		if plain == nil {
			return send(nil)
		}

		return ue.SendDownlinkNAS(plain, uint8(fgs.SHTIntegrityProtectedCiphered), send)
	}

	if err := deliver(); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not deliver the SMF's 5GSM message", zap.Error(err))
	}
}

func transport5GSMMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, ulNasTransport *fgs.ULNASTransport) {
	smMessage := ulNasTransport.PayloadContainer

	if ulNasTransport.PDUSessionID == nil {
		logger.From(ctx, logger.AmfLog).Warn("pdu session id is nil")
		return
	}

	pduSessionID := *ulNasTransport.PDUSessionID

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Warn("RAN UE context is nil, cannot transport 5GSM message")
		return
	}

	if ulNasTransport.OldPDUSessionID != nil {
		logger.From(ctx, logger.AmfLog).Warn("old pdu session id is not supported")
		return
	}

	// Reserved or unassigned PDU session identity value (TS 24.501).
	if pduSessionID < 1 || pduSessionID > 15 {
		sendPayloadNotForwarded(ctx, ueConn, uint8(pduSessionID), smMessage)
		return
	}

	requestType := ulNasTransport.RequestType

	if requestType != nil && *requestType == fgs.RequestTypeInitialEmergencyRequest {
		logger.From(ctx, logger.AmfLog).Warn("Emergency PDU Session is not supported")
		sendPayloadNotForwarded(ctx, ueConn, uint8(pduSessionID), smMessage)

		return
	}

	if ulNasTransport.SNSSAI != nil && requestType != nil {
		switch *requestType {
		case fgs.RequestTypeInitialRequest, fgs.RequestTypeModificationRequest,
			fgs.RequestTypeExistingPDUSession:
			if snssai := util.SnssaiToModels(*ulNasTransport.SNSSAI); !ue.IsAllowedNssai(snssai) {
				logger.From(ctx, logger.AmfLog).Warn("requested S-NSSAI is not in the allowed NSSAI",
					zap.Any("snssai", snssai), logger.PDUSessionID(uint8(pduSessionID)))
				sendPayloadNotForwarded(ctx, ueConn, uint8(pduSessionID), smMessage)

				return
			}
		}
	}

	smContext, smContextExist := ue.SmContextFindByPDUSessionID(uint8(pduSessionID))

	isInitialRequest := requestType != nil &&
		*requestType == fgs.RequestTypeInitialRequest

	// Duplicate PDU session ID: an initial request for an active session locally
	// releases it and re-establishes (TS 24.501).
	if smContextExist && isInitialRequest {
		ue.DeleteSmContext(uint8(pduSessionID))

		smContext, smContextExist = nil, false
	}

	if smContextExist {
		// Existing PDU session whose S-NSSAI is not allowed for the access type
		// (TS 24.501).
		if requestType != nil &&
			*requestType == fgs.RequestTypeExistingPDUSession &&
			!ue.IsAllowedNssai(smContext.Snssai) {
			logger.From(ctx, logger.AmfLog).Error("S-NSSAI is not allowed for access type", zap.Any("snssai", smContext.Snssai), logger.PDUSessionID(uint8(pduSessionID)))
			sendPayloadNotForwarded(ctx, ueConn, uint8(pduSessionID), smMessage)

			return
		}

		forward5GSMMessageToSMF(ctx, amfInstance, ue, uint8(pduSessionID), smContext.Ref, smMessage)

		return
	}

	if isInitialRequest || requestTypeReachesSMF(requestType) {
		establishPDUSession(ctx, amfInstance, ue, ueConn, ulNasTransport, uint8(pduSessionID), smMessage)
		return
	}

	// A 5GSM STATUS for a PDU session with no context is ignored (TS 24.501).
	if isStatus5GSM(smMessage) {
		logger.From(ctx, logger.AmfLog).Warn("5GSM STATUS for unknown PDU session, ignoring", logger.PDUSessionID(uint8(pduSessionID)))
		return
	}

	// No routing context and not an initial request (TS 24.501).
	sendPayloadNotForwarded(ctx, ueConn, uint8(pduSessionID), smMessage)
}

// sendPayloadNotForwarded returns the 5GSM message to the UE in a DL NAS
// TRANSPORT with 5GMM cause #90 "payload was not forwarded" (TS 24.501).
func sendPayloadNotForwarded(ctx context.Context, ueConn *amf.UeConn, pduSessionID uint8, smMessage []byte) {
	amf.SendDLNASTransport(ctx, ueConn, fgs.PayloadContainerTypeN1SMInfo, smMessage, fgs.PDUSessionID(pduSessionID), fgs.GMMCausePayloadNotForwarded)
}

func isStatus5GSM(smMessage []byte) bool {
	mt, err := fgs.PeekGSMMessageType(smMessage)
	if err != nil {
		return false
	}

	return mt == fgs.MsgGSMStatus
}

// establishPDUSession selects an SMF and creates the SM context for an initial
// request (TS 24.501). When the SMF rejects, its reject message
// is returned to the UE; when it produces none, the payload is reported as not
// forwarded.
func establishPDUSession(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, ueConn *amf.UeConn, ulNasTransport *fgs.ULNASTransport, pduSessionID uint8, smMessage []byte) {
	var (
		snssai *models.Snssai
		dnn    string
	)

	// Already checked against the allowed NSSAI by the caller.
	if ulNasTransport.SNSSAI != nil {
		snssai = util.SnssaiToModels(*ulNasTransport.SNSSAI)
	} else {
		if len(ue.AllowedNssai) == 0 {
			logger.From(ctx, logger.AmfLog).Warn("allowed nssai is empty in UE context")
			sendPayloadNotForwarded(ctx, ueConn, pduSessionID, smMessage)

			return
		}

		snssai = &ue.AllowedNssai[0]
	}

	if ulNasTransport.DNN != nil {
		dnn = string(*ulNasTransport.DNN)
	} else {
		dnnResp, err := amfInstance.SubscriberDnn(ctx, ue.Supi(), snssai)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to get subscriber data", zap.Error(err))
			sendPayloadNotForwarded(ctx, ueConn, pduSessionID, smMessage)

			return
		}

		dnn = dnnResp
	}

	requestType := fgs.RequestTypeInitialRequest
	if ulNasTransport.RequestType != nil {
		requestType = *ulNasTransport.RequestType
	}

	epsBearerIdentity := assignEPSBearerIdentity(ctx, ue, pduSessionID)

	smContextRef, errResponse, err := amfInstance.Session.CreateSmContext(ctx, ue.Supi(), pduSessionID, dnn, snssai, requestType, smMessage, epsBearerIdentity)

	// The SMF produced a 5GSM reject. Delivering it is a normal negative outcome,
	// not a 5GMM protocol error (TS 24.501).
	if errResponse != nil {
		amf.SendDLNASTransport(ctx, ueConn, fgs.PayloadContainerTypeN1SMInfo, errResponse, fgs.PDUSessionID(pduSessionID), 0)

		logger.From(ctx, logger.AmfLog).Info("PDU session establishment rejected by SMF", zap.Uint8("pdu_session_id", pduSessionID), zap.Error(err))

		return
	}

	// The SMF failed without producing a reject. Tell the UE the payload was not
	// forwarded (5GMM cause #90) so it does not time out (TS 24.501).
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("couldn't create sm context", zap.Error(err), zap.Uint8("pdu_session_id", pduSessionID))

		sendPayloadNotForwarded(ctx, ueConn, pduSessionID, smMessage)

		return
	}

	// The SMF processed the message but produced no context and no response,
	// e.g. an establishment request with a reserved PTI it had to ignore
	// (TS 24.501). Send nothing.
	if smContextRef == "" {
		logger.From(ctx, logger.AmfLog).Info("SMF ignored the PDU session establishment request, sending no response", zap.Uint8("pduSessionID", pduSessionID))
		return
	}

	if err := ue.CreateSmContext(pduSessionID, smContextRef, snssai, dnn); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("error creating SM context", zap.Error(err))
		return
	}

	ue.SetEPSBearerIdentity(pduSessionID, epsBearerIdentity)

	logger.From(ctx, logger.AmfLog).Debug("Created sm context for pdu session", zap.Uint8("pduSessionID", pduSessionID))
}

func assignEPSBearerIdentity(ctx context.Context, ue *amf.UeContext, pduSessionID uint8) uint8 {
	if !ue.EPSInterworkingAllowed() {
		return 0
	}

	ebi, err := ue.NextEPSBearerIdentity(pduSessionID)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("no EPS bearer identity for this PDU session, it will not transfer to EPS",
			zap.Uint8("pduSessionID", pduSessionID), zap.Error(err))

		return 0
	}

	logger.From(ctx, logger.AmfLog).Debug("assigned EPS bearer identity",
		zap.Uint8("pduSessionID", pduSessionID), zap.Uint8("ebi", ebi))

	return ebi
}

func handleULNASTransport(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *fgs.ULNASTransport) nasreply.Disposition {
	if ue.State() != amf.Registered {
		logger.From(ctx, logger.AmfLog).Warn("expected UE to be in Registered state during UL NAS Transport", zap.String("state", string(ue.State())))
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	switch msg.PayloadContainerType {
	case fgs.PayloadContainerTypeN1SMInfo:
		transport5GSMMessage(ctx, amfInstance, ue, msg)
	case fgs.PayloadContainerTypeSMS:
		logger.From(ctx, logger.AmfLog).Warn("PayloadContainerTypeSMS has not been implemented yet in UL NAS TRANSPORT")
	case fgs.PayloadContainerTypeLPP:
		lppData := msg.PayloadContainer

		// The UE echoes its LCS correlation identifier in the Additional
		// information IE (TS 24.501 §5.4.5.2.1); it identifies the LPP session, so
		// it is forwarded to the LMF to route a reply (e.g. an acknowledgement)
		// back to the same session.
		correlationID := msg.AdditionalInformation

		logger.From(ctx, logger.AmfLog).Debug("UL NAS Transport carries LPP payload", zap.Int("length", len(lppData)))

		if amfInstance.LPPHandler != nil {
			if err := amfInstance.LPPHandler.ForwardLPP(ctx, ue.Supi(), correlationID, lppData); err != nil {
				logger.From(ctx, logger.AmfLog).Error("failed to forward LPP to LMF", zap.Error(err))
			}
		} else {
			logger.From(ctx, logger.AmfLog).Error("LPP handler not configured")
		}
	case fgs.PayloadContainerTypeSOR:
		logger.From(ctx, logger.AmfLog).Warn("PayloadContainerTypeSOR has not been implemented yet in UL NAS TRANSPORT")
	case fgs.PayloadContainerTypeUEPolicy:
		logger.From(ctx, logger.AmfLog).Info("amf.AMF Transfer UEPolicy To PCF")
	case fgs.PayloadContainerTypeUEParameterUpdate:
		logger.From(ctx, logger.AmfLog).Info("amf.AMF Transfer UEParameterUpdate To UDM")

		upuMac, err := fgs.ParseUPUAcknowledgement(msg.PayloadContainer)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to parse UPU ACK", zap.Error(err))
			return nasreply.Handled()
		}

		logger.From(ctx, logger.AmfLog).Debug("UpuMac in UPU ACK NAS Msg", zap.String("UpuMac", hex.EncodeToString(upuMac.MAC[:])))
	case fgs.PayloadContainerTypeMultiplePayload:
		logger.From(ctx, logger.AmfLog).Warn("PayloadContainerTypeMultiplePayload has not been implemented yet in UL NAS TRANSPORT")
	}

	return nasreply.Handled()
}

func requestTypeReachesSMF(t *fgs.RequestType) bool {
	if t == nil {
		return false
	}

	return *t == fgs.RequestTypeExistingPDUSession || *t == fgs.RequestTypeExistingEmergencyPDUSession
}
