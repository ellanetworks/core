package gnb

import (
	"github.com/ellanetworks/core/internal/tester/logger"
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

	causeStr := "(none)"
	if cause != nil {
		causeStr = causeToString(*cause)
	}

	logger.GnbLogger.Error("Received ErrorIndication",
		zap.String("Cause", causeStr),
	)

	return nil
}
