package ue

import (
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func handlePDUSessionEstablishmentReject(ue *UE, msg *nasMessage.PDUSessionEstablishmentReject) error {
	if msg == nil {
		return fmt.Errorf("received nil NAS message in PDU Session Establishment Reject handler")
	}

	cause := msg.GetCauseValue()

	logger.UeLogger.Debug(
		"Received PDU Session Establishment Reject NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Uint8("PDU Session ID", msg.GetPDUSessionID()),
		zap.String("Cause", cause5GSMToString(cause)),
	)

	return nil
}
