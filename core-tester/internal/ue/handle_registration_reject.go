package ue

import (
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/nas"
	"go.uber.org/zap"
)

func handleRegistrationReject(ue *UE, msg *nas.Message) error {
	if msg == nil {
		return fmt.Errorf("received nil NAS message in Registration Reject handler")
	}

	cause := msg.RegistrationReject.GetCauseValue()

	logger.UeLogger.Debug(
		"Received Registration Reject NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.String("Cause", cause5GMMToString(cause)),
	)

	return nil
}
