// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func handleDLNASTransport(ue *UE, msg *nas.Message) error {
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
	case nasMessage.PayloadContainerTypeN1SMInfo:
		return handle5GSMPayload(ue, payloadContainerContents)
	default:
		logger.UeLogger.Warn("Unknown payload container type in DL NAS Transport",
			zap.Uint8("type", payloadContainerType))
	}

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
