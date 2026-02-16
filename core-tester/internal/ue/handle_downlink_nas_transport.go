package ue

import (
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/free5gc/nas"
	"go.uber.org/zap"
)

func handleDLNASTransport(ue *UE, msg *nas.Message) error {
	pduSessionID := msg.DLNASTransport.GetPduSessionID2Value()

	payloadContainer, err := utils.GetNasPduFromPduAccept(msg)
	if err != nil {
		return fmt.Errorf("could not get PDU Session establishment accept: %v", err)
	}

	logger.UeLogger.Debug(
		"Received DL NAS Transport NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Uint8("PDU Session ID", pduSessionID),
	)

	pcMsgType := payloadContainer.GsmHeader.GetMessageType()

	switch pcMsgType {
	case nas.MsgTypePDUSessionEstablishmentAccept:
		err := handlePDUSessionEstablishmentAccept(ue, payloadContainer.PDUSessionEstablishmentAccept)
		if err != nil {
			return fmt.Errorf("could not handle PDU Session Establishment Accept: %v", err)
		}
	case nas.MsgTypePDUSessionEstablishmentReject:
		err := handlePDUSessionEstablishmentReject(ue, payloadContainer.PDUSessionEstablishmentReject)
		if err != nil {
			return fmt.Errorf("could not handle PDU Session Establishment Reject: %v", err)
		}
	default:
		logger.UeLogger.Warn("Message type not implemented", zap.String("Message Type", getGSMMessageName(pcMsgType)))
	}

	updateReceivedGSMMessages(ue, payloadContainer)

	return nil
}
