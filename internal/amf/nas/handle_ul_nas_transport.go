// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
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
		logger.From(ctx, logger.AmfLog).Warn("couldn't send update sm context request", zap.Error(err))
		return
	}

	if response == nil {
		logger.From(ctx, logger.AmfLog).Warn("SMF did not return any N1/N2 message", zap.Uint8("pduSessionID", pduSessionID))
		return
	}

	var n1Msg []byte

	if response.N1Msg != nil {
		logger.From(ctx, logger.AmfLog).Debug("Receive N1 SM Message from SMF")

		n1Msg, err = amf.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, response.N1Msg, pduSessionID, nil, nil)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error building DL NAS Transport", zap.Error(err))
			return
		}
	}

	if response.N2Msg != nil {
		logger.From(ctx, logger.AmfLog).Debug("Receive N2 SM Information from SMF")

		if !response.ReleaseN2 {
			logger.From(ctx, logger.AmfLog).Debug("amf.AMF forward N2 SM Information to UE")
			return
		}

		list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
		send.AppendPDUSessionResourceToReleaseListRelCmd(&list, pduSessionID, response.N2Msg)

		err := ueConn.SendPDUSessionResourceReleaseCommand(ctx, n1Msg, list)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error sending pdu session resource release command", zap.Error(err))
			return
		}

		logger.From(ctx, logger.AmfLog).Info("sent pdu session resource release command to UE")

		return
	}

	if n1Msg != nil {
		err := ueConn.SendDownlinkNASTransport(ctx, n1Msg)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error sending downlink nas transport", zap.Error(err))
			return
		}

		logger.From(ctx, logger.AmfLog).Info("sent downlink nas transport to UE")
	}
}

func transport5GSMMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, ulNasTransport *nasMessage.ULNASTransport) {
	smMessage := ulNasTransport.GetPayloadContainerContents()

	id := ulNasTransport.PduSessionID2Value
	if id == nil {
		logger.From(ctx, logger.AmfLog).Warn("pdu session id is nil")
		return
	}

	pduSessionID := id.GetPduSessionID2Value()

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
		sendPayloadNotForwarded(ctx, ueConn, pduSessionID, smMessage)
		return
	}

	requestType := ulNasTransport.RequestType

	if requestType != nil {
		switch requestType.GetRequestTypeValue() {
		case nasMessage.ULNASTransportRequestTypeInitialEmergencyRequest,
			nasMessage.ULNASTransportRequestTypeExistingEmergencyPduSession:
			logger.From(ctx, logger.AmfLog).Warn("Emergency PDU Session is not supported")
			sendPayloadNotForwarded(ctx, ueConn, pduSessionID, smMessage)

			return
		}
	}

	smContext, smContextExist := ue.SmContextFindByPDUSessionID(pduSessionID)

	isInitialRequest := requestType != nil &&
		requestType.GetRequestTypeValue() == nasMessage.ULNASTransportRequestTypeInitialRequest

	// Duplicate PDU session ID: an initial request for an active session locally
	// releases it and re-establishes (TS 24.501).
	if smContextExist && isInitialRequest {
		ue.DeleteSmContext(pduSessionID)

		smContext, smContextExist = nil, false
	}

	if smContextExist {
		// Existing PDU session whose S-NSSAI is not allowed for the access type
		// (TS 24.501).
		if requestType != nil &&
			requestType.GetRequestTypeValue() == nasMessage.ULNASTransportRequestTypeExistingPduSession &&
			!ue.IsAllowedNssai(smContext.Snssai) {
			logger.From(ctx, logger.AmfLog).Error("S-NSSAI is not allowed for access type", zap.Any("snssai", smContext.Snssai), zap.Uint8("pduSessionID", pduSessionID))
			sendPayloadNotForwarded(ctx, ueConn, pduSessionID, smMessage)

			return
		}

		forward5GSMMessageToSMF(ctx, amfInstance, ue, pduSessionID, smContext.Ref, smMessage)

		return
	}

	if isInitialRequest {
		establishPDUSession(ctx, amfInstance, ue, ueConn, ulNasTransport, pduSessionID, smMessage)
		return
	}

	// A 5GSM STATUS for a PDU session with no context is ignored (TS 24.501).
	if isStatus5GSM(smMessage) {
		logger.From(ctx, logger.AmfLog).Warn("5GSM STATUS for unknown PDU session, ignoring", zap.Uint8("pduSessionID", pduSessionID))
		return
	}

	// No routing context and not an initial request (TS 24.501).
	sendPayloadNotForwarded(ctx, ueConn, pduSessionID, smMessage)
}

