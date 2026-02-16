package gnb

import (
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleErrorIndication(errorIndication *ngapType.ErrorIndication) error {
	var cause *ngapType.Cause

	for _, ie := range errorIndication.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
		}
	}

	logger.GnbLogger.Error("Received ErrorIndication",
		zap.String("Cause", causeToString(*cause)),
	)

	return nil
}
