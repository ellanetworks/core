package ue

import (
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/logger"
	"go.uber.org/zap"
)

func handleIdentityRequest(ue *UE, amfUENGAPID int64, ranUENGAPID int64) error {
	logger.UeLogger.Debug("Received Identity Request NAS message")

	identityResp, err := BuildIdentityResponse(&IdentityResponseOpts{
		Suci: ue.GetSuci(),
	})
	if err != nil {
		return fmt.Errorf("could not build Identity Response NAS PDU: %v", err)
	}

	err = ue.Gnb.SendUplinkNAS(identityResp, amfUENGAPID, ranUENGAPID)
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent Identity Response NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	return nil
}