// sendPayloadNotForwarded returns the 5GSM message to the UE in a DL NAS
// TRANSPORT with 5GMM cause #90 "payload was not forwarded" (TS 24.501).
func sendPayloadNotForwarded(ctx context.Context, ueConn *amf.UeConn, pduSessionID uint8, smMessage []byte) {
	amf.SendDLNASTransport(ctx, ueConn, nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
}

func isStatus5GSM(smMessage []byte) bool {
	m := new(nas.Message)
	if err := m.PlainNasDecode(&smMessage); err != nil {
		return false
	}

	return m.GsmMessage != nil && m.Status5GSM != nil
}

// establishPDUSession selects an SMF and creates the SM context for an initial
// request (TS 24.501). When the SMF rejects, its reject message
// is returned to the UE; when it produces none, the payload is reported as not
// forwarded.
func establishPDUSession(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, ueConn *amf.UeConn, ulNasTransport *nasMessage.ULNASTransport, pduSessionID uint8, smMessage []byte) {
	var (
		snssai *models.Snssai
		dnn    string
	)

	if ulNasTransport.SNSSAI != nil {
		snssai = util.SnssaiToModels(ulNasTransport.SNSSAI)
	} else {
		if len(ue.AllowedNssai) == 0 {
			logger.From(ctx, logger.AmfLog).Warn("allowed nssai is empty in UE context")
			return
		}

		snssai = &ue.AllowedNssai[0]
	}

	if ulNasTransport.DNN != nil && ulNasTransport.DNN.GetLen() > 0 {
		dnn = ulNasTransport.GetDNN()
	} else {
		dnnResp, err := amfInstance.SubscriberDnn(ctx, ue.Supi(), snssai)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to get subscriber data", zap.Error(err))
			return
		}

		dnn = dnnResp
	}

	smContextRef, errResponse, err := amfInstance.Session.CreateSmContext(ctx, ue.Supi(), pduSessionID, dnn, snssai, smMessage)

	// The SMF produced a 5GSM reject. Delivering it is a normal negative outcome,
	// not a 5GMM protocol error (TS 24.501).
	if errResponse != nil {
		amf.SendDLNASTransport(ctx, ueConn, nasMessage.PayloadContainerTypeN1SMInfo, errResponse, pduSessionID, 0)

		logger.From(ctx, logger.AmfLog).Info("PDU session establishment rejected by SMF", zap.Uint8("pduSessionID", pduSessionID), zap.Error(err))

		return
	}

	// The SMF failed without producing a reject. Tell the UE the payload was not
	// forwarded (5GMM cause #90) so it does not time out (TS 24.501).
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("couldn't create sm context", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))

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

	if err := ue.CreateSmContext(pduSessionID, smContextRef, snssai); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("error creating SM context", zap.Error(err))
		return
	}

	logger.From(ctx, logger.AmfLog).Debug("Created sm context for pdu session", zap.Uint8("pduSessionID", pduSessionID))
}

func handleULNASTransport(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.ULNASTransport) nasreply.Disposition {
	if ue.State() != amf.Registered {
		logger.From(ctx, logger.AmfLog).Warn("expected UE to be in Registered state during UL NAS Transport", zap.String("state", string(ue.State())))
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	switch msg.GetPayloadContainerType() {
	case nasMessage.PayloadContainerTypeN1SMInfo:
		transport5GSMMessage(ctx, amfInstance, ue, msg)
	case nasMessage.PayloadContainerTypeSMS:
		logger.From(ctx, logger.AmfLog).Warn("PayloadContainerTypeSMS has not been implemented yet in UL NAS TRANSPORT")
	case nasMessage.PayloadContainerTypeLPP:
		lppData := msg.GetPayloadContainerContents()

		// The UE echoes its routing context in the Additional information IE
		// (TS 24.501 §5.4.5.2.1 case c); record whether it is present so an
		// LPP reply can be correlated back to the LMF that sent the request.
		additionalInfo := ""
		if msg.AdditionalInformation != nil {
			additionalInfo = hex.EncodeToString(msg.GetAdditionalInformationValue())
		}

		logger.From(ctx, logger.AmfLog).Info("UL NAS Transport carries LPP payload",
			logger.SUPI(ue.Supi().String()),
			zap.Int("length", len(lppData)),
			zap.String("lpp_hex", hex.EncodeToString(lppData)),
			zap.String("additional_information", additionalInfo),
		)

		if amfInstance.LPPHandler != nil {
			if err := amfInstance.LPPHandler.ForwardLPP(ctx, ue.Supi(), lppData); err != nil {
				logger.From(ctx, logger.AmfLog).Error("failed to forward LPP to LMF", zap.Error(err))
			}
		} else {
			logger.From(ctx, logger.AmfLog).Error("LPP handler not configured")
		}
	case nasMessage.PayloadContainerTypeSOR:
		logger.From(ctx, logger.AmfLog).Warn("PayloadContainerTypeSOR has not been implemented yet in UL NAS TRANSPORT")
	case nasMessage.PayloadContainerTypeUEPolicy:
		logger.From(ctx, logger.AmfLog).Info("amf.AMF Transfer UEPolicy To PCF")
	case nasMessage.PayloadContainerTypeUEParameterUpdate:
		logger.From(ctx, logger.AmfLog).Info("amf.AMF Transfer UEParameterUpdate To UDM")

		upuMac, err := nasConvert.UpuAckToModels(msg.GetPayloadContainerContents())
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to convert UPU ACK to models", zap.Error(err))
			return nasreply.Handled()
		}

		logger.From(ctx, logger.AmfLog).Debug("UpuMac in UPU ACK NAS Msg", zap.String("UpuMac", upuMac))
	case nasMessage.PayloadContainerTypeMultiplePayload:
		logger.From(ctx, logger.AmfLog).Warn("PayloadContainerTypeMultiplePayload has not been implemented yet in UL NAS TRANSPORT")
	}

	return nasreply.Handled()
}
