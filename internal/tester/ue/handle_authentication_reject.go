package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/nas"
	"go.uber.org/zap"
)

func handleAuthenticationReject(ue *UE, msg *nas.Message) error {
	if msg == nil {
		return fmt.Errorf("received nil NAS message in Authentication Reject handler")
	}

	logger.UeLogger.Debug("Received Authentication Reject NAS message", zap.String("IMSI", ue.UeSecurity.Supi))

	return nil
}
