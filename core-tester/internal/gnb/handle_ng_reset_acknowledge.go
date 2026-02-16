package gnb

import (
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func handleNGResetAcknowledge(_ *ngapType.NGResetAcknowledge) error {
	logger.GnbLogger.Debug("Received NGResetAcknowledge")

	return nil
}
