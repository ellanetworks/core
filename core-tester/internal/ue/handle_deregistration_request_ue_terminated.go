package ue

import (
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/nas"
)

func handleDeregistrationRequestUETerminated(ue *UE, _ *nas.Message, amfUENGAPID int64, ranUENGAPID int64) error {
	logger.UeLogger.Debug("Received Deregistration Request UE Terminated NAS message")
	return nil
}
