package ue

import (
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func handleConfigurationUpdateCommand(ue *UE, cfgUpdCmd *nasMessage.ConfigurationUpdateCommand, amfUENGAPID int64, ranUENGAPID int64) error {
	ue.Set5gGuti(cfgUpdCmd.GUTI5G)

	commandComplete, err := BuildConfigurationUpdateComplete()
	if err != nil {
		return fmt.Errorf("could not build Configuration Update Complete NAS PDU: %v", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(commandComplete, nas.SecurityHeaderTypeIntegrityProtectedAndCiphered)
	if err != nil {
		return fmt.Errorf("error encoding %s IMSI UE NAS Configuration Update Complete", ue.UeSecurity.Supi)
	}

	err = ue.Gnb.SendUplinkNAS(encodedPdu, amfUENGAPID, ranUENGAPID)
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent Configuration Update Complete NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	return nil
}
