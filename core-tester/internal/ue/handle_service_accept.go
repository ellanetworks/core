package ue

import (
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/nas"
	"go.uber.org/zap"
)

func handleServiceAccept(ue *UE, msg *nas.Message) error {
	logger.UeLogger.Debug("Received Service Accept NAS message", zap.String("IMSI", ue.UeSecurity.Supi))

	if msg == nil {
		return fmt.Errorf("received nil NAS message in Service Accept handler")
	}

	return nil
}
