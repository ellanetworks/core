package gnb

import (
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleNGSetupFailure(nGSetupFailure *ngapType.NGSetupFailure) error {
	var cause *ngapType.Cause

	for _, ie := range nGSetupFailure.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
		}
	}

	logger.GnbLogger.Debug("Received NGSetupFailure",
		zap.String("Cause", causeToString(*cause)),
	)

	return nil
}
