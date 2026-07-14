// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/openapi/models"
	"go.uber.org/zap"
)

func handleDLNASTransport(ue *UE, msg *nas.Message, amfUENGAPID int64, ranUENGAPID int64) error {
	payloadContainerContents := msg.DLNASTransport.GetPayloadContainerContents()
	payloadContainerType := msg.DLNASTransport.GetPayloadContainerType()

	var pduSessionID uint8
	if msg.DLNASTransport.PduSessionID2Value != nil {
		pduSessionID = msg.DLNASTransport.GetPduSessionID2Value()
	}

	logger.UeLogger.Debug(
		"Received DL NAS Transport NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Uint8("PDU Session ID", pduSessionID),
		zap.Uint8("Payload Container Type", payloadContainerType),
	)

	switch payloadContainerType {
	case nasMessage.PayloadContainerTypeLPP:
		return handleLPPPayload(ue, payloadContainerContents, amfUENGAPID, ranUENGAPID)
	case nasMessage.PayloadContainerTypeN1SMInfo:
		return handle5GSMPayload(ue, payloadContainerContents)
	default:
		logger.UeLogger.Warn("Unknown payload container type in DL NAS Transport",
			zap.Uint8("type", payloadContainerType))
	}

	return nil
}

func handleLPPPayload(ue *UE, lppPayload []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	if len(lppPayload) == 0 {
		return fmt.Errorf("LPP payload is empty")
	}

	transactionID, bodyKind, err := DecodeLPPMessage(lppPayload)
	if err != nil {
		return fmt.Errorf("decode LPP message: %w", err)
	}

	switch bodyKind {
	case lpptype.LPPMessageBodyC1PresentRequestCapabilities:
		return handleLPPCapabilitiesRequest(ue, transactionID, amfUENGAPID, ranUENGAPID)
	case lpptype.LPPMessageBodyC1PresentRequestLocationInformation:
		return handleLPPLocationRequest(ue, transactionID, amfUENGAPID, ranUENGAPID)
	case lpptype.LPPMessageBodyC1PresentAbort:
		logger.UeLogger.Error("Received LPP Abort message from LMF",
			zap.Uint8("transactionID", transactionID),
			zap.Int("bodyKind", bodyKind))

		return fmt.Errorf("received LPP Abort from LMF")
	case lpptype.LPPMessageBodyC1PresentError:
		logger.UeLogger.Error("Received LPP Error message from LMF",
			zap.Uint8("transactionID", transactionID),
			zap.Int("bodyKind", bodyKind))

		return fmt.Errorf("received LPP Error from LMF")
	default:
		logger.UeLogger.Warn("Unimplemented LPP message body kind (not an error, ignoring)",
			zap.Int("bodyKind", bodyKind),
			zap.Uint8("transactionID", transactionID))
	}

	return nil
}

// handleLPPCapabilitiesRequest responds to a RequestCapabilities with ProvideCapabilities.
func handleLPPCapabilitiesRequest(ue *UE, transactionID byte, amfUENGAPID int64, ranUENGAPID int64) error {
	logger.UeLogger.Info("Received LPP RequestCapabilities",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Uint8("transactionID", transactionID),
	)

	capPayload, err := BuildLPPCapabilitiesResponse(&LPPCapabilitiesResponseOpts{
		TransactionID: transactionID,
		GNSSGPS:       true,
		GNSSGLO:       true,
	})
	if err != nil {
		return fmt.Errorf("build LPP capabilities response: %w", err)
	}

	capNasPdu, err := BuildUplinkNasTransportLPP(capPayload, amfUENGAPID, ranUENGAPID, "", models.Snssai{})
	if err != nil {
		return fmt.Errorf("build UL NAS Transport for LPP capabilities: %w", err)
	}

	capNasPduSecured, err := ue.EncodeNasPduWithSecurity(capNasPdu, nas.SecurityHeaderTypeIntegrityProtectedAndCiphered)
	if err != nil {
		return fmt.Errorf("encrypt LPP capabilities NAS PDU: %w", err)
	}

	if err := ue.Gnb.SendUplinkNAS(capNasPduSecured, amfUENGAPID, ranUENGAPID); err != nil {
		return fmt.Errorf("send LPP capabilities response: %w", err)
	}

	ue.mu.Lock()
	ue.lppCapsSent = true
	ue.mu.Unlock()

	logger.UeLogger.Info("Sent LPP ProvideCapabilities")

	return nil
}

// handleLPPLocationRequest responds to a RequestLocationInformation with
// ProvideLocationInformation containing hardcoded coordinates.
func handleLPPLocationRequest(ue *UE, transactionID byte, amfUENGAPID int64, ranUENGAPID int64) error {
	logger.UeLogger.Info("Received LPP RequestLocationInformation",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Uint8("transactionID", transactionID),
	)

	locPayload, err := BuildLPPLocationResponse(&LPPLocationResponseOpts{
		TransactionID:      transactionID,
		Latitude:           450000000, // 45.0 degrees * 1e7
		Longitude:          214500000, // 21.45 degrees * 1e7
		Altitude:           10000,     // 100m in cm
		HorizontalAccuracy: 10,        // 10 meters
		VerticalAccuracy:   15,        // 15 meters
		Timestamp:          0,         // 0 = current time (LMF will handle)
	})
	if err != nil {
		return fmt.Errorf("build LPP location response: %w", err)
	}

	locNasPdu, err := BuildUplinkNasTransportLPP(locPayload, amfUENGAPID, ranUENGAPID, "", models.Snssai{})
	if err != nil {
		return fmt.Errorf("build UL NAS Transport for LPP location: %w", err)
	}

	locNasPduSecured, err := ue.EncodeNasPduWithSecurity(locNasPdu, nas.SecurityHeaderTypeIntegrityProtectedAndCiphered)
	if err != nil {
		return fmt.Errorf("encrypt LPP location NAS PDU: %w", err)
	}

	if err := ue.Gnb.SendUplinkNAS(locNasPduSecured, amfUENGAPID, ranUENGAPID); err != nil {
		return fmt.Errorf("send LPP location response: %w", err)
	}

	logger.UeLogger.Info("Sent LPP ProvideLocationInformation",
		zap.Int32("latitude", 450000000),
		zap.Int32("longitude", 214500000),
	)

	return nil
}

func handle5GSMPayload(ue *UE, payload []byte) error {
	m := new(nas.Message)
	if err := m.PlainNasDecode(&payload); err != nil {
		return fmt.Errorf("could not decode 5GSM payload: %v", err)
	}

	pcMsgType := m.GsmMessage.GetMessageType()

	switch pcMsgType {
	case nas.MsgTypePDUSessionEstablishmentAccept:
		err := handlePDUSessionEstablishmentAccept(ue, m.PDUSessionEstablishmentAccept)
		if err != nil {
			return fmt.Errorf("could not handle PDU Session Establishment Accept: %v", err)
		}
	case nas.MsgTypePDUSessionEstablishmentReject:
		err := handlePDUSessionEstablishmentReject(ue, m.PDUSessionEstablishmentReject)
		if err != nil {
			return fmt.Errorf("could not handle PDU Session Establishment Reject: %v", err)
		}
	default:
		logger.UeLogger.Warn("5GSM message type not implemented", zap.String("Message Type", getGSMMessageName(pcMsgType)))
	}

	updateReceivedGSMMessages(ue, m)

	return nil
}
